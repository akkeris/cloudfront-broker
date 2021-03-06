package service

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudfront"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

const ttl int64 = 2592000

// IsDuplicateInstance checks if distribution id already created
func (s *AwsConfig) IsDuplicateInstance(distributionID string) (bool, error) {
	glog.V(4).Info(" ===== IsDuplicateInstance =====")

	_, err := s.stg.GetDistributionWithDeleted(distributionID)

	if err != nil {
		if err.Error() == "DistributionNotFound" {
			return false, nil
		}

		return false, err
	}

	return true, err
}

// IsDeployedInstance checks if distribution has been fully deployed
func (s *AwsConfig) IsDeployedInstance(distributionID string) (bool, error) {
	glog.V(4).Infof("===== IsDeployedInstance =====")

	dist, err := s.stg.GetDistributionWithDeleted(distributionID)

	if err != nil {
		return false, err
	}

	if dist.Status == statusDeployed {
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
		serviceID:            &distribution.ServiceID,
		planID:               &distribution.PlanID,
		cloudfrontID:         s.stg.NullString(distribution.CloudfrontID),
		cloudfrontURL:        s.stg.NullString(distribution.CloudfrontURL),
		originAccessIdentity: s.stg.NullString(distribution.OriginAccessIdentity),
		callerReference:      &distribution.CallerReference,
	}

	origin, err := s.stg.GetOriginByDistributionID(*cf.distributionID)

	if err == nil {
		cf.s3Bucket = &s3Bucket{
			originID:   &origin.OriginID,
			bucketName: &origin.BucketName,
			bucketURI:  &origin.BucketURL,
			iAMUser: &iAMUser{
				userName:  &origin.IAMUser.String,
				accessKey: &origin.AccessKey.String,
				secretKey: &origin.SecretKey.String,
			},
		}
	}

	return cf, nil
}

// GetCloudFrontInstanceSpec retrieves  instance spec from database
func (s *AwsConfig) GetCloudFrontInstanceSpec(distributionID string) (*InstanceSpec, error) {
	cf, err := s.getCloudfrontInstance(distributionID)

	if err != nil {
		msg := fmt.Sprintf("GetCloudFrontInstanceSpec: error getting distribution %s", err.Error())
		glog.Error(msg)
		return nil, err
	}

	cfi := &InstanceSpec{
		ServiceID:            cf.planID,
		PlanID:               cf.planID,
		BillingCode:          cf.billingCode,
		CloudfrontID:         cf.cloudfrontID,
		CloudfrontURL:        cf.cloudfrontURL,
		OriginAccessIdentity: cf.originAccessIdentity,
		S3Bucket: &S3BucketSpec{
			BucketName: cf.s3Bucket.bucketName,
			Fullname:   cf.s3Bucket.fullname,
			BucketURI:  cf.s3Bucket.bucketURI,
			IAMUser: &IAMUserSpec{
				UserName:  cf.s3Bucket.iAMUser.userName,
				ARN:       cf.s3Bucket.iAMUser.arn,
				AccessKey: cf.s3Bucket.iAMUser.accessKey,
				SecretKey: cf.s3Bucket.iAMUser.secretKey,
			},
		},
		Access: &AccessSpec{
			CloudFrontURL:      cf.cloudfrontURL,
			BucketName:         cf.s3Bucket.bucketName,
			AwsAccessKey:       cf.s3Bucket.iAMUser.accessKey,
			AwsSecretAccessKey: cf.s3Bucket.iAMUser.secretKey,
		},
	}

	return cfi, nil
}

