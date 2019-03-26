// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"cloudfront-broker/pkg/storage"

	osb "github.com/pmorie/go-open-service-broker-client/v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

type AwsConfig struct {
	namePrefix string
	conf       *aws.Config
	sess       *session.Session
	waitSecs   int64

	stg *storage.PostgresStorage
}

type cloudFrontInstance struct {
	distributionID       *string
	billingCode          *string
	planID               *string
	serviceId            *string
	cloudfrontID         *string
	cloudfrontURL        *string
	callerReference      *string
	originAccessIdentity *string
	s3Bucket             *s3Bucket
	operationKey         *string
	distChan             chan error
}

type s3Bucket struct {
	originID   *string
	bucketName *string
	fullname   *string
	bucketURI  *string
	iAMUser    *iAMUser
}

type iAMUser struct {
	userName   *string
	arn        *string
	accessKey  *string
	secretKey  *string
	policyName *string
}

var (
	OperationInProgress = string(osb.StateInProgress)
	OperationSucceeded  = string(osb.StateSucceeded)
	OperationFailed     = string(osb.StateFailed)
)

type OperationState struct {
	Status      *string
	Description *string
}
