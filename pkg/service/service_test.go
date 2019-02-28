package service

import (
	"os"
	"testing"

	"github.com/nu7hatch/gouuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCloudFrontService(t *testing.T) {
	Convey("AWS Cloudfront Services", t, func() {
		var c *AwsConfigSpec
		var err error
		var in *cloudFrontInstanceSpec
		devTesting := "devTesting"
		dist := "dist"
		nUuid, _ := uuid.NewV4()
		sUuid := nUuid.String()

		in = &cloudFrontInstanceSpec{
			billingCode:     &devTesting,
			callerReference: &sUuid,
			planId:          &dist,
			distChan:        make(chan error),
		}

		c, err = Init(os.Getenv("NAME_PREFIX"), 10, 150)

		So(c, ShouldNotBeNil)
		So(err, ShouldBeNil)

		Printf("\nnamePrefix: %s\n", c.namePrefix)
		Printf("sess: %s\n", *c.sess.Config.Region)
		Printf("conf: %s\n", *c.conf.Region)

		Convey("create s3 bucket", func() {
			go c.createS3Bucket(in)
			err = <-in.distChan

			So(err, ShouldBeNil)
			So(in.s3Bucket.name, ShouldNotBeNil)
			So(in.s3Bucket, ShouldNotBeNil)
			So(*in.s3Bucket.name, ShouldNotBeBlank)

			Printf("\ns3 name: %s\n", *in.s3Bucket.name)
			Printf("s3 uri: %s\n", *in.s3Bucket.uri)
			Printf("s3 id: %s\n", *in.s3Bucket.id)

			Convey("create iam user", func() {
				err = c.createIAMUser(in)

				So(err, ShouldBeNil)
				So(in.iAMUser, ShouldNotBeNil)
				So(*in.iAMUser.userName, ShouldNotBeBlank)
				So(*in.iAMUser.policyName, ShouldNotBeBlank)

				Printf("iam user: %s\n", *in.iAMUser.userName)
				Printf("iam access key: %s\n", *in.iAMUser.accessKey)
				Printf("iam secret key: %s\n", *in.iAMUser.secretKey)
				Printf("policy name: %s\n", *in.iAMUser.policyName)

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
						So(in.distributionId, ShouldNotBeNil)
						So(*in.distributionId, ShouldNotBeBlank)

						Printf("\ndistribution id: %s\n", *in.distributionId)

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
