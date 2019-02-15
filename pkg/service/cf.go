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
  out.CallerReference = in.CallerReference

  go s.createCloudFrontDistribution(ctx, in, out)

  return nil
}

func (s *AwsConfigSpec) createCloudFrontDistribution(ctx context.Context, in *InCreateDistributionSpec, out *CloudFrontInstanceSpec) {
  var err error
  in.distChan = make(chan error)
  defer close(in.distChan)

  go s.createS3Bucket(ctx, in, out)
  err = <-in.distChan

  if err != nil {
    klog.Errorf("error creating bucket: %s\n", err)
  } else {
    // originAccessIdentity
    go s.createOriginAccessIdentity(ctx, in, out)
    err = <-in.distChan

    if err != nil {
      klog.Errorf("error creating OAI: %s\n", err)
    } else {
      // TODO attach oai
      fmt.Print("attach OAI")
      // TODO CreateDistributionWithTags
    }
  }
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

  out.OriginAccessIdentity = *originAccessIdentity.CloudFrontOriginAccessIdentity.Id

  klog.Infof("oai id: %s\n", out.OriginAccessIdentity)

  /*
  input := cloudfront.GetCloudFrontOriginAccessIdentityConfigInput{
    Id: &out.OriginAccessIdentity,
  }

  for i := 10; i >= 0; i-- {
    msg := fmt.Sprintf("oai id %s for %d\n", *input.Id, i)
    fmt.Print(msg)
    klog.Info(msg)
    _, aerr := svc.GetCloudFrontOriginAccessIdentityConfigWithContext(ctx, &input)
    if aerr == nil {
      fmt.Printf("oai is ready: %s\n", *input.Id)
      klog.Infof("oai is ready: %s", *input.Id)
      in.distChan <- nil
      return
    } else {
      if awsErr, ok := aerr.(awserr.Error); ok {
        err = errors.New(awsErr.Error())
        msg := fmt.Sprintf("oai is not ready: %s", err.Error())
        fmt.Println(msg)
        if err.Error() == "InvalidURI" {
          in.distChan <- errors.New(msg)
          return
        }
      } else {
        fmt.Println(err)
      }
    }
    fmt.Println("sleeping")
    err = aws.SleepWithContext(ctx, time.Second * s.taskSleep)
    select {
    case <- ctx.Done():
      err = ctx.Err()
      break
    default:
      if err != nil {
        break
      }
    }
  }

  if err != nil {
    fmt.Printf("returning with error: %s\n", err.Error())
    in.distChan <- err
  } else {
    in.distChan <- nil
  }
  */

  in.distChan <- nil
}
