package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/nu7hatch/gouuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

func (s *AwsConfig) genBucketName() *string {
	newUUID, _ := uuid.NewV4()

	bucketName := strings.Split(newUUID.String(), "-")[0]
	bucketName = s.namePrefix + "-" + bucketName

	return &bucketName
}

func (s *AwsConfig) createS3Bucket(cf *cloudFrontInstance) error {

	glog.V(4).Info("==== createS3Bucket ====")
	svc := s3.New(s.sess)
	if svc == nil {
		msg := "createS3Bucket: error getting s3 session"
		glog.Errorf(msg)
		return errors.New(msg)
	}

	bucketName := s.genBucketName()

	glog.V(0).Infof("createS3Bucket: bucket name: %s", *bucketName)

	s3in := &s3.CreateBucketInput{
		Bucket: bucketName,
	}

	s3out, err := svc.CreateBucket(s3in)

	if err != nil {
		msg := fmt.Sprintf("error creating s3 bucket: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	fullname := strings.Replace(*s3out.Location, "http://", "", -1)
	fullname = strings.Replace(fullname, "/", "", -1)

	cf.s3Bucket = &s3Bucket{
		bucketURI:  s3out.Location,
		bucketName: bucketName,
		fullname:   aws.String(fullname),
	}

	origin, err := s.stg.AddOrigin(*cf.distributionID, *bucketName, *s3out.Location, "/")

	if err != nil {
		msg := fmt.Sprintf("createS3Bucket: error adding origin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	cf.s3Bucket.originID = &origin.OriginID

	return nil
}

func (s *AwsConfig) isBucketReady(s3BucketIn *s3Bucket) bool {
	getBucketLocationIn := &s3.GetBucketLocationInput{
		Bucket: s3BucketIn.bucketName,
	}

	svc := s3.New(s.sess)

	_, err := svc.GetBucketLocation(getBucketLocationIn)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NoSuchBucket":
				return false
			default:
				msg := fmt.Sprintf("isBucketReady: error checking bucket: %s", err.Error())
				glog.Error(msg)
				return false
			}
		}
	}

	return true
}

func (s *AwsConfig) getBucket(originID string) *s3Bucket {

	origin, err := s.stg.GetOriginByID(originID)

	if err != nil {
		msg := fmt.Sprintf("getBucket: error finding bucket: %s", err.Error())
		glog.Error(msg)
		return nil
	}

	s3BucketOut := &s3Bucket{
		bucketName: &origin.BucketName,
		bucketURI:  &origin.BucketURL,
		originID:   &origin.OriginID,
		iAMUser: &iAMUser{
			userName:  &origin.IAMUser.String,
			accessKey: &origin.AccessKey.String,
			secretKey: &origin.SecretKey.String,
		},
	}

	return s3BucketOut
}

func (s *AwsConfig) addBucketPolicy(cf *cloudFrontInstance) error {
	glog.V(4).Infof("==== addBucketPolicy [%s] ====", *cf.operationKey)

	policy, _ := json.Marshal(map[string]interface{}{
		"Version": "2012-10-17",
		"Id":      fmt.Sprintf("Policy%s", *cf.cloudfrontID),
		"Statement": []map[string]interface{}{
			{
				"Sid":    fmt.Sprintf("Stmt%s", *cf.originAccessIdentity),
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"AWS": fmt.Sprintf("arn:aws:iam::cloudfront:user/CloudFront Origin Access Identity %s", *cf.originAccessIdentity),
				},
				"Action":   "s3:GetObject",
				"Resource": fmt.Sprintf("arn:aws:s3:::%s/*", *cf.s3Bucket.bucketName),
			},
		},
	})

	glog.V(4).Infof("addBucketPolicy [%s]: policy %#v", *cf.operationKey, string(policy))
	svc := s3.New(s.sess)
	if svc == nil {
		msg := "error getting s3 session"
		glog.Error(msg)
		return errors.New(msg)
	}

	_, err := svc.PutBucketPolicy(&s3.PutBucketPolicyInput{
		Bucket: cf.s3Bucket.bucketName,
		Policy: aws.String(string(policy)),
	})

	if err != nil {
		msg := fmt.Sprintf("error adding bucketpolicy to %s: %s", *cf.s3Bucket.bucketName, err.Error())
		glog.Errorf(msg)
		return errors.New(msg)
	}

	corsRule := s3.CORSRule{
		AllowedHeaders: aws.StringSlice([]string{"Authorization"}),
		AllowedOrigins: aws.StringSlice([]string{"*"}),
		AllowedMethods: aws.StringSlice([]string{"GET", "HEAD"}),
		MaxAgeSeconds:  aws.Int64(3600),
	}

	corsIn := &s3.PutBucketCorsInput{
		Bucket: cf.s3Bucket.bucketName,
		CORSConfiguration: &s3.CORSConfiguration{
			CORSRules: []*s3.CORSRule{&corsRule},
		},
	}

	_, err = svc.PutBucketCors(corsIn)

	if err != nil {
		msg := fmt.Sprintf("error adding CORS Policy to %s: %s", *cf.s3Bucket.bucketName, err.Error())
		glog.Errorf(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) deleteS3Bucket(cf *cloudFrontInstance) error {
	glog.V(4).Infof("==== deleteS3Bucket [%s] ====", *cf.operationKey)

	svc := s3.New(s.sess)

	input := &s3.DeleteBucketInput{
		Bucket: cf.s3Bucket.bucketName,
	}

	err := input.Validate()
	if err != nil {
		glog.Errorf("deleteS3Bucket: error validating delete bucket input: %s", err)
		return err
	}

	_, err = svc.DeleteBucket(input)

	if err != nil {
		glog.Errorf("deleteS3Bucket: error deleting bucket %s: %s\n", *cf.s3Bucket.bucketName, err)
		return err
	}

	_, err = s.stg.UpdateDeleteOrigin(*cf.distributionID, *cf.s3Bucket.originID)

	if err != nil {
		glog.Errorf("deleteS3Bucket: error updating deleted at: %s\n", err)
		return err
	}

	return nil
}
