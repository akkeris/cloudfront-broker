// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/klog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudfront"
)

const ttl int64 = 2592000

func (s *AwsConfigSpec) CreateCloudFrontDistribution(ctx context.Context, callerReference string, billingCode string, plan string) (err error) {

	cf := &cloudFrontInstanceSpec{
		callerReference: aws.String(callerReference),
		billingCode:     aws.String(billingCode),
		plan:            aws.String(plan),
	}

	klog.Infof("out: %+#v", cf)
	go s.createDistributionController(ctx, cf)

	return nil
}

func (s *AwsConfigSpec) createDistributionController(ctx context.Context, cf *cloudFrontInstanceSpec) {
	var err error

	cf.distChan = make(chan error)
	defer close(cf.distChan)

	// TODO create task in db

	go s.createS3Bucket(ctx, cf)
	err = <-cf.distChan

	if err != nil {
		klog.Errorf("error creating bucket: %s\n", err.Error())
		return
	}

	err = s.createIAMUser(ctx, cf)

	if err != nil {
		msg := fmt.Sprintf("error creating iam user: %s", err.Error())
		klog.Error(msg)
		// TODO write error to tasks
		return
	}

	// originAccessIdentity
	go s.createOriginAccessIdentity(ctx, cf)
	err = <-cf.distChan

	if err != nil {
		msg := fmt.Sprintf("error creating OAI: %s\n", err.Error())
		klog.Error(msg)
		// TODO update task in db
		return
	}

	go s.createDistribution(ctx, cf)
	err = <-cf.distChan

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		klog.Error(msg)
		// TODO write error to tasks
		return
	}

	err = s.addBucketPolicy(ctx, cf)
	// err = <-cf.distChan

	if err != nil {
		msg := fmt.Sprintf("error adding bucket policy: %s", err.Error())
		klog.Error(msg)
		// TODO write status to db
		return
	}

	return
}

func (s *AwsConfigSpec) createDistribution(ctx context.Context, cf *cloudFrontInstanceSpec) {
	var err error
	var cfOut *cloudfront.CreateDistributionWithTagsOutput
	var ttlPtr *int64

	ttlPtr = aws.Int64(int64(ttl))

	svc := cloudfront.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	klog.Info("attach origin access idenity")
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
		klog.Errorf(msg)
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
				Comment: cf.s3Bucket.name,
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
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}
	cfOut, err = svc.CreateDistributionWithTagsWithContext(ctx, cin)

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}
	cf.distributionID = cfOut.Distribution.Id
	cf.distributionURL = cfOut.Location

	err = svc.WaitUntilDistributionDeployedWithContext(ctx, &cloudfront.GetDistributionInput{Id: cfOut.Distribution.Id}, func(waiterIn *request.Waiter) {
		waiterIn.MaxAttempts = 50
	})

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		klog.Error(msg)
		cf.distChan <- errors.New(msg)

		return
	}

	cf.distChan <- nil
}

func (s *AwsConfigSpec) DeleteCloudFrontDistribution(ctx context.Context, callerReference string, id string) error {

	cf := &cloudFrontInstanceSpec{
		callerReference: aws.String(callerReference),
		distributionID:  aws.String(id),
	}

	klog.Infof("out: %+#v", cf)

	go s.deleteDistributionController(ctx, cf)

	return nil
}

func (s *AwsConfigSpec) deleteDistributionController(ctx context.Context, cf *cloudFrontInstanceSpec) {
	var err error

	cf.distChan = make(chan error)
	defer close(cf.distChan)

	go s.disableDistribution(ctx, cf)
	err = <-cf.distChan
	if err != nil {
		msg := fmt.Sprintf("error disabling distribution: %s", err)
		klog.Error(msg)
		// TODO write status to db
		return
	}

	go s.deleteDistribution(ctx, cf)
	err = <-cf.distChan
	if err != nil {
		msg := fmt.Sprintf("error deleting distribution: %s", err.Error())
		klog.Error(msg)
		// TODO write status to db
		return
	}

	err = s.deleteOriginAccessIdentity(ctx, cf)
	if err != nil {
		msg := fmt.Sprintf("error deleting origin access id: %s", err.Error())
		klog.Error(msg)
		// TODO write status to db
		return
	}

	err = s.deleteS3Bucket(ctx, cf)
	if err != nil {
		msg := fmt.Sprintf("error deleting bucket: %s", err.Error())
		klog.Error(msg)
		// TODO wraite status to db
		return
	}

	err = s.deleteIAMUser(ctx, cf)
	if err != nil {
		msg := fmt.Sprintf("error deleting user: %s", err.Error())
		klog.Error(msg)
		// TODO write status to db
		return
	}

	return
}

