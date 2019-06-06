// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
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
	distributionID := "61c9932c-52fc-4168-8a4e-86b48375aac4"
	originID := "9ea4b23a-3641-46f8-b424-a14fa12ae22d"
	taskID := "726c0b65-bc07-4c4c-bebc-4d69f9c02007"
	bucketName := "cfdev-a1b2c3d4"
	operationKey := "PRV123456789"
	callerReference := "fe06e76e-9823-4e59-9feb-4a95d4f6eddc"
	status := "pending"
	bucketURL := fmt.Sprintf("https://%s.s3.aws.io", bucketName)
	originPath := "/"
	// iAMUser := "AD23F3443FGW34"
	// accessKey := "ALKASJF234234H5H32K234"
	// secretKey := "ajdskf2sksdahffds2jhkjhk56hk"
	// originAccessIdentity := "EASDF23SLKJSFKJ24JLK"
	// cloudfrontId := "EA1B2C3D4E5"

	stg, err := InitStorage(context.TODO(), "")
	if err != nil {
		Printf("error init db: %s\n", err)
		return
	}

	Convey("With database initialized", t, func() {

		Convey("null strings", func() {
			nNullStr := SetNullString("")
			So(nNullStr.Valid, ShouldBeFalse)

			nVal := nullStringValue(nNullStr)
			So(nVal, ShouldBeBlank)

			tNullStr := SetNullString(status)
			So(tNullStr.Valid, ShouldBeTrue)
			So(tNullStr.String, ShouldEqual, status)

			tVal := nullStringValue(tNullStr)
			So(tVal, ShouldNotBeBlank)
			So(tVal, ShouldEqual, status)

			tValPtr := stg.NullString(tNullStr)
			So(tValPtr, ShouldNotBeNil)
			So(*tValPtr, ShouldNotBeBlank)
			So(*tValPtr, ShouldEqual, status)
		})

		Convey("null time", func() {
			var nowP *time.Time
			now := time.Now()

			nNullTime := SetNullTime(nowP)
			So(nNullTime.Valid, ShouldBeFalse)

			tNullTime := SetNullTime(&now)
			So(tNullTime.Valid, ShouldBeTrue)
			So(tNullTime.Time, ShouldEqual, now)
		})

		Convey("get service catalog", func() {
			services, err := stg.GetServicesCatalog()
			So(err, ShouldBeNil)
			So(services, ShouldNotBeNil)
			So(services[0].ID, ShouldEqual, serviceID)
			So(services[0].Plans, ShouldNotBeEmpty)
			So(services[0].Plans[0].ID, ShouldEqual, planID)
		})

		Convey("new distribution", func() {
			err := stg.NewDistribution(distributionID, planID, billingCode, callerReference, status)
			So(err, ShouldBeNil)

			Convey("insert new task", func() {

				task := &Task{
					DistributionID: distributionID,
					Action:         "create-new",
					Status:         "pending",
					Retries:        0,
					OperationKey:   sql.NullString{String: operationKey, Valid: true},
					Result:         sql.NullString{String: "in progress", Valid: true},
					Metadata:       sql.NullString{String: "", Valid: false},
					StartedAt:      pq.NullTime{Time: time.Now(), Valid: true},
				}

				task, err = stg.AddTask(task)

				So(err, ShouldBeNil)
				So(task.TaskID, ShouldNotBeBlank)

				taskID = task.TaskID

				Printf("\ntask id: %s\n", task.TaskID)

				Convey("insert new origin", func() {
					origin, err := stg.AddOrigin(distributionID, bucketName, bucketURL, originPath, billingCode)

					So(err, ShouldBeNil)
					So(origin.OriginID, ShouldNotBeBlank)
					So(origin.DistributionID, ShouldEqual, distributionID)

					originID = origin.OriginID
					Printf("\norigin id: %s\n", originID)
				})
			})
		})

		/*
			Reset(func() {
				err = stg.UpdateDeleteDistribution(distributionID)

				if err != nil {
					Print("error 'deleting' distribution")
				}
				_, err := stg.UpdateDeleteOrigin(distributionID, originID)

				if err != nil {
					Print("error 'deleting' origin")
				}
			})
		*/
	})

	err = stg.deleteItOrigin(originID)
	err = stg.deleteItTask(taskID)
	err = stg.deleteItDistribution(distributionID)
}
