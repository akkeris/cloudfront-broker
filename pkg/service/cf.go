// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudfront"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

const ttl int64 = 2592000

func (s *AwsConfig) IsDuplicateInstance(distributionID string) (bool, error) {
	glog.Info(" ===== IsDuplicateInstance =====")

	_, err := s.stg.GetDistributionWithDeleted(distributionID)

	if err != nil {
		if err.Error() == "DistributionNotFound" {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, err
}

func (s *AwsConfig) IsDeployedInstance(distributionID string) (bool, error) {
	glog.Infof("===== IsDeployedInstance =====")

	dist, err := s.stg.GetDistributionWithDeleted(distributionID)

	if err != nil {
		return false, err
	}

	if dist.Status == StatusDeployed {
		return true, nil
	}

	return false, errors.New("DistributionNotDeployed")
}

func (s *AwsConfig) getCloudfrontInstance(distributionID string) (*cloudFrontInstance, error) {

	distribution, err := s.stg.GetDistribution(distributionID)

	if err != nil {
		msg := fmt.Sprintf("getCloudfrontInstance: error finding distribution: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	cf := &cloudFrontInstance{
		distributionID:       &distribution.DistributionID,
		billingCode:          s.stg.NullString(distribution.BillingCode),
		planID:               &distribution.PlanID,
		cloudfrontID:         s.stg.NullString(distribution.CloudfrontID),
		cloudfrontURL:        s.stg.NullString(distribution.CloudfrontUrl),
		originAccessIdentity: s.stg.NullString(distribution.OriginAccessIdentity),
		callerReference:      &distribution.CallerReference,
	}

	origin, err := s.stg.GetOriginByDistributionID(*cf.distributionID)

	if err == nil {
		cf.s3Bucket = &s3Bucket{
			originID:   &origin.OriginID,
			bucketName: &origin.BucketName,
			bucketURI:  &origin.BucketUrl,
			iAMUser: &iAMUser{
				userName:  &origin.IAMUser.String,
				accessKey: &origin.AccessKey.String,
				secretKey: &origin.SecretKey.String,
			},
		}
	}

	return cf, nil
}

func (s *AwsConfig) CreateCloudFrontDistribution(distributionID string, callerReference string, operationKey string, serviceID string, planID string, billingCode string) error {
	cf := &cloudFrontInstance{
		callerReference: aws.String(callerReference),
		distributionID:  aws.String(distributionID),
		billingCode:     aws.String(billingCode),
		planID:          aws.String(planID),
		serviceId:       aws.String(serviceID),
		operationKey:    aws.String(operationKey),
	}

	err := s.ActionCreateNew(cf)

	if err != nil {
		msg := fmt.Sprintf("CreateCloudFrontDistribution: error creating new task: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return err
}

func (s *AwsConfig) createDistribution(cf *cloudFrontInstance) error {
	var err error
	var cfOut *cloudfront.CreateDistributionWithTagsOutput
	var ttlPtr *int64

	glog.Info("==== createDistribution ====")

	ttlPtr = aws.Int64(int64(ttl))

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := "createDistribution: error getting cloudfront session"
		glog.Error(msg)
		return errors.New(msg)
	}

	glog.Info("createDistribution: attach origin access identity")
	var s3Origin = []*cloudfront.Origin{}

	fullname := strings.Replace(*cf.s3Bucket.bucketURI, "http://", "", -1)
	fullname = strings.Replace(fullname, "/", "", -1)

	s3Origin = append(s3Origin, &cloudfront.Origin{
		DomainName: &fullname,
		Id:         cf.s3Bucket.bucketName,
		S3OriginConfig: &cloudfront.S3OriginConfig{
			OriginAccessIdentity: aws.String("origin-access-identity/cloudfront/" + *cf.originAccessIdentity),
		},
	})

	err = s3Origin[0].Validate()
	if err != nil {
		msg := fmt.Sprintf("createDistribution: error in S3Origin: %s", err.Error())
		glog.Errorf(msg)
		return err
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
		msg := fmt.Sprintf("createDistribution: error with cin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	cfOut, err = svc.CreateDistributionWithTags(cin)

	if err != nil {
		msg := fmt.Sprintf("createDistribution: error creating distribution: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	cf.cloudfrontID = cfOut.Distribution.Id
	cf.cloudfrontURL = cfOut.Location

	glog.Infof("cloudfrontID: %s\n", *cf.cloudfrontID)

	_, err = s.stg.UpdateDistributionCloudfront(*cf.distributionID, *cf.cloudfrontID, *cf.cloudfrontURL)
	if err != nil {
		msg := fmt.Sprintf("createDistribution: error updating distribution with cloudfront: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) DeleteCloudFrontDistribution(distributionID string, operationKey string) error {

	cf := &cloudFrontInstance{
		distributionID: aws.String(distributionID),
		operationKey:   aws.String(operationKey),
	}

	err := s.ActionDeleteNew(cf)
	if err != nil {
		msg := fmt.Sprintf("DeleteCloudFrontDistribution: error creating new task: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) deleteDistributionController(cf *cloudFrontInstance) {
	var err error

	glog.Info("==== deleteDistributionController [%s] ====", *cf.operationKey)

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

func (s *AwsConfig) getCloudfrontDistribution(cf *cloudFrontInstance) (*cloudfront.GetDistributionOutput, error) {
	glog.Infof("==== getCloudfrontDistribution [%s] ====", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := "getCloudfrontDistibution: error getting cloudfront session:"
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	getDistOut, err := svc.GetDistribution(&cloudfront.GetDistributionInput{Id: cf.cloudfrontID})
	if err != nil {
		msg := fmt.Sprintf("getCloudfrontDistibution: getting distribution: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return getDistOut, nil

}

func (s *AwsConfig) getDistibutionConfig(svc *cloudfront.CloudFront, cf *cloudFrontInstance) (*cloudfront.GetDistributionConfigOutput, error) {
	var err error

	glog.Infof("==== getDistributionConfig [%s] ====", *cf.operationKey)
	glog.Infof("getDistibutionConfig: cloudfront id: %s", *cf.cloudfrontID)

	getDistConfIn := &cloudfront.GetDistributionConfigInput{
		Id: aws.String(*cf.cloudfrontID),
	}

	getDistConfOut, err := svc.GetDistributionConfig(getDistConfIn)
	if err != nil {
		msg := fmt.Sprintf("getDistibutionConfig: error getting distribution config: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return getDistConfOut, nil
}

func (s AwsConfig) getDistEnabled(cf *cloudFrontInstance) (bool, error) {
	var enabled bool = false

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := "getDistEnabled: error getting cloudfront session"
		glog.Error(msg)
		return enabled, errors.New(msg)
	}

	distConfig, err := s.getDistibutionConfig(svc, cf)

	if err != nil {
		msg := fmt.Sprintf("getDistEnabled: error getting distribution config: %s", err.Error())
		glog.Error(msg)
		return enabled, errors.New(msg)
	}

	enabled = *distConfig.DistributionConfig.Enabled

	return enabled, nil
}

func (s AwsConfig) deleteDistribution(cf *cloudFrontInstance) {
	var err error
	var deleteWaitCnt = 50
	var deleteWaitSec = s.waitSecs

	glog.Infof("==== deleteDistribution [%s] ====", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		glog.Error(msg)
		// TODO write status to db

		cf.distChan <- err
		return
	}

	getDistConfOut, err := s.getDistibutionConfig(svc, cf)

	delDistIn := &cloudfront.DeleteDistributionInput{
		Id:      cf.cloudfrontID,
		IfMatch: getDistConfOut.ETag,
	}

	for i := 0; i < deleteWaitCnt; i++ {
		_, err = svc.DeleteDistribution(delDistIn)

		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudfront.ErrCodeDistributionNotDisabled:
				glog.Infof("%d: not disabled: %s\n", i, aerr.Error())
			default:
				glog.Infof("%d: err deleting: %s", i, aerr.Error())
				cf.distChan <- err
				return
			}

		} else {
			fmt.Printf("deleted")
			break
		}
		time.Sleep(time.Second * time.Duration(deleteWaitSec))
	}

	cf.distChan <- err
}

func (s *AwsConfig) updateDistributionEnableFlag(cf *cloudFrontInstance, enabled bool) error {
	var err error

	glog.Infof("==== updateDistributionEnabledFlag [%s] <%t> ====", *cf.operationKey, enabled)

	svc := cloudfront.New(s.sess)
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
		Id:                 cf.cloudfrontID,
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

func (s *AwsConfig) disableCloudfrontDistribution(cf *cloudFrontInstance) error {
	glog.Info("==== disableCloudfrontDistribution [%s] ====", cf.operationKey)

	if err := s.disableDistribution(cf); err != nil {
		msg := fmt.Sprintf("disableCloudfrontDistribution: setting disable flag: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) createOriginAccessIdentity(cf *cloudFrontInstance) error {
	var err error

	glog.Info("==== createOriginAccessIdentity ====")

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := fmt.Sprint("createOriginAccessIdentity: error creating new cloudfront session")
		glog.Error(msg)
		return errors.New(msg)
	}

	originAccessIdentity, err := svc.CreateCloudFrontOriginAccessIdentity(&cloudfront.CreateCloudFrontOriginAccessIdentityInput{
		CloudFrontOriginAccessIdentityConfig: &cloudfront.OriginAccessIdentityConfig{
			CallerReference: cf.callerReference,
			Comment:         aws.String(*cf.s3Bucket.bucketName),
		},
	})

	if err != nil {
		msg := fmt.Sprintf("createOriginAccessIdentity: error creating OriginAccessIdenity: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	cf.originAccessIdentity = originAccessIdentity.CloudFrontOriginAccessIdentity.Id

	glog.Infof("createOriginAccessIdentity: oai id: %s", *cf.originAccessIdentity)

	_, err = s.stg.AddOriginAccessIdentity(*cf.distributionID, *cf.originAccessIdentity)
	if err != nil {
		msg := fmt.Sprintf("createOriginAccessIdenity: error adding: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
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

func (s *AwsConfig) CheckLastOperation(distributionID string) (*osb.LastOperationResponse, error) {
	glog.Infof("===== CheckLastOperation [%s] =====", distributionID)

	response, err := s.GetTaskState(distributionID)
	if err != nil {
		msg := fmt.Sprintf("CheckLastOperation: error getting task state: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return response, nil
}
