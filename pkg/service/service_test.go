package service

import (
	"os"
	"testing"

	"cloudfront-broker/pkg/storage"

	"github.com/nu7hatch/gouuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCloudFrontService(t *testing.T) {
	Convey("AWS Cloudfront Services", t, func() {
		var c *AwsConfig
		var err error
		var in *cloudFrontInstance
		devTesting := "devTesting"
		dist := "dist"
		nUuid, _ := uuid.NewV4()
		sUuid := nUuid.String()

		in = &cloudFrontInstance{
			billingCode:     &devTesting,
			callerReference: &sUuid,
			planID:          &dist,
			distChan:        make(chan error),
		}

		c, err = Init(&storage.PostgresStorage{}, os.Getenv("NAME_PREFIX"), 10)

		So(c, ShouldNotBeNil)
		So(err, ShouldBeNil)

		Printf("\nnamePrefix: %s\n", c.namePrefix)
		Printf("sess: %s\n", *c.sess.Config.Region)
		Printf("conf: %s\n", *c.conf.Region)

		Convey("create s3 bucket", func() {
			go c.createS3Bucket(in)
			err = <-in.distChan

			So(err, ShouldBeNil)
			So(in.s3Bucket.bucketName, ShouldNotBeNil)
			So(in.s3Bucket, ShouldNotBeNil)
			So(*in.s3Bucket.bucketName, ShouldNotBeBlank)

			Printf("\ns3 name: %s\n", *in.s3Bucket.bucketName)
			Printf("s3 uri: %s\n", *in.s3Bucket.bucketURI)
			Printf("s3 id: %s\n", *in.s3Bucket.originID)

			Convey("create iam user", func() {
				err = c.createIAMUser(in)

				So(err, ShouldBeNil)
				So(in.s3Bucket.iAMUser, ShouldNotBeNil)
				So(*in.s3Bucket.iAMUser.userName, ShouldNotBeBlank)
				So(*in.s3Bucket.iAMUser.policyName, ShouldNotBeBlank)

				Printf("iam user: %s\n", *in.s3Bucket.iAMUser.userName)
				Printf("iam access key: %s\n", *in.s3Bucket.iAMUser.accessKey)
				Printf("iam secret key: %s\n", *in.s3Bucket.iAMUser.secretKey)
				Printf("policy name: %s\n", *in.s3Bucket.iAMUser.policyName)

				Convey("create origin access idenity", func() {
					go c.createOriginAccessIdentity(in)
					err = <-in.distChan

					So(err, ShouldBeNil)
					So(in.originAccessIdentity, ShouldNotBeNil)
					So(*in.originAccessIdentity, ShouldNotBeBlank)

					Printf("\noai: %s\n", *in.originAccessIdentity)

					Convey("create cloudfront distribution", func() {
						go c.createDistribution(in)
						err = <-in.distChan

						So(err, ShouldBeNil)
						So(in.cloudfrontID, ShouldNotBeNil)
						So(*in.cloudfrontID, ShouldNotBeBlank)

						Printf("\ndistribution id: %s\n", *in.cloudfrontID)

						Convey("add bucket policy", func() {
							err = c.addBucketPolicy(in)
							// err = <-in.distChan

							So(err, ShouldBeNil)

							Println("bucket policy added?")

							Convey("delete bucket", func() {
								err = c.deleteS3Bucket(in)

								So(err, ShouldBeNil)

								Println("bucket deleted?")

								Convey("delete iam user", func() {

									err = c.deleteIAMUser(in)

									So(err, ShouldBeNil)

									Println("iam user deleted")

									Convey("disable distribution", func() {
										err = c.disableDistribution(in)

										So(err, ShouldBeNil)

										Println("distribution disabled")

										Convey("delete distribution", func() {
											go c.deleteDistribution(in)
											err = <-in.distChan

											So(err, ShouldBeNil)

											Println("distribution Deleted")

											Convey("delete origin access id", func() {
												err = c.deleteOriginAccessIdentity(in)

												So(err, ShouldBeNil)

												Println("origin access id deleted")
											})
										})
									})
								})
							})
						})
					})
				})
			})
		})
	})
}
