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
	billingCode          *string `json:"billing_code"`
	planID               *string `json:"plan_id"`
	serviceID            *string `json:"service_id"`
	cloudfrontID         *string `json:"cloudfront_id"`
	cloudfrontURL        *string `json:"cloudfront_url"`
	callerReference      *string
	originAccessIdentity *string   `json:"origin_access_identity"`
	s3Bucket             *s3Bucket `json:"s3_bucket"`
	operationKey         *string
}

type s3Bucket struct {
	originID   *string
	bucketName *string  `json:"bucket_name"`
	fullname   *string  `json:"fullname"`
	bucketURI  *string  `json:"bucket_uri"`
	iAMUser    *iAMUser `json:"iam_user"`
}

type iAMUser struct {
	userName  *string `json:"username"`
	arn       *string `json:"arn"`
	accessKey *string `json:"access_key"`
	secretKey *string `json:"secret_key"`
}

// InstanceSpec returned from bare GET request, OSB V2.14

type AccessSpec struct {
	CloudFrontURL      *string `structs:"CLOUDFRONT_URL"`
	BucketName         *string `structs:"CLOUDFRONT_BUCKET_NAME"`
	AwsAccessKey       *string `structs:"CLOUDFRONT_AWS_ACCESS_KEY"`
	AwsSecretAccessKey *string `structs:"CLOUDFRONT_AWS_SECRET_ACCESS_KEY"`
}

type IAMUserSpec struct {
	UserName  *string `json:"username"`
	ARN       *string `json:"ARN"`
	AccessKey *string `json:"access_key"`
	SecretKey *string `json:"secret_access_key"`
}

type S3BucketSpec struct {
	BucketName *string      `json:"bucket_name"`
	Fullname   *string      `json:"fullname"`
	BucketURI  *string      `json:"bucket_uri"`
	IAMUser    *IAMUserSpec `json:"iam_user"`
}

type InstanceSpec struct {
	ServiceID            *string       `json:"service_id"`
	PlanID               *string       `json:"plan_id"`
	BillingCode          *string       `json:"billingcode"`
	CloudfrontID         *string       `json:"cloudfront_id"`
	CloudfrontURL        *string       `json:"cloudfront_url"`
	OriginAccessIdentity *string       `json:"origin_access_identity"`
	S3Bucket             *S3BucketSpec `json:"s3_bucket"`
	Access               *AccessSpec   `json:"credentials"`
}

// Status strings from osb-service-lib
var (
	OperationInProgress = string(osb.StateInProgress)
	OperationSucceeded  = string(osb.StateSucceeded)
	OperationFailed     = string(osb.StateFailed)
)
