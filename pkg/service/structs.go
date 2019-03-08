// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"time"

	osb "github.com/pmorie/go-open-service-broker-client/v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

type AwsConfig struct {
	namePrefix string
	conf       *aws.Config
	sess       *session.Session
	waitCnt    int
	waitSecs   time.Duration
}

type cloudFrontInstance struct {
	instanceId           *string // osb instance id
	billingCode          *string
	planId               *string
	serviceId            *string
	distributionId       *string // cloudfront id
	distributionURL      *string
	callerReference      *string
	originAccessIdentity *string
	iAMUser              *iAMUser
	s3Bucket             *s3Bucket
	operationKey         *string
	distChan             chan error
}

type s3Bucket struct {
	bucketName *string
	fullname   *string
	bucketURI  *string
	originID   *string
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
