// Author: ned.hanks
// Date Created: December 7, 2018
// Project:
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/klog"

	"cloudfront-broker/pkg/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func (s *AwsConfigSpec) genBucketName() string {
	newUuid := utils.NewUuid()
	bucketName := strings.Split(newUuid, "-")[0]
	bucketName = s.namePrefix + "-" + bucketName

	return bucketName
}

func (s *AwsConfigSpec) createS3Bucket(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) {

	svc := s3.New(s.sess)
	klog.Infof("\nsvc: %#+v\n", svc)
	if svc == nil {
		msg := "error getting s3 session"
		klog.Errorf(msg)
		in.distChan <- errors.New(msg)
	}

	in.BucketName = s.genBucketName()

	s3in := &s3.CreateBucketInput{
		Bucket: aws.String(in.BucketName),
	}

	s3out, err := svc.CreateBucket(s3in)
	klog.Infof("\ns3out: %#+v\n", s3out)
	if err != nil {
		msg := fmt.Sprintf("error creating s3 bucket: %s", err.Error())
		klog.Error(msg)
		in.distChan <- errors.New(msg)
		return
	}

	fullname := strings.Replace(*s3out.Location, "http://", "", -1)
	fullname = strings.Replace(fullname, "/", "", -1)

	fmt.Printf("\nbucket name: %s", in.BucketName)

	input := &s3.HeadBucketInput{
		Bucket: aws.String(in.BucketName),
	}

	err = svc.WaitUntilBucketExistsWithContext(ctx, input)

	if err != nil {
		fmt.Printf("error waiting for bucket %s: %s\n", in.BucketName, err.Error())
		in.distChan <- err
		return
	}
	out.S3Bucket = &S3BucketSpec{
		Uri:      s3out.Location,
		Name:     utils.StrPtr(in.BucketName),
		Fullname: utils.StrPtr(fullname),
		ID:       utils.StrPtr("S3-" + in.BucketName),
	}

	in.distChan <- nil
}

func (s *AwsConfigSpec) addBucketPolicy(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) {

	policy, _ := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Id":      fmt.Sprintf("Policy%s", *out.DistributionID),
		"Statement": []map[string]interface{}{
			{
				"Sid":    fmt.Sprintf("Stmt%s", *out.OriginAccessIdentity),
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"AWS": fmt.Sprintf("arn:aws:iam::cloudfront:user/CloudFront Origin Access Identity %s", *out.OriginAccessIdentity),
				},
				"Action":   "s3:GetObject",
				"Resource": fmt.Sprintf("arn:aws:s3:::%s/*", in.BucketName),
			},
		},
	})

	fmt.Printf("\nbucket policy: %s\n", policy)
	svc := s3.New(s.sess)
	klog.Infof("\nsvc: %#+v\n", svc)
	if svc == nil {
		msg := "error getting s3 session"
		klog.Error(msg)
		in.distChan <- errors.New(msg)
	}

	bucketPolicyInput := &s3.PutBucketPolicyInput{
		Bucket: aws.String(in.BucketName),
		Policy: aws.String(string(policy)),
	}

	klog.Infof("\nbpIn: %#+v\n", bucketPolicyInput)

	_, err := svc.PutBucketPolicy(bucketPolicyInput)
	if err != nil {
		msg := fmt.Sprintf("error adding bucketpolicy to %s: %s", in.BucketName, err.Error())
		klog.Errorf(msg)
		in.distChan <- errors.New(msg)
	}

	in.distChan <- nil
}

func (s *AwsConfigSpec) DeleteS3Bucket(ctx context.Context, b *S3BucketSpec) error {
	svc := s3.New(s.sess)

	input := &s3.DeleteBucketInput{
		Bucket: b.Name,
	}

	err := input.Validate()
	if err != nil {
		klog.Errorf("error validating delete bucket input: %s\n", err)
		return err
	}

	_, err = svc.DeleteBucket(input)

	if err != nil {
		klog.Errorf("error deleting bucket %s: %s\n", *b.Name, err)
		return err
	}

	waitIn := &s3.HeadBucketInput{
		Bucket: b.Name,
	}

	err = svc.WaitUntilBucketNotExistsWithContext(ctx, waitIn)

	if err != nil {
		klog.Errorf("error deleting bucket %s: %s\n", *b.Name, err)
		return err
	}

	return nil
}
