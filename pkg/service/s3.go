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

	"github.com/nu7hatch/gouuid"
	"k8s.io/klog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func (s *AwsConfigSpec) genBucketName() *string {
	newUuid, _ := uuid.NewV4()

	bucketName := strings.Split(newUuid.String(), "-")[0]
	bucketName = s.namePrefix + "-" + bucketName

	return &bucketName
}

func (s *AwsConfigSpec) createS3Bucket(ctx context.Context, cf *cloudFrontInstanceSpec) {

	svc := s3.New(s.sess)
	klog.Infof("\nsvc: %#+v\n", svc)
	if svc == nil {
		msg := "error getting s3 session"
		klog.Errorf(msg)
		cf.distChan <- errors.New(msg)
	}

	bucketName := s.genBucketName()

	s3in := &s3.CreateBucketInput{
		Bucket: bucketName,
	}

	s3out, err := svc.CreateBucket(s3in)
	klog.Infof("\ns3out: %#+v\n", s3out)
	if err != nil {
		msg := fmt.Sprintf("error creating s3 bucket: %s", err.Error())
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	fullname := strings.Replace(*s3out.Location, "http://", "", -1)
	fullname = strings.Replace(fullname, "/", "", -1)

	fmt.Printf("\nbucket name: %s", *bucketName)

	headBucketIn := &s3.HeadBucketInput{
		Bucket: bucketName,
	}

	err = svc.WaitUntilBucketExistsWithContext(ctx, headBucketIn)

	if err != nil {
		fmt.Printf("error waiting for bucket %s: %s\n", *bucketName, err.Error())
		cf.distChan <- err
		return
	}

	cf.s3Bucket = &s3BucketSpec{
		uri:      s3out.Location,
		name:     bucketName,
		fullname: aws.String(fullname),
		id:       aws.String("S3-" + *bucketName),
	}

	cf.distChan <- nil
}

func (s *AwsConfigSpec) addBucketPolicy(ctx context.Context, cf *cloudFrontInstanceSpec) error {

	policy, _ := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Id":      fmt.Sprintf("Policy%s", *cf.distributionID),
		"Statement": []map[string]interface{}{
			{
				"Sid":    fmt.Sprintf("Stmt%s", *cf.originAccessIdentity),
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"AWS": fmt.Sprintf("arn:aws:iam::cloudfront:user/CloudFront Origin Access Identity %s", *cf.originAccessIdentity),
				},
				"Action":   "s3:GetObject",
				"Resource": fmt.Sprintf("arn:aws:s3:::%s/*", *cf.s3Bucket.name),
			},
		},
	})

	klog.Infof("\nbucket policy: %s\n", policy)
	svc := s3.New(s.sess)
	klog.Infof("\nsvc: %#+v\n", svc)
	if svc == nil {
		msg := "error getting s3 session"
		klog.Error(msg)
		// cf.distChan <- errors.New(msg)
		return errors.New(msg)
	}

	/*
		bucketPolicyInput := &s3.PutBucketPolicyInput{
			Bucket: cf.s3Bucket.name,
			Policy: aws.String(string(policy)),
		}
		klog.Infof("\nbpIn: %#+v\n", bucketPolicyInput)
	*/

	_, err := svc.PutBucketPolicy(&s3.PutBucketPolicyInput{
		Bucket: cf.s3Bucket.name,
		Policy: aws.String(string(policy)),
	})

	if err != nil {
		msg := fmt.Sprintf("error adding bucketpolicy to %s: %s", *cf.s3Bucket.name, err.Error())
		klog.Errorf(msg)
		// cf.distChan <- errors.New(msg)
		return errors.New(msg)
	}

	// cf.distChan <- nil
	return nil
}

func (s *AwsConfigSpec) deleteS3Bucket(ctx context.Context, cf *cloudFrontInstanceSpec) error {
	svc := s3.New(s.sess)

	input := &s3.DeleteBucketInput{
		Bucket: cf.s3Bucket.name,
	}

	err := input.Validate()
	if err != nil {
		klog.Errorf("error validating delete bucket input: %s\n", err)
		return err
	}

	_, err = svc.DeleteBucket(input)

	if err != nil {
		klog.Errorf("error deleting bucket %s: %s\n", *cf.s3Bucket.name, err)
		return err
	}

	waitIn := &s3.HeadBucketInput{
		Bucket: cf.s3Bucket.name,
	}

	err = svc.WaitUntilBucketNotExistsWithContext(ctx, waitIn)

	if err != nil {
		klog.Errorf("error deleting bucket %s: %s\n", *cf.s3Bucket.name, err)
		return err
	}

	return nil
}
