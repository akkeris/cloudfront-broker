// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package storage

import (
	"context"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDBInit(t *testing.T) {
	dbUrl := os.Getenv("DATABASE_URL")

	Convey("Given no database url", t, func() {
		os.Setenv("DATABASE_URL", "")
		pgs, err := InitStorage(context.TODO(), "")
		So(err, ShouldNotBeNil)
		So(pgs, ShouldBeNil)
	})

	os.Setenv("DATABASE_URL", dbUrl)
	Convey("Given valid db connection params", t, func() {
		pgStorage, err := InitStorage(context.TODO(), "")
		So(err, ShouldBeNil)
		So(pgStorage, ShouldNotBeNil)
		pgStorage.db.Close()
	})
}

func TestStorage(t *testing.T) {
	p, err := InitStorage(context.TODO(), "")
	if err != nil {
		Printf("error init db: %s\n", err)
		return
	}

	Convey("Get the services and plan from DB verify target plan exists", t, func() {
			services, err := p.GetServices()
			serviceName := "distribution"
			So(err, ShouldBeNil)
			So(services, ShouldNotBeNil)
			So(services[0].Name, ShouldEqual, serviceName)
			tPlan := "dist"
			So(services[0].Plans, ShouldNotBeEmpty)
			plan := services[0].Plans[0]
			So(plan.Name, ShouldEqual, tPlan)
		})

		Convey("Get plan by id", t, func() {
		  planName := "dist"
			plan, err := p.GetPlanByID("5eac120c-5303-4f55-8a62-46cde1b52d0b")
			So(err, ShouldBeNil)
			So(plan, ShouldNotBeNil)
			So(plan.basePlan.Name, ShouldEqual, planName)
		})
		Convey("Insert provisioned distribution", t, nil)
}