func (s *AwsConfigSpec) getDistibutionConfig(ctx context.Context, svc *cloudfront.CloudFront, cf *cloudFrontInstanceSpec) (getDistConfOut *cloudfront.GetDistributionOutput, err error) {
	var err error

	getDistConfIn := &cloudfront.GetDistributionConfigInput{
		Id: cf.distributionID,
	}

	getDistConfOut, err = svc.GetDistributionConfigWithContext(ctx, getDistConfIn)
	if err != nil {
		msg := fmt.Sprintf("error getting distribution config: %s", err.Error())
		klog.Error(msg)
		return nil, errors.New(msg)
	}

	return getDistConfOut, nil
}

func (s AwsConfigSpec) deleteDistribution(ctx context.Context, cf *cloudFrontInstanceSpec) {
	var err error

	svc := cloudfront.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		klog.Error(msg)
		// TODO write status to db

		cf.distChan <- err
		return
	}

	getDistConfIn := &cloudfront.GetDistributionConfigInput{
		Id: cf.distributionID,
	}

	getDistConfOut, err := svc.GetDistributionConfigWithContext(ctx, getDistConfIn)
	if err != nil {
		msg := fmt.Sprintf("error getting distribution config: %s", err.Error())
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	delDistIn := &cloudfront.DeleteDistributionInput{
		Id:      cf.distributionID,
		IfMatch: getDistConfOut.ETag,
	}

	_, err = svc.DeleteDistributionWithContext(ctx, delDistIn)

	cf.distChan <- err
}

func (s *AwsConfigSpec) updateDistributionEnableFlag(ctx context.Context, cf *cloudFrontInstanceSpec, enabled bool) error {
	var err error

	svc := cloudfront.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		klog.Error(msg)
		// TODO write status to db
		return errors.New(msg)
	}

	getDistConfIn := &cloudfront.GetDistributionConfigInput{
		Id: cf.distributionID,
	}

	getDistConfOut, err := svc.GetDistributionConfigWithContext(ctx, getDistConfIn)
	if err != nil {
		msg := fmt.Sprintf("error getting distribution config: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	distributionConfigOut := &cloudfront.DistributionConfig{}

	distributionConfigOut = getDistConfOut.DistributionConfig
	distributionConfigOut.SetEnabled(enabled)

	updateDistIn := &cloudfront.UpdateDistributionInput{
		DistributionConfig: distributionConfigOut,
		Id:                 cf.distributionID,
		IfMatch:            getDistConfOut.ETag,
	}

	_, err = svc.UpdateDistributionWithContext(ctx, updateDistIn)

	if err != nil {
		msg := fmt.Sprintf("error enabling distribution: %s", err.Error())
		klog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfigSpec) enableDistribution(ctx context.Context, cf *cloudFrontInstanceSpec) {
	cf.distChan <- s.updateDistributionEnableFlag(ctx, cf, true)
}

func (s *AwsConfigSpec) disableDistribution(ctx context.Context, cf *cloudFrontInstanceSpec) {
	cf.distChan <- s.updateDistributionEnableFlag(ctx, cf, false)
}

func (s *AwsConfigSpec) createOriginAccessIdentity(ctx context.Context, cf *cloudFrontInstanceSpec) {
	var err error

	svc := cloudfront.New(s.sess)
	klog.Infof("cf sess: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprint("error creating new cloudfront session")
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	originAccessIdentity, err := svc.CreateCloudFrontOriginAccessIdentityWithContext(ctx, &cloudfront.CreateCloudFrontOriginAccessIdentityInput{
		CloudFrontOriginAccessIdentityConfig: &cloudfront.OriginAccessIdentityConfig{
			CallerReference: cf.callerReference,
			Comment:         aws.String(*cf.s3Bucket.name),
		},
	})

	if err != nil {
		msg := fmt.Sprintf("error creating OriginAccessIdenity: %s", err.Error())
		klog.Error(msg)
		cf.distChan <- errors.New(msg)
		return
	}

	cf.originAccessIdentity = originAccessIdentity.CloudFrontOriginAccessIdentity.Id

	klog.Infof("oai id: %s\n", *cf.originAccessIdentity)

	cf.distChan <- nil
}

func (s *AwsConfigSpec) deleteOriginAccessIdentity(ctx context.Context, cf *cloudFrontInstanceSpec) error {
	var err error

	svc := cloudfront.New(s.sess)
	klog.Infof("cf sess: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprint("error creating new cloudfront session")
		klog.Error(msg)
		return err
	}

	gcfoaiIn := &cloudfront.GetCloudFrontOriginAccessIdentityInput{
		Id: cf.originAccessIdentity,
	}

	gcfoaiOut, err := svc.GetCloudFrontOriginAccessIdentityWithContext(ctx, gcfoaiIn)

	dcfoaiIn := &cloudfront.DeleteCloudFrontOriginAccessIdentityInput{
		Id:      cf.originAccessIdentity,
		IfMatch: gcfoaiOut.ETag,
	}

	_, err = svc.DeleteCloudFrontOriginAccessIdentityWithContext(ctx, dcfoaiIn)
	if err != nil {
		msg := fmt.Sprintf("error deleting origin access id: %s", err.Error())
		klog.Error(msg)
		return err
	}

	return nil
}