// CreateCloudFrontDistribution starts the provision process by creating a new task
func (s *AwsConfig) CreateCloudFrontDistribution(distributionID string, callerReference string, operationKey string, serviceID string, planID string, billingCode *string) error {
	cf := &cloudFrontInstance{
		callerReference: aws.String(callerReference),
		distributionID:  aws.String(distributionID),
		planID:          aws.String(planID),
		serviceID:       aws.String(serviceID),
		operationKey:    aws.String(operationKey),
		billingCode:     billingCode,
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

	glog.V(4).Info("==== createDistribution ====")

	ttlPtr = aws.Int64(int64(ttl))

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := "createDistribution: error getting cloudfront session"
		glog.Error(msg)
		return errors.New(msg)
	}

	glog.V(4).Info("createDistribution: attach origin access identity")
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

	if cf.billingCode != nil {
		tags = append(tags, &cloudfront.Tag{
			Key:   aws.String("billingcode"),
			Value: cf.billingCode,
		})
	}

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

	curl := fmt.Sprintf("https://%s", *cfOut.Distribution.DomainName)
	cf.cloudfrontID = cfOut.Distribution.Id
	cf.cloudfrontURL = &curl

	glog.V(0).Infof("cloudfrontID: %s\n", *cf.cloudfrontID)

	_, err = s.stg.UpdateDistributionCloudfront(*cf.distributionID, *cf.cloudfrontID, *cf.cloudfrontURL)
	if err != nil {
		msg := fmt.Sprintf("createDistribution: error updating distribution with cloudfront: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

// DeleteCloudFrontDistribution starts the de-provision process my creating a new task
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

func (s *AwsConfig) getCloudfrontDistribution(cf *cloudFrontInstance) (*cloudfront.GetDistributionOutput, error) {
	glog.V(4).Infof("==== getCloudfrontDistribution [%s] ====", *cf.operationKey)

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

func (s *AwsConfig) isDistributionDeployed(cf *cloudFrontInstance) (bool, error) {
	glog.V(4).Infof("==== isDistributionDeployed [%s] ====", *cf.operationKey)

	distOut, err := s.getCloudfrontDistribution(cf)

	if err != nil {
		msg := fmt.Sprintf("isDistributionDeplyed[%s]: error checking distribution deployed: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return false, errors.New(msg)
	}

	glog.V(4).Infof("isDistributionDeployed[%s]: %s", *cf.operationKey, *distOut.Distribution.Status)
	if *distOut.Distribution.Status == "Deployed" && *distOut.Distribution.DistributionConfig.Enabled {
		return true, nil
	}

	return false, nil
}

func (s *AwsConfig) isDistributionDisabled(cf *cloudFrontInstance) (bool, error) {
	glog.V(4).Infof("==== isDistributionDisabled [%s] ====", *cf.operationKey)

	distOut, err := s.getCloudfrontDistribution(cf)

	if err != nil {
		msg := fmt.Sprintf("isDistributionDisabled[%s]: error checking distribution deployed: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return false, errors.New(msg)
	}

	glog.V(4).Infof("isDistributionDisabled[%s]: %s", *cf.operationKey, *distOut.Distribution.Status)
	if *distOut.Distribution.Status == "Deployed" && !*distOut.Distribution.DistributionConfig.Enabled {
		return true, nil
	}

	return false, nil
}

func (s *AwsConfig) getDistributionConfig(svc *cloudfront.CloudFront, cf *cloudFrontInstance) (*cloudfront.GetDistributionConfigOutput, error) {
	var err error

	glog.V(4).Infof("==== getDistributionConfig [%s] ====", *cf.operationKey)
	glog.V(1).Infof("getDistributionConfig: cloudfront id: %s", *cf.cloudfrontID)

	getDistConfIn := &cloudfront.GetDistributionConfigInput{
		Id: aws.String(*cf.cloudfrontID),
	}

	getDistConfOut, err := svc.GetDistributionConfig(getDistConfIn)
	if err != nil {
		msg := fmt.Sprintf("getDistributionConfig: error getting distribution config: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return getDistConfOut, nil
}

func (s *AwsConfig) deleteDistribution(cf *cloudFrontInstance) error {
	glog.V(4).Infof("==== deleteDistribution [%s] ====", *cf.cloudfrontID)
	glog.V(0).Infof("deleteDistribution operationKey: %s", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := "error getting cloudfront session"
		glog.Error(msg)
		return errors.New(msg)
	}

	getDistConfOut, _ := s.getDistributionConfig(svc, cf)

	delDistIn := &cloudfront.DeleteDistributionInput{
		Id:      cf.cloudfrontID,
		IfMatch: getDistConfOut.ETag,
	}

	_, err := svc.DeleteDistribution(delDistIn)

	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudfront.ErrCodeDistributionNotDisabled:
			msg := fmt.Sprintf("deleteDistribution[%s]: distribution not disabled: %s", *cf.operationKey, aerr.Error())
			glog.V(4).Info(msg)
			return errors.New(aerr.Code())
		default:
			glog.V(4).Infof("deleteDistribution: err deleting: %s", aerr.Error())
			return err
		}
	}

	return nil
}

func (s *AwsConfig) updateDistributionDeletedAt(cf *cloudFrontInstance) error {
	err := s.stg.UpdateDeleteDistribution(*cf.distributionID)

	if err != nil {
		msg := fmt.Sprintf("updateDistributionDeletedAt: error from UpdateDeleteDistribution: %s", err.Error())
		glog.Error(msg)
		return err
	}

	return nil
}

func (s *AwsConfig) updateDistributionEnableFlag(cf *cloudFrontInstance, enabled bool) error {
	var err error

	glog.V(4).Infof("==== updateDistributionEnabledFlag [%s] <%t> ====", *cf.operationKey, enabled)

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := "error getting cloudfront session"
		glog.Error(msg)
		return errors.New(msg)
	}

	getDistConfOut, err := s.getDistributionConfig(svc, cf)

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
		msg := fmt.Sprintf("error setting distribution enabled flag: %s", err.Error())
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
	glog.V(4).Infof("==== disableCloudfrontDistribution [%s] ====", *cf.operationKey)

	if err := s.disableDistribution(cf); err != nil {
		msg := fmt.Sprintf("disableCloudfrontDistribution: setting disable flag: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) createOriginAccessIdentity(cf *cloudFrontInstance) error {
	var err error

	glog.V(4).Info("==== createOriginAccessIdentity ====")

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

	glog.V(0).Infof("createOriginAccessIdentity: id: %s", *cf.originAccessIdentity)

	err = s.stg.UpdateDistributionWIthOriginAccessIdentity(*cf.distributionID, *cf.originAccessIdentity)
	if err != nil {
		msg := fmt.Sprintf("createOriginAccessIdenity: error adding: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (s *AwsConfig) isOriginAccessIdentityReady(cf *cloudFrontInstance) (bool, error) {
	glog.V(4).Info("==== isOriginAccessIdentityReady ====")

	svc := cloudfront.New(s.sess)
	if svc == nil {
		msg := fmt.Sprint("isOriginAccessIdentityReady: error creating new cloudfront session")
		glog.Error(msg)
		return false, errors.New(msg)
	}

	_, err := svc.GetCloudFrontOriginAccessIdentity(&cloudfront.GetCloudFrontOriginAccessIdentityInput{
		Id: cf.originAccessIdentity,
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudfront.ErrCodeNoSuchCloudFrontOriginAccessIdentity:
				msg := fmt.Sprintf("isOriginAccessIdentityReady: [%s]: origin access identity not ready: %s", *cf.operationKey, aerr.Error())
				glog.V(4).Info(msg)
				return false, nil
			default:
				glog.V(4).Infof("isOriginAccessIdentityReady: error getting origin access identity: %s", aerr.Error())
				return false, err
			}
		}
	}

	return true, nil
}

func (s *AwsConfig) deleteOriginAccessIdentity(cf *cloudFrontInstance) error {
	glog.V(4).Infof("==== deleteOriginAccessIdentity [%s] ====", *cf.operationKey)

	svc := cloudfront.New(s.sess)
	glog.V(4).Infof("cf sess: %#+v\n", svc)
	if svc == nil {
		msg := "error creating new cloudfront session"
		glog.Error(msg)
		return errors.New(msg)
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

// CheckLastOperation retrieves state from database
func (s *AwsConfig) CheckLastOperation(distributionID string) (*osb.LastOperationResponse, error) {
	glog.V(4).Infof("===== CheckLastOperation [%s] =====", distributionID)

	response, err := s.getTaskState(distributionID)
	if err != nil {
		msg := fmt.Sprintf("CheckLastOperation: error getting task state: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return response, nil
}
