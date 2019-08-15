package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/golang/glog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
)

func (s *AwsConfig) createIAMUser(cf *cloudFrontInstance) error {
	var err error
	var iamIn *iam.CreateUserInput

	glog.Infof("==== createIAMUser ====")

	svc := iam.New(s.sess)
	if svc == nil {
		msg := fmt.Sprintf("createIAMUser: error getting iam session: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	tags := []*iam.Tag{}

	if cf.billingCode != nil {
		tags = append(tags, &iam.Tag{
			Key:   aws.String("billingcode"),
			Value: cf.billingCode,
		})
	}

	iamIn = &iam.CreateUserInput{
		UserName: cf.s3Bucket.bucketName,
		Tags:     tags,
	}

	iamOut, err := svc.CreateUser(iamIn)

	if err != nil {
		msg := fmt.Sprintf("createIAMUSer: error creating iam user: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	cf.s3Bucket.iAMUser = &iAMUser{
		userName:  iamOut.User.UserName,
		arn:       iamOut.User.Arn,
		accessKey: nil,
		secretKey: nil,
	}

	err = s.stg.AddIAMUser(*cf.s3Bucket.originID, *cf.s3Bucket.iAMUser.userName)

	if err != nil {
		msg := fmt.Sprintf("createIAMUser: error adding iam user: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) isIAMUserReady(userName string) (bool, error) {
	glog.Info("==== isIAMUserReady ====")

	svc := iam.New(s.sess)
	if svc == nil {
		msg := "checkIAMUser: error getting iam session"
		glog.Error(msg)
		return false, errors.New(msg)
	}

	giamIn := &iam.GetUserInput{
		UserName: aws.String(userName),
	}

	giamOut, err := svc.GetUser(giamIn)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				msg := fmt.Sprintf("checkIAMUser: iam user not found: %s", err.Error())
				glog.Info(msg)
				return false, errors.New(aerr.Code())
			default:
				msg := fmt.Sprintf("checkIAMUser: error getting iam user: %s", aerr.Error())
				glog.Error(msg)
				return false, errors.New(msg)
			}
		}
	}

	glog.Infof("isIAMUserReady: iam username: %s", *giamOut.User.UserName)

	return true, nil
}

func (s *AwsConfig) createAccessKey(cf *cloudFrontInstance) error {
	glog.Infof("==== createAccessKey [%s] ====", *cf.operationKey)

	svc := iam.New(s.sess)
	if svc == nil {
		msg := "createAccessKey: error getting iam session"
		glog.Error(msg)
		return errors.New(msg)
	}

	accessKeyOut, err := svc.CreateAccessKey(&iam.CreateAccessKeyInput{
		UserName: cf.s3Bucket.iAMUser.userName,
	})

	if err != nil {
		msg := fmt.Sprintf("createAccessKey: error creating access key: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	userPolicy, _ := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":    "list",
				"Effect": "Allow",
				"Action": []string{
					"s3:PutAccountPublicAccessBlock",
					"s3:GetAccountPublicAccessBlock",
					"s3:ListAllMyBuckets",
					"s3:HeadBucket",
				},
				"Resource": "*",
			},
			{
				"Sid":    "access",
				"Effect": "Allow",
				"Action": "s3:*",
				"Resource": []string{
					fmt.Sprintf("arn:aws:s3:::%s", *cf.s3Bucket.bucketName),
					fmt.Sprintf("arn:aws:s3:::%s/*", *cf.s3Bucket.bucketName),
				},
			},
		},
	})

	policyName := aws.String(fmt.Sprintf("%s-policy", *cf.s3Bucket.bucketName))

	_, err = svc.PutUserPolicy(&iam.PutUserPolicyInput{
		PolicyName:     policyName,
		PolicyDocument: aws.String(string(userPolicy)),
		UserName:       cf.s3Bucket.iAMUser.userName,
	})

	glog.Infof("createAccessKey: access key: %s", *accessKeyOut.AccessKey.AccessKeyId)
	cf.s3Bucket.iAMUser.accessKey = accessKeyOut.AccessKey.AccessKeyId
	cf.s3Bucket.iAMUser.secretKey = accessKeyOut.AccessKey.SecretAccessKey

	err = s.stg.AddAccessKey(*cf.s3Bucket.originID, *cf.s3Bucket.iAMUser.accessKey, *cf.s3Bucket.iAMUser.secretKey)

	if err != nil {
		msg := fmt.Sprintf("createAccessKey: error attaching policy: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) deleteIAMUser(cf *cloudFrontInstance) error {
	glog.Infof("==== deleteIAMUser [%s] ====", *cf.operationKey)

	svc := iam.New(s.sess)
	if svc == nil {
		msg := "error getting iam session"
		glog.Error(msg)
		return errors.New(msg)
	}

	glog.Infof("deleteIAMUser [%s]: deleting iam user: %s", *cf.operationKey, *cf.s3Bucket.iAMUser.userName)

	accessKeysOut, err := svc.ListAccessKeys(&iam.ListAccessKeysInput{
		UserName: cf.s3Bucket.iAMUser.userName,
	})

	if err != nil {
		msg := fmt.Sprintf("deleteIAMUser [%s]: error listing access keys: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	for i, accessKeyMeta := range accessKeysOut.AccessKeyMetadata {
		glog.Infof("deleteIAMUser [%s]: deleting access key[%d]: %s", *cf.operationKey, i, *accessKeyMeta.AccessKeyId)

		_, err := svc.DeleteAccessKey(&iam.DeleteAccessKeyInput{
			UserName:    cf.s3Bucket.iAMUser.userName,
			AccessKeyId: accessKeyMeta.AccessKeyId,
		})

		if err != nil {
			msg := fmt.Sprintf("deleteIAMUser [%s]: error deleting access key: %s", *cf.operationKey, err.Error())
			glog.Error(msg)
			return errors.New(msg)
		}
	}

	userPolicyOut, err := svc.ListUserPolicies(&iam.ListUserPoliciesInput{
		UserName: cf.s3Bucket.iAMUser.userName,
	})

	if err != nil {
		msg := fmt.Sprintf("deleteIAMUser [%s]: error listing policies: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	for i, policyName := range userPolicyOut.PolicyNames {

		glog.Infof("deleteIAMUser [%s]: delete user policy{%d]: %s", *cf.operationKey, i, *policyName)

		_, err = svc.DeleteUserPolicy(&iam.DeleteUserPolicyInput{
			UserName:   cf.s3Bucket.iAMUser.userName,
			PolicyName: policyName,
		})

		if err != nil {
			msg := fmt.Sprintf("deleteIAMUser [%s]: error deleting user policy: %s", *cf.operationKey, err.Error())
			glog.Error(msg)
			return errors.New(msg)
		}
	}

	glog.Infof("deleteIAMUser [%s]: delete user: %s", *cf.operationKey, *cf.s3Bucket.iAMUser.userName)

	_, err = svc.DeleteUser(&iam.DeleteUserInput{
		UserName: cf.s3Bucket.iAMUser.userName,
	})

	if err != nil {
		msg := fmt.Sprintf("deleteIAMUser [%s]: error deleting iam user: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}
