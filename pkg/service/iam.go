package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/klog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"

	"cloudfront-broker/pkg/utils"
)

func (s *AwsConfigSpec) createIAMUser(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) error {
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
		Key:   utils.StrPtr("billingcode"),
		Value: utils.StrPtr(in.BillingCode),
	})

	iamIn = &iam.CreateUserInput{
		UserName: &in.BucketName,
		Tags:     tags,
	}

	iamOut, err := svc.CreateUserWithContext(ctx, iamIn)

	if err != nil {
		msg := fmt.Sprintf("error creating iam user: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	out.IAMUser = iamOut.User.UserName
	out.IAMArn = iamOut.User.Arn

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
		UserName: out.IAMUser,
	}

	accessKeyOut, err := svc.CreateAccessKeyWithContext(ctx, accessKeyInput)

	if err != nil {
		msg := fmt.Sprintf("error creating access key: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	out.AccessKey = accessKeyOut.AccessKey.AccessKeyId
	out.SecretKey = accessKeyOut.AccessKey.SecretAccessKey

	// err = s.createIAMPolicy(ctx, in, out)

	// TODO attach policy to user

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
					fmt.Sprintf("arn:aws:s3:::%s", *out.S3Bucket.Name),
					fmt.Sprintf("arn:aws:s3:::%s/*", *out.S3Bucket.Name),
				},
			},
		},
	})

	out.PolicyName = aws.String(fmt.Sprintf("%s-policy", *out.S3Bucket.Name))

	policyIn = &iam.PutUserPolicyInput{
		PolicyName:     out.PolicyName,
		PolicyDocument: aws.String(string(userPolicy)),
		UserName:       out.IAMUser,
	}

	_, err = svc.PutUserPolicyWithContext(ctx, policyIn)

	if err != nil {
		msg := fmt.Sprintf("error attaching polixy: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfigSpec) createIAMPolicy(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) error {
	var err error
	var policyIn *iam.CreatePolicyInput
	var policyOut *iam.CreatePolicyOutput
	svc := iam.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting iam session: %s", err.Error())
		klog.Error(msg)
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
					fmt.Sprintf("arn:aws:s3:::%s", *out.S3Bucket.Name),
					fmt.Sprintf("arn:aws:s3:::%s/*", *out.S3Bucket.Name),
				},
			},
		},
	})

	policyIn = &iam.CreatePolicyInput{
		Description:    aws.String(fmt.Sprintf("Access to bucket %s", *out.S3Bucket.Name)),
		PolicyName:     aws.String(fmt.Sprintf("%s-policy", *out.S3Bucket.Name)),
		PolicyDocument: aws.String(string(userPolicy)),
	}

	policyOut, err = svc.CreatePolicyWithContext(ctx, policyIn)

	if err != nil {
		msg := fmt.Sprintf("error creating user policy: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	out.PolicyArn = policyOut.Policy.Arn
	out.PolicyName = policyOut.Policy.PolicyName
	return nil
}
