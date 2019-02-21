// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package service

import (
	"errors"
	"os"

	"cloudfront-broker/pkg/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"k8s.io/klog"
)

func Init(np string) (*AwsConfigSpec, error) {
	utils.Init()
	c := AwsConfigSpec{}
	c.conf = &aws.Config{}

	c.conf.Region = utils.StrPtr(os.Getenv("REGION"))
	if *c.conf.Region == "" {
		klog.Errorln("REGION environment variable not set")
		return nil, errors.New("REGION environment variable not set")
	}

	c.namePrefix = np

	klog.Infof("namePrefix: %s", c.namePrefix)
	klog.Infof("region: %s", *c.conf.Region)
	klog.Infof("AWS_ACCESS_KEY_ID=%s", os.Getenv("AWS_ACCESS_KEY_ID"))
	klog.Infof("AWS_SECRET_ACCESS_KEY=%s", os.Getenv("AWS_SECRET_ACCESS_KEY"))

	c.sess = session.Must(session.NewSession(c.conf))
	return &c, nil
}
