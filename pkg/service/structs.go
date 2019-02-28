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

type AwsConfigSpec struct {
	namePrefix string
	conf       *aws.Config
	sess       *session.Session
	waitCnt    int
	waitSecs   time.Duration
}

type cloudFrontInstanceSpec struct {
	instanceId           *string
	billingCode          *string
	planId               *string
	serviceId            *string
	distributionId       *string
	distributionURL      *string
	callerReference      *string
	originAccessIdentity *string
	iAMUser              *iAMUserSpec
	s3Bucket             *s3BucketSpec
	operationKey         *string
	distChan             chan error
}

type s3BucketSpec struct {
	name     *string
	fullname *string
	uri      *string
	id       *string
}

type iAMUserSpec struct {
	userName   *string
	arn        *string
	accessKey  *string
	secretKey  *string
	policyName *string
}

var (
	StateInProgress = string(osb.StateInProgress)
	StateSucceeded  = string(osb.StateSucceeded)
	StateFailed     = string(osb.StateFailed)
)

type StatusSpec struct {
	Status      *string
	Description *string
}
