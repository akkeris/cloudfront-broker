// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package service

import (
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"k8s.io/klog"
)

func Init(namePrefix string) (*AwsConfigSpec, error) {
	c := AwsConfigSpec{}
	c.conf = &aws.Config{}

	region := os.Getenv("REGION")
	if region == "" {
		msg := "REGION environment variable not set"
		klog.Errorln(msg)
		return nil, errors.New(msg)
	}

	awsAccessKey := os.Getenv("AWS_ACCESS_KEY")
	if awsAccessKey == "" {
		msg := "AWS_ACCESS_KEY not set"
		klog.Errorln(msg)
		return nil, errors.New(msg)
	}

	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretAccessKey == "" {
		msg := "AWS_SECRET_ACCESS_KEY not set"
		klog.Errorln(msg)
		return nil, errors.New(msg)
	}

	c.conf.Region = &region
	c.namePrefix = namePrefix

	klog.Infof("namePrefix: %s", c.namePrefix)
	klog.Infof("region: %s", *c.conf.Region)
	klog.Infof("AWS_ACCESS_KEY=%s", os.Getenv("AWS_ACCESS_KEY"))

	c.sess = session.Must(session.NewSession(c.conf))
	return &c, nil
}
