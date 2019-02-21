// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package service

import (
	"context"
	"fmt"

	"cloudfront-broker/pkg/utils"

	"github.com/pkg/errors"
	"k8s.io/klog"

	"github.com/aws/aws-sdk-go/service/cloudfront"
)

func (s *AwsConfigSpec) CreateCloudFrontDistribution(ctx context.Context, callerReference string, billingCode string, plan string) (err error) {

	in := &InCreateDistributionSpec{
		CallerReference: callerReference,
		BillingCode:     billingCode,
		Plan:            plan,
	}

	out := &CloudFrontInstanceSpec{}
	out.CallerReference = &in.CallerReference

	fmt.Printf("out: %+#v", out)
	go s.createCloudFrontDistribution(ctx, in, out)

	return nil
}

func (s *AwsConfigSpec) createCloudFrontDistribution(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) {
	var err error
	in.distChan = make(chan error)
	defer close(in.distChan)

	// TODO create task in db

	go s.createS3Bucket(ctx, in, out)
	err = <-in.distChan

	if err != nil {
		klog.Errorf("error creating bucket: %s\n", err.Error())
		return
	}

	err = s.createIAMUser(ctx, in, out)

	if err != nil {
		msg := fmt.Sprintf("error creating iam user: %s", err.Error())
		klog.Error(msg)
		// TODO write error to tasks
		return
	}

	// originAccessIdentity
	go s.createOriginAccessIdentity(ctx, in, out)
	err = <-in.distChan

	if err != nil {
		msg := fmt.Sprintf("error creating OAI: %s\n", err.Error())
		klog.Error(msg)
		// TODO update task in db
		return
	}

	go s.createDistribution(ctx, in, out)
	err = <-in.distChan

	if err != nil {
		msg := fmt.Sprintf("error creating distribution: %s", err.Error())
		klog.Error(msg)
		// TODO write error to tasks
		return
	}

	s.addBucketPolicy(ctx, in, out)
	err = <-in.distChan

	if err != nil {
		msg := fmt.Sprintf("error adding bucket policy: %s", err.Error())
		klog.Error(msg)
		in.distChan <- errors.New(msg)
		return
	}

	err = s.createIAMUser(ctx, in, out)
}

func (s *AwsConfigSpec) createOriginAccessIdentity(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) {
	var err error

	svc := cloudfront.New(s.sess)
	klog.Infof("cf sess: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprint("error creating new cloudfront session")
		klog.Error(msg)
		in.distChan <- errors.New(msg)
		return
	}

	originAccessIdentity, err := svc.CreateCloudFrontOriginAccessIdentityWithContext(ctx, &cloudfront.CreateCloudFrontOriginAccessIdentityInput{
		CloudFrontOriginAccessIdentityConfig: &cloudfront.OriginAccessIdentityConfig{
			CallerReference: &in.CallerReference,
			Comment:         utils.StrPtr(in.BucketName),
		},
	})

	if err != nil {
		msg := fmt.Sprintf("error creating OriginAccessIdenity: %s", err.Error())
		klog.Error(msg)
		in.distChan <- errors.New(msg)
		return
	}

	out.OriginAccessIdentity = originAccessIdentity.CloudFrontOriginAccessIdentity.Id

	klog.Infof("oai id: %s\n", *out.OriginAccessIdentity)

	in.distChan <- nil
}

func (s *AwsConfigSpec) createDistribution(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) {
	var err error
	var cfout *cloudfront.CreateDistributionWithTagsOutput

	svc := cloudfront.New(s.sess)
	klog.Infof("svc: %#+v\n", svc)
	if svc == nil {
		msg := fmt.Sprintf("error getting cloudfront session: %s", err.Error())
		klog.Error(msg)
		in.distChan <- errors.New(msg)
		return
	}

	fmt.Print("attach OAI")
	var s3Origin = []*cloudfront.Origin{}

	s3Origin = append(s3Origin, &cloudfront.Origin{
		DomainName: out.S3Bucket.Fullname,
		Id:         out.S3Bucket.ID,
		S3OriginConfig: &cloudfront.S3OriginConfig{
			OriginAccessIdentity: utils.StrPtr("origin-access-identity/cloudfront/" + *out.OriginAccessIdentity),
		},
	})

	err = s3Origin[0].Validate()
	if err != nil {
		msg := fmt.Sprintf("error in S3Origin: %s", err.Error())
		klog.Errorf(msg)
		in.distChan <- err
		// TODO write update task with error
	}

	cmi := []*string{}
	cmi = append(cmi, utils.StrPtr("GET"))
	cmi = append(cmi, utils.StrPtr("HEAD"))

	tags := []*cloudfront.Tag{}
	tags = append(tags, &cloudfront.Tag{
		Key:   utils.StrPtr("billingcode"),
		Value: utils.StrPtr(in.BillingCode),
	})

	cin := &cloudfront.CreateDistributionWithTagsInput{
		DistributionConfigWithTags: &cloudfront.DistributionConfigWithTags{
			DistributionConfig: &cloudfront.DistributionConfig{
				CallerReference: &in.CallerReference,
				Origins: &cloudfront.Origins{
					Items:    s3Origin,
					Quantity: utils.OnePtr,
				},
				Comment: out.S3Bucket.Name,
				DefaultCacheBehavior: &cloudfront.DefaultCacheBehavior{
					AllowedMethods: &cloudfront.AllowedMethods{
						CachedMethods: &cloudfront.CachedMethods{
							Items:    cmi,
							Quantity: utils.TwoPtr,
						},
						Items:    cmi,
						Quantity: utils.TwoPtr,
					},
					DefaultTTL: utils.TtlPtr,
					MinTTL:     utils.TtlPtr,
					MaxTTL:     utils.TtlPtr,
					ForwardedValues: &cloudfront.ForwardedValues{
						Cookies: &cloudfront.CookiePreference{
							Forward: utils.StrPtr("none"),
						},
						QueryString: utils.FalsePtr(),
					},
					TargetOriginId: s3Origin[0].Id,
					TrustedSigners: &cloudfront.TrustedSigners{
						Enabled:  utils.FalsePtr(),
						Quantity: utils.Int64Ptr(0),
					},
					ViewerProtocolPolicy: utils.StrPtr("redirect-to-https"),
				},
				Enabled: utils.FalsePtr(),
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
		in.distChan <- errors.New(msg)
	} else {
		cfout, err = svc.CreateDistributionWithTagsWithContext(ctx, cin)
		if err != nil {
			msg := fmt.Sprintf("error creating distribution: %s", err.Error())
			klog.Error(msg)
			in.distChan <- errors.New(msg)
			return
		} else {
			out.DistributionID = cfout.Distribution.Id
			out.DistributionURL = cfout.Location

			gdin := &cloudfront.GetDistributionInput{
				Id: cfout.Distribution.Id,
			}

			err = svc.WaitUntilDistributionDeployedWithContext(ctx, gdin)
			if err != nil {
				msg := fmt.Sprintf("error creating distribution: %s", err.Error())
				klog.Error(msg)
				in.distChan <- errors.New(msg)
				return
			}
		}
	}
	in.distChan <- nil
}
