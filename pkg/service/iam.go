package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/klog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
)

func (s *AwsConfigSpec) createIAMUser(ctx context.Context, cf *cloudFrontInstanceSpec) error {
	var err error
	var iamIn *iam.CreateUserInput
	// var iamOut *iam.CreateUserOutput

	svc := iam.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting iam session: %s", err.Error())
		klog.Error(msg)
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

	iamOut, err := svc.CreateUserWithContext(ctx, iamIn)

	if err != nil {
		msg := fmt.Sprintf("error creating iam user: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	giamIn := &iam.GetUserInput{
		UserName: iamOut.User.UserName,
	}

	err = svc.WaitUntilUserExistsWithContext(ctx, giamIn)
	if err != nil {
		msg := fmt.Sprintf("error waiting for iam user: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	accessKeyInput := &iam.CreateAccessKeyInput{
		UserName: iamOut.User.UserName,
	}

	accessKeyOut, err := svc.CreateAccessKeyWithContext(ctx, accessKeyInput)

	if err != nil {
		msg := fmt.Sprintf("error creating access key: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

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

	_, err = svc.PutUserPolicyWithContext(ctx, policyIn)

	if err != nil {
		msg := fmt.Sprintf("error attaching policy: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfigSpec) deleteIAMUser(ctx context.Context, cf *cloudFrontInstanceSpec) error {
	var err error

	svc := iam.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting iam session: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	delKeyInput := &iam.DeleteAccessKeyInput{
		UserName:    cf.iAMUser.userName,
		AccessKeyId: cf.iAMUser.accessKey,
	}

	klog.Infof("deleteing access key for: %s\n", *delKeyInput.UserName)
	klog.Infof("deleting access key: %s", *delKeyInput.AccessKeyId)

	_, err = svc.DeleteAccessKeyWithContext(ctx, delKeyInput)
	if err != nil {
		msg := fmt.Sprintf("error deleting access key: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	delUserInput := &iam.DeleteUserInput{
		UserName: cf.iAMUser.userName,
	}

	delUserPolicy := &iam.DeleteUserPolicyInput{
		UserName:   cf.iAMUser.userName,
		PolicyName: cf.iAMUser.policyName,
	}

	_, err = svc.DeleteUserPolicyWithContext(ctx, delUserPolicy)
	if err != nil {
		msg := fmt.Sprintf("error deleting user policy: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	_, err = svc.DeleteUserWithContext(ctx, delUserInput)
	if err != nil {
		msg := fmt.Sprintf("error deleting iam user: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	return nil
}
