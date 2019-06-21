package service

import (
	"cloudfront-broker/pkg/storage"

	osb "github.com/pmorie/go-open-service-broker-client/v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

// AwsConfig holds values for AWS services interaction
type AwsConfig struct {
	namePrefix string
	conf       *aws.Config
	sess       *session.Session
	waitSecs   int64
	maxRetries int64
	stg        *storage.PostgresStorage
}

type cloudFrontInstance struct {
	distributionID       *string
	billingCode          *string
	planID               *string
	serviceID            *string
	cloudfrontID         *string
	cloudfrontURL        *string
	callerReference      *string
	originAccessIdentity *string
	s3Bucket             *s3Bucket
	operationKey         *string
}

type s3Bucket struct {
	originID   *string
	bucketName *string
	fullname   *string
	bucketURI  *string
	iAMUser    *iAMUser
}

type iAMUser struct {
	userName  *string
	arn       *string
	accessKey *string
	secretKey *string
}

// InstanceSpec is what's returned to calling app
type InstanceSpec struct {
	CloudFrontURL      string `json:"CLOUDFRONT_URL"`
	BucketName         string `json:"CF_BUCKET_NAME"`
	AwsAccessKey       string `json:"CF_AWS_ACCESS_KEY"`
	AwsSecretAccessKey string `json:"CF_AWS_SECRET_ACCESS_KEY"`
}

// Status strings from osb-service-lib
var (
	OperationInProgress = string(osb.StateInProgress)
	OperationSucceeded  = string(osb.StateSucceeded)
	OperationFailed     = string(osb.StateFailed)
)
