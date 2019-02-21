package service

import (
	"context"
	"os"
	"testing"

	"github.com/nu7hatch/gouuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCloudFrontService(t *testing.T) {
	Convey("AWS Cloudfront Services", t, func() {
		var c *AwsConfigSpec
		var err error
		newUuid, _ := uuid.NewV4()
		in := &InCreateDistributionSpec{
			BillingCode:     "devTesting",
			CallerReference: newUuid.String(),
			Plan:            "dist",
			distChan:        make(chan error),
		}
		out := &CloudFrontInstanceSpec{}
		ctx, _ := context.WithCancel(context.Background())

		Convey("initialize", func() {
			c, err = Init(os.Getenv("NAME_PREFIX"))

			So(c, ShouldNotBeNil)
			So(err, ShouldBeNil)

			Printf("\nnamePrefix: %s\n", c.namePrefix)
			Printf("sess: %s\n", *c.sess.Config.Region)
			Printf("conf: %s\n", *c.conf.Region)

			Convey("create s3 bucket", func() {
				go c.createS3Bucket(ctx, in, out)
				err = <-in.distChan

				So(err, ShouldBeNil)
				So(in.BucketName, ShouldNotBeNil)
				So(out.S3Bucket, ShouldNotBeNil)

				Printf("\ns3 name: %s\n", *out.S3Bucket.Name)
				Printf("s3 uri: %s\n", *out.S3Bucket.Uri)
				Printf("s3 id: %s\n", *out.S3Bucket.ID)

				Convey("create iam user", func() {
					err = c.createIAMUser(ctx, in, out)

					So(err, ShouldBeNil)
					So(out.IAMUser, ShouldNotBeNil)
					So(*out.IAMUser, ShouldNotBeBlank)
					So(out.PolicyName, ShouldNotBeNil)
					So(*out.PolicyName, ShouldNotBeBlank)

					Printf("iam user: %s\n", *out.IAMUser)
					Printf("iam access key: %s\n", *out.AccessKey)
					Printf("iam secret key: %s\n", *out.SecretKey)
					Printf("iam policy name: %s\n", *out.PolicyName)
				})

				go c.createOriginAccessIdentity(ctx, in, out)
				err = <-in.distChan

				So(err, ShouldBeNil)
				So(out.OriginAccessIdentity, ShouldNotBeNil)
				So(*out.OriginAccessIdentity, ShouldNotBeBlank)

				Printf("\noai: %s\n", *out.OriginAccessIdentity)

				Convey("create origin access idenity", func() {
					go c.createOriginAccessIdentity(ctx, in, out)
					err = <-in.distChan

					So(err, ShouldBeNil)
					So(out.OriginAccessIdentity, ShouldNotBeNil)
					So(*out.OriginAccessIdentity, ShouldNotBeBlank)
				})

				Convey("delete bucket", func() {
					err = c.DeleteS3Bucket(context.TODO(), out.S3Bucket)
					So(err, ShouldBeNil)
				})
			})
		})
	})
}
