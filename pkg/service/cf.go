// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"context"
	"fmt"
	"time"

	"cloudfront-broker/pkg/storage"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudfront"
)

const ttl int64 = 2592000

func (s *AwsConfig) CreateCloudFrontDistribution(distributionId string, callerReference string, operationKey string, serviceId string, planId string, billingCode string) (err error) {

	cf := &cloudFrontInstance{
		callerReference: aws.String(callerReference),
		instanceId:      aws.String(distributionId),
		billingCode:     aws.String(billingCode),
		planId:          aws.String(planId),
		serviceId:       aws.String(serviceId),
		operationKey:    aws.String(operationKey),
	}

	glog.Infof("out: %+#v\n", cf)
	go s.createDistributionController(cf)

	return nil
}

func (s *AwsConfig) createDistributionController(cf *cloudFrontInstance) {
	var err error

	glog.Infof("==== createDistributionController [%s] ====", *cf.operationKey)
	cf.distChan = make(chan error)
	defer close(cf.distChan)

	// TODO create task in db

	go s.createS3Bucket(cf)
	err = <-cf.distChan

	if err != nil {
		glog.Errorf("error creating bucket: %s\n", err.Error())
		return
	}

	err = s.createIAMUser(cf)

	if err != nil {
		msg := fmt.Sprintf("error creating iam user: %s", err.Error())
		glog.Error(msg)
		// TODO write error to tasks
		return
	}

	go s.createOriginAccessIdentity(cf)
	err = <-cf.distChan

	if err != nil {
		msg := fmt.Sprintf("error creating OAI: %s\n", err.Error())
		glog.Error(msg)
		// TODO update task in db
		return
	}

	go s.createDistribution(cf)
	err = <-cf.distChan

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		glog.Error(msg)
		// TODO write error to tasks
		return
	}

	err = s.addBucketPolicy(cf)
	// err = <-cf.distChan

	if err != nil {
		msg := fmt.Sprintf("error adding bucket policy: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db
		return
	}

	glog.Infof("==== distribution created: [%s] %s ====", *cf.operationKey, *cf.distributionId)
	return
}

func (s *AwsConfig) createDistribution(cf *cloudFrontInstance) {
	var err error
	var cfOut *cloudfront.CreateDistributionWithTagsOutput
	var ttlPtr *int64

	glog.Infof("==== createDistribution [%s] ====", *cf.operationKey)

	ttlPtr = aws.Int64(int64(ttl))

	svc := cloudfront.New(s.sess)
	glog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		glog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	glog.Info("attach origin access idenity")
	var s3Origin = []*cloudfront.Origin{}

	s3Origin = append(s3Origin, &cloudfront.Origin{
		DomainName: cf.s3Bucket.fullname,
		Id:         cf.s3Bucket.id,
		S3OriginConfig: &cloudfront.S3OriginConfig{
			OriginAccessIdentity: aws.String("origin-access-identity/cloudfront/" + *cf.originAccessIdentity),
		},
	})

	err = s3Origin[0].Validate()
	if err != nil {
		msg := fmt.Sprintf("error in S3Origin: %s", err.Error())
		glog.Errorf(msg)
		cf.distChan <- err
		// TODO write update task with error
	}

	cmi := []*string{}
	cmi = append(cmi, aws.String("GET"))
	cmi = append(cmi, aws.String("HEAD"))

	tags := []*cloudfront.Tag{}
	tags = append(tags, &cloudfront.Tag{
		Key:   aws.String("billingcode"),
		Value: (cf.billingCode),
	})

	cin := &cloudfront.CreateDistributionWithTagsInput{
		DistributionConfigWithTags: &cloudfront.DistributionConfigWithTags{
			DistributionConfig: &cloudfront.DistributionConfig{
				CallerReference: cf.callerReference,
				Origins: &cloudfront.Origins{
					Items:    s3Origin,
					Quantity: aws.Int64(1),
				},
				Comment: cf.s3Bucket.bucketName,
				DefaultCacheBehavior: &cloudfront.DefaultCacheBehavior{
					AllowedMethods: &cloudfront.AllowedMethods{
						CachedMethods: &cloudfront.CachedMethods{
							Items:    cmi,
							Quantity: aws.Int64(2),
						},
						Items:    cmi,
						Quantity: aws.Int64(2),
					},
					DefaultTTL: ttlPtr,
					MinTTL:     ttlPtr,
					MaxTTL:     ttlPtr,
					ForwardedValues: &cloudfront.ForwardedValues{
						Cookies: &cloudfront.CookiePreference{
							Forward: aws.String("none"),
						},
						QueryString: aws.Bool(false),
					},
					TargetOriginId: s3Origin[0].Id,
					TrustedSigners: &cloudfront.TrustedSigners{
						Enabled:  aws.Bool(false),
						Quantity: aws.Int64(0),
					},
					ViewerProtocolPolicy: aws.String("redirect-to-https"),
				},
				Enabled: aws.Bool(true),
			},
			Tags: &cloudfront.Tags{
				Items: tags,
			},
		},
	}

	err = cin.Validate()
	if err != nil {
		msg := fmt.Sprintf("error with cin: %s", err.Error())
		glog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}
	cfOut, err = svc.CreateDistributionWithTags(cin)

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		glog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}
	cf.distributionId = cfOut.Distribution.Id
	cf.distributionURL = cfOut.Location

	glog.Info(">>>> waiting for distribution <<<<")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = svc.WaitUntilDistributionDeployedWithContext(ctx, &cloudfront.GetDistributionInput{Id: cfOut.Distribution.Id}, func(w *request.Waiter) {
		w.MaxAttempts = s.waitCnt
		// w.Delay = func(a int) time.Duration{ return s.waitSecs }
	})

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		glog.Error(msg)
		cf.distChan <- errors.New(msg)

		return
	}

	glog.Infof("distributionId: %s\n", *cf.distributionId)

	cf.distChan <- nil
}

