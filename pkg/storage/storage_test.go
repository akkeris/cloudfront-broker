// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package storage

import (
	"context"
	"fmt"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDBInit(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")

	Convey("Test initializing storage", t, func() {
		Convey("without DATABASE_URL", func() {
			os.Setenv("DATABASE_URL", "")
			storage, err := InitStorage(context.TODO(), "")
			So(err, ShouldNotBeNil)
			So(storage, ShouldBeNil)
		})

		Convey("Given valid db connection params", func() {
			storage, err := InitStorage(context.TODO(), "")
			So(err, ShouldBeNil)
			So(storage, ShouldNotBeNil)
			storage.db.Close()
		})

		Reset(func() {
			os.Setenv("DATABASE_URL", dbUrl)
		})
	})
}

func TestStorage(t *testing.T) {
	billingCode := "cfdev"
	serviceID := "3b8d2e75-ca9f-463f-84e4-4b85513f1bc8"
	planID := "5eac120c-5303-4f55-8a62-46cde1b52d0b"
	instanceID := "61c9932c-52fc-4168-8a4e-86b48375aac4"
	bucketName := "cfdev-a1b2c3d4"
	bucketURL := fmt.Sprintf("https://%s.s3.aws.io", bucketName)
	iAMUser := "AD23F3443FGW34"
	accessKey := "ALKASJF234234H5H32K234"
	secretKey := "ajdskf2sksdahffds2jhkjhk56hk"
	originAccessIdentity := "EASDF23SLKJSFKJ24JLK"
	// cloudfrontId := "EA1B2C3D4E5"

	p, err := InitStorage(context.TODO(), "")
	if err != nil {
		fmt.Printf("error init db: %s\n", err)
		return
	}

	Convey("With database initialized", t, func() {

		Convey("get service catalog", func() {
			services, err := p.GetServicesCatalog()
			So(err, ShouldBeNil)
			So(services, ShouldNotBeNil)
			So(services[0].ID, ShouldEqual, serviceID)
			So(services[0].Plans, ShouldNotBeEmpty)
			So(services[0].Plans[0].ID, ShouldEqual, planID)

			Convey("create new distribution", func() {
				distribution, err := p.NewDistribution(instanceID, planID, billingCode)

				So(err, ShouldBeNil)
				So(distribution.DistributionID, ShouldNotBeBlank)
				So(distribution.DistributionID, ShouldEqual, instanceID)

				Convey("when creating origin", func() {
					origin, err := p.NewOrigin(distribution.DistributionID, bucketName, bucketURL, "/", billingCode)

					So(err, ShouldBeNil)
					So(origin.OriginID, ShouldNotBeBlank)
					So(origin.BillingCode, ShouldEqual, billingCode)

					Convey("create iam user for s3 bucket", func() {
						err := p.AddIAMUser(origin.OriginID, iAMUser, accessKey, secretKey)

						So(err, ShouldBeNil)

						Convey("add origin access identity", func() {
							err := p.AddOriginAccessIdentity(distribution.DistributionID, originAccessIdentity)

							So(err, ShouldBeNil)
						})
					})
				})
			})
		})
	})
}
