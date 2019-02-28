// Author: ned.hanks
// Date Created: December 7, 2018
// Project:
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/nu7hatch/gouuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func (s *AwsConfigSpec) genBucketName() *string {
	newUuid, _ := uuid.NewV4()

	bucketName := strings.Split(newUuid.String(), "-")[0]
	bucketName = s.namePrefix + "-" + bucketName

	return &bucketName
}

func (s *AwsConfigSpec) createS3Bucket(cf *cloudFrontInstanceSpec) {

	glog.Infof("==== createS3Bucket [%s] ====", *cf.operationKey)
	svc := s3.New(s.sess)
	glog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := "error getting s3 session"
		glog.Errorf(msg)
		cf.distChan <- errors.New(msg)
	}

	bucketName := s.genBucketName()

	s3in := &s3.CreateBucketInput{
		Bucket: bucketName,
	}

	s3out, err := svc.CreateBucket(s3in)
	if err != nil {
		msg := fmt.Sprintf("error creating s3 bucket: %s", err.Error())
		glog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	fullname := strings.Replace(*s3out.Location, "http://", "", -1)
	fullname = strings.Replace(fullname, "/", "", -1)

	glog.Infof("bucket name: %s\n", *bucketName)

	headBucketIn := &s3.HeadBucketInput{
		Bucket: bucketName,
	}

	glog.Info(">>>> waiting for distribution <<<<")
	err = svc.WaitUntilBucketExists(headBucketIn)

	if err != nil {
		glog.Errorf("error waiting for bucket %s: %s\n", *bucketName, err.Error())
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

func (s *AwsConfigSpec) addBucketPolicy(cf *cloudFrontInstanceSpec) error {
	glog.Infof("==== addBucketPolicy [%s] ====", *cf.operationKey)

	policy, _ := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Id":      fmt.Sprintf("Policy%s", *cf.distributionId),
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

	glog.Infof("\nbucket policy: %s\n", policy)
	svc := s3.New(s.sess)
	glog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := "error getting s3 session"
		glog.Error(msg)
		return errors.New(msg)
	}

	_, err := svc.PutBucketPolicy(&s3.PutBucketPolicyInput{
		Bucket: cf.s3Bucket.name,
		Policy: aws.String(string(policy)),
	})

	if err != nil {
		msg := fmt.Sprintf("error adding bucketpolicy to %s: %s", *cf.s3Bucket.name, err.Error())
		glog.Errorf(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfigSpec) deleteS3Bucket(cf *cloudFrontInstanceSpec) error {
	glog.Infof("==== deleteS3Bucket [%s] ====", *cf.operationKey)

	svc := s3.New(s.sess)

	input := &s3.DeleteBucketInput{
		Bucket: cf.s3Bucket.name,
	}

	err := input.Validate()
	if err != nil {
		glog.Errorf("error validating delete bucket input: %s\n", err)
		return err
	}

	_, err = svc.DeleteBucket(input)

	if err != nil {
		glog.Errorf("error deleting bucket %s: %s\n", *cf.s3Bucket.name, err)
		return err
	}

	waitIn := &s3.HeadBucketInput{
		Bucket: cf.s3Bucket.name,
	}

	err = svc.WaitUntilBucketNotExists(waitIn)

	if err != nil {
		glog.Errorf("error deleting bucket %s: %s\n", *cf.s3Bucket.name, err)
		return err
	}

	return nil
}