func (s *AwsConfig) DeleteCloudFrontDistribution(callerReference string, cfInstance *storage.Distribution) error {

	cf := &cloudFrontInstance{
		callerReference: aws.String(callerReference),
		instanceId:      aws.String(cfInstance.DistributionID),
		operationKey:    aws.String(cfInstance.OperationKey),
	}

	glog.Infof("out: %+#v", cf)

	go s.deleteDistributionController(cf)

	return nil
}

func (s *AwsConfig) deleteDistributionController(cf *cloudFrontInstance) {
	var err error

	glog.Info("==== deleteDistributinController [%s] ====", *cf.operationKey)

	cf.distChan = make(chan error)
	defer close(cf.distChan)

	err = s.deleteS3Bucket(cf)
	if err != nil {
		msg := fmt.Sprintf("error deleting bucket: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db
		return
	}

	err = s.deleteIAMUser(cf)
	if err != nil {
		msg := fmt.Sprintf("error deleting user: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db
		return
	}

	go s.disableDistribution(cf)
	err = <-cf.distChan
	if err != nil {
		msg := fmt.Sprintf("error disabling distribution: %s", err)
		glog.Error(msg)
		// TODO write status to db
		return
	}

	go s.deleteDistribution(cf)
	err = <-cf.distChan
	if err != nil {
		msg := fmt.Sprintf("error deleting distribution: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db
		return
	}

	err = s.deleteOriginAccessIdentity(cf)
	if err != nil {
		msg := fmt.Sprintf("error deleting origin access id: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db
		return
	}

	return
}

func (s *AwsConfig) getDistibutionConfig(svc *cloudfront.CloudFront, cf *cloudFrontInstance) (*cloudfront.GetDistributionConfigOutput, error) {
	var err error

	glog.Infof("==== getDistributionConfig [%s] ====", *cf.operationKey)

	getDistConfIn := &cloudfront.GetDistributionConfigInput{
		Id: cf.distributionId,
	}

	getDistConfOut, err := svc.GetDistributionConfig(getDistConfIn)
	if err != nil {
		msg := fmt.Sprintf("error getting distribution config: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return getDistConfOut, nil
}

func (s AwsConfig) deleteDistribution(cf *cloudFrontInstance) {
	var err error
	var deleteWaitCnt = s.waitCnt
	var deleteWaitSec = s.waitSecs

	glog.Infof("==== deleteDistribution [%s] ====", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	glog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db

		cf.distChan <- err
		return
	}

	getDistConfOut, err := s.getDistibutionConfig(svc, cf)

	delDistIn := &cloudfront.DeleteDistributionInput{
		Id:      cf.distributionId,
		IfMatch: getDistConfOut.ETag,
	}

	for i := 0; i < deleteWaitCnt; i++ {
		_, err = svc.DeleteDistribution(delDistIn)

		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudfront.ErrCodeDistributionNotDisabled:
				fmt.Printf("%d: not disabled: %s\n", i, aerr.Error())
			default:
				fmt.Printf("%d: err deleting: %s", i, aerr.Error())
				cf.distChan <- err
				return
			}

		} else {
			fmt.Printf("deleted")
			break
		}
		time.Sleep(time.Second * deleteWaitSec)
	}

	cf.distChan <- err
}

func (s *AwsConfig) updateDistributionEnableFlag(cf *cloudFrontInstance, enabled bool) error {
	var err error

	glog.Infof("==== updateDistributionEnabledFlag [%s] <%t> ====", *cf.operationKey, enabled)

	svc := cloudfront.New(s.sess)
	glog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db
		return errors.New(msg)
	}

	getDistConfOut, err := s.getDistibutionConfig(svc, cf)

	distConfigOut := &cloudfront.DistributionConfig{}

	distConfigOut = getDistConfOut.DistributionConfig
	distConfigOut.SetEnabled(enabled)

	updateDistIn := &cloudfront.UpdateDistributionInput{
		DistributionConfig: distConfigOut,
		Id:                 cf.distributionId,
		IfMatch:            getDistConfOut.ETag,
	}

	_, err = svc.UpdateDistribution(updateDistIn)

	if err != nil {
		msg := fmt.Sprintf("error enabling distribution: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) enableDistribution(cf *cloudFrontInstance) error {
	return s.updateDistributionEnableFlag(cf, true)
}

func (s *AwsConfig) disableDistribution(cf *cloudFrontInstance) error {
	return s.updateDistributionEnableFlag(cf, false)
}

func (s *AwsConfig) createOriginAccessIdentity(cf *cloudFrontInstance) {
	var err error

	glog.Infof("==== createOriginAccessIdentity [%s] ====", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	glog.Infof("cf sess: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprint("error creating new cloudfront session")
		glog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	originAccessIdentity, err := svc.CreateCloudFrontOriginAccessIdentity(&cloudfront.CreateCloudFrontOriginAccessIdentityInput{
		CloudFrontOriginAccessIdentityConfig: &cloudfront.OriginAccessIdentityConfig{
			CallerReference: cf.callerReference,
			Comment:         aws.String(*cf.s3Bucket.bucketName),
		},
	})

	if err != nil {
		msg := fmt.Sprintf("error creating OriginAccessIdenity: %s", err.Error())
		glog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	cf.originAccessIdentity = originAccessIdentity.CloudFrontOriginAccessIdentity.Id

	glog.Infof("oai id: %s\n", *cf.originAccessIdentity)

	cf.distChan <- nil
}

func (s *AwsConfig) deleteOriginAccessIdentity(cf *cloudFrontInstance) error {
	var err error

	glog.Infof("==== deleteOriginAccessIdentity [%s] ====", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	glog.Infof("cf sess: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprint("error creating new cloudfront session")
		glog.Error(msg)
		return err
	}

	gcfoaiIn := &cloudfront.GetCloudFrontOriginAccessIdentityInput{
		Id: cf.originAccessIdentity,
	}

	gcfoaiOut, err := svc.GetCloudFrontOriginAccessIdentity(gcfoaiIn)

	dcfoaiIn := &cloudfront.DeleteCloudFrontOriginAccessIdentityInput{
		Id:      cf.originAccessIdentity,
		IfMatch: gcfoaiOut.ETag,
	}

	_, err = svc.DeleteCloudFrontOriginAccessIdentity(dcfoaiIn)
	if err != nil {
		msg := fmt.Sprintf("error deleting origin access id: %s", err.Error())
		glog.Error(msg)
		return err
	}

	return nil
}

func (s *AwsConfig) CheckLastOperation(in *storage.Distribution) (*OperationState, error) {

	sMsg := statusMsg(OperationInProgress, "task")

	retStatus := &OperationState{
		Status:      &OperationInProgress,
		Description: &sMsg,
	}

	return retStatus, nil
}
