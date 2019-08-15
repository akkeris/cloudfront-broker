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
	dbURL := os.Getenv("DATABASE_URL")

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
			os.Setenv("DATABASE_URL", dbURL)
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
	status := "new"
	bucketURL := fmt.Sprintf("https://%s.s3.aws.io", bucketName)
	originPath := "/"
	cloudfrontID := "EA1B2C3D4E5"
	cloudfrontURL := "https://d123456abcd.cloudfront.net"
	iamUser := "AD23F3443FGW34"
	accessKey := "ALKASJF234234H5H32K234"
	secretKey := "ajdskf2sksdahffds2jhkjhk56hk"
	originAccessIdentity := "EASDF23SLKJSFKJ24JLK"

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
	})

	Convey("distributions", t, func() {
		Convey("new distribution", func() {
			err := stg.NewDistribution(distributionID, planID, &billingCode, callerReference, status)
			So(err, ShouldBeNil)

			Convey("get distribution", func() {
				_, err = stg.GetDistribution(distributionID)

				So(err, ShouldBeNil)
				Convey("update distribution status", func() {
					var pendingStatus = "pending"
					err = stg.UpdateDistributionStatus(distributionID, pendingStatus, false)

					So(err, ShouldBeNil)

					Convey("update with cloudfront", func() {
						dist, err := stg.UpdateDistributionCloudfront(distributionID, cloudfrontID, cloudfrontURL)

						So(err, ShouldBeNil)
						So(dist, ShouldNotBeNil)
						So(dist.CloudfrontID.String, ShouldEqual, cloudfrontID)
						So(dist.CloudfrontURL.String, ShouldEqual, cloudfrontURL)

						Convey("update with origin access identity", func() {
							err := stg.UpdateDistributionWIthOriginAccessIdentity(distributionID, originAccessIdentity)

							So(err, ShouldBeNil)
						})
					})
				})
			})
		})
	})

	Convey("origins", t, func() {
		Convey("insert new origin", func() {
			origin, err := stg.AddOrigin(distributionID, bucketName, bucketURL, originPath)

			So(err, ShouldBeNil)
			So(origin.OriginID, ShouldNotBeBlank)
			So(origin.DistributionID, ShouldEqual, distributionID)

			originID = origin.OriginID

			Convey("get origin from distribution", func() {
				origin, err := stg.GetOriginByDistributionID(distributionID)

				So(err, ShouldBeNil)
				So(origin, ShouldNotBeNil)
				So(origin.OriginID, ShouldEqual, originID)

				Convey("get origin by id", func() {
					origin, err := stg.GetOriginByID(originID)

					So(err, ShouldBeNil)
					So(origin, ShouldNotBeNil)
					So(origin.DistributionID, ShouldEqual, distributionID)

					Convey("add iam user", func() {
						err := stg.AddIAMUser(originID, iamUser)

						So(err, ShouldBeNil)

						Convey("add access key", func() {
							err := stg.AddAccessKey(originID, accessKey, secretKey)

							So(err, ShouldBeNil)
						})
					})
				})
			})
		})
	})

	Convey("tasks", t, func() {
		Convey("insert new task", func() {
			task := &Task{
				DistributionID: distributionID,
				Action:         "create-new",
				Status:         "new",
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

			// Printf("\ntask id: %s\n", task.TaskID)

			Convey("pop next task", func() {
				popTask, err := stg.PopNextTask()

				So(err, ShouldBeNil)
				So(popTask.TaskID, ShouldEqual, taskID)
			})

			Convey("update task action", func() {
				var newAction = "is-distribution-deployed"
				task := &Task{
					TaskID:         taskID,
					DistributionID: distributionID,
					Action:         newAction,
					Status:         "pending",
					Retries:        0,
					OperationKey:   sql.NullString{String: operationKey, Valid: true},
					Result:         sql.NullString{String: "in progress", Valid: true},
					Metadata:       sql.NullString{String: "", Valid: false},
					StartedAt:      pq.NullTime{Time: time.Now(), Valid: true},
				}

				updatedTask, err := stg.UpdateTaskAction(task)

				So(err, ShouldBeNil)
				So(updatedTask.TaskID, ShouldEqual, taskID)
				So(updatedTask.Action, ShouldEqual, newAction)
			})

			Convey("get task by distribution", func() {
				task, err := stg.GetTaskByDistribution(distributionID)

				So(err, ShouldBeNil)
				So(task.TaskID, ShouldEqual, taskID)
			})
			Reset(func() {
				err = stg.deleteItTask(taskID)
			})
		})
	})

	Convey("'delete' distribution", t, func() {
		Convey("update distribution as deleted", func() {
			err := stg.UpdateDeleteDistribution(distributionID)

			So(err, ShouldBeNil)

			Convey("get deleted distribution", func() {
				dist, err := stg.GetDistributionWithDeleted(distributionID)

				So(err, ShouldBeNil)
				So(dist.DistributionID, ShouldEqual, distributionID)
				So(dist.DeletedAt.Valid, ShouldBeTrue)
			})
		})
	})

	err = stg.deleteItOrigin(originID)
	err = stg.deleteItDistribution(distributionID)
}
