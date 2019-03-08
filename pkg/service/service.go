// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package service

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golang/glog"
)

func Init(namePrefix string, waitCnt int, waitSecs time.Duration) (*AwsConfig, error) {
	c := AwsConfig{
		namePrefix: namePrefix,
		waitCnt:    waitCnt,
		waitSecs:   waitSecs,
		conf:       &aws.Config{},
	}

	region := os.Getenv("REGION")
	if region == "" {
		msg := "REGION environment variable not set"
		glog.Errorln(msg)
		return nil, errors.New(msg)
	}

	awsAccessKey := os.Getenv("AWS_ACCESS_KEY")
	if awsAccessKey == "" {
		msg := "AWS_ACCESS_KEY not set"
		glog.Errorln(msg)
		return nil, errors.New(msg)
	}

	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretAccessKey == "" {
		msg := "AWS_SECRET_ACCESS_KEY not set"
		glog.Errorln(msg)
		return nil, errors.New(msg)
	}

	c.conf.Region = &region

	glog.Infof("namePrefix: %s", c.namePrefix)
	glog.Infof("region: %s", *c.conf.Region)
	glog.Infof("AWS_ACCESS_KEY=%s", os.Getenv("AWS_ACCESS_KEY"))

	c.sess = session.Must(session.NewSession(c.conf))
	return &c, nil
}

func statusMsg(status string, process string) string {
	var msg string
	switch status {
	case OperationInProgress:
		msg = fmt.Sprintf("%s is in progess", process)
	case OperationSucceeded:
		msg = fmt.Sprintf("%s has completed successfully", process)
	case OperationFailed:
		msg = fmt.Sprintf("%s has failed", process)
	default:
		msg = fmt.Sprintf("%s status unknown", process)
	}

	return msg
}
