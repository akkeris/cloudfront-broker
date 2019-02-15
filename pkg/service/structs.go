// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
)

type AwsConfigSpec struct {
	namePrefix        string
	conf              *aws.Config
	sess              *session.Session
	taskSleep 				time.Duration
}

type InCreateDistributionSpec struct {
	CallerReference string
	BillingCode     string
	Plan            string
	BucketName            string
	distChan				      chan error
}

type CloudFrontInstanceSpec struct {
	DistributionID        string
	DistributionURL       string
	CallerReference       string
	OriginAccessIdentity  string
	AccessKey             string
	SecretKey             string
	S3Bucket              *S3BucketSpec
}

type S3BucketSpec struct {
	Name      *string
	Fullname  *string
	Uri       *string
	ID        *string
}
