package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang/glog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
)

func (s *AwsConfigSpec) createIAMUser(cf *cloudFrontInstanceSpec) error {
	var err error
	var iamIn *iam.CreateUserInput

	glog.Infof("==== createIAMUser [%s] ====", *cf.operationKey)

	svc := iam.New(s.sess)
	if svc == nil {
		msg := fmt.Sprintf("error getting iam session: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	tags := []*iam.Tag{}
	tags = append(tags, &iam.Tag{
		Key:   aws.String("billingcode"),
		Value: cf.billingCode,
	})

	iamIn = &iam.CreateUserInput{
		UserName: cf.s3Bucket.name,
		Tags:     tags,
	}

	iamOut, err := svc.CreateUser(iamIn)

	if err != nil {
		msg := fmt.Sprintf("error creating iam user: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	giamIn := &iam.GetUserInput{
		UserName: iamOut.User.UserName,
	}

	err = svc.WaitUntilUserExists(giamIn)
	if err != nil {
		msg := fmt.Sprintf("error waiting for iam user: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	glog.Infof("iam username: %s", *giamIn.UserName)

	accessKeyInput := &iam.CreateAccessKeyInput{
		UserName: iamOut.User.UserName,
	}

	accessKeyOut, err := svc.CreateAccessKey(accessKeyInput)

	if err != nil {
		msg := fmt.Sprintf("error creating access key: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	glog.Infof("access key: %s", *accessKeyOut.AccessKey.AccessKeyId)
	cf.iAMUser = &iAMUserSpec{
		userName:  iamOut.User.UserName,
		arn:       iamOut.User.Arn,
		accessKey: accessKeyOut.AccessKey.AccessKeyId,
		secretKey: accessKeyOut.AccessKey.SecretAccessKey,
	}

	var policyIn *iam.PutUserPolicyInput
	// var policyOut *iam.PutUserPolicyOutput

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
					fmt.Sprintf("arn:aws:s3:::%s", *cf.s3Bucket.name),
					fmt.Sprintf("arn:aws:s3:::%s/*", *cf.s3Bucket.name),
				},
			},
		},
	})

	cf.iAMUser.policyName = aws.String(fmt.Sprintf("%s-policy", *cf.s3Bucket.name))

	policyIn = &iam.PutUserPolicyInput{
		PolicyName:     cf.iAMUser.policyName,
		PolicyDocument: aws.String(string(userPolicy)),
		UserName:       cf.iAMUser.userName,
	}

	_, err = svc.PutUserPolicy(policyIn)

	if err != nil {
		msg := fmt.Sprintf("error attaching policy: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfigSpec) deleteIAMUser(cf *cloudFrontInstanceSpec) error {
	var err error

	glog.Info("==== deleteIAMUser ====")

	svc := iam.New(s.sess)
	glog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting iam session: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	delKeyInput := &iam.DeleteAccessKeyInput{
		UserName:    cf.iAMUser.userName,
		AccessKeyId: cf.iAMUser.accessKey,
	}

	glog.Infof("deleteing access key for: %s\n", *delKeyInput.UserName)
	glog.Infof("deleting access key: %s", *delKeyInput.AccessKeyId)

	_, err = svc.DeleteAccessKey(delKeyInput)
	if err != nil {
		msg := fmt.Sprintf("error deleting access key: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	delUserInput := &iam.DeleteUserInput{
		UserName: cf.iAMUser.userName,
	}

	delUserPolicy := &iam.DeleteUserPolicyInput{
		UserName:   cf.iAMUser.userName,
		PolicyName: cf.iAMUser.policyName,
	}

	_, err = svc.DeleteUserPolicy(delUserPolicy)
	if err != nil {
		msg := fmt.Sprintf("error deleting user policy: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	_, err = svc.DeleteUser(delUserInput)
	if err != nil {
		msg := fmt.Sprintf("error deleting iam user: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}
