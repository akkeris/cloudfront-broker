// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

type AwsConfigSpec struct {
	namePrefix string
	conf       *aws.Config
	sess       *session.Session
}

type cloudFrontInstanceSpec struct {
	instanceId           *string
	billingCode          *string
	plan                 *string
	distributionID       *string
	distributionURL      *string
	callerReference      *string
	originAccessIdentity *string
	iAMUser              *iAMUserSpec
	s3Bucket             *s3BucketSpec
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
