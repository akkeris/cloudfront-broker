// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/golang/glog"

	_ "github.com/lib/pq"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

/*
type Storage interface {
  GetServicesCatalog() ([]osb.Service, error)
  NewDistribution(string, string, string) (*Distribution, error)
  NewOrigin(string, string, string, string, string) (*Origin, error)
}
*/

type PostgresStorage struct {
	// Storage
	db *sql.DB
}

func cancelOnInterrupt(ctx context.Context, db *sql.DB) {
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-term:
			db.Close()
		case <-ctx.Done():
			db.Close()
		}
	}
}

func InitStorage(ctx context.Context, DatabaseUrl string) (*PostgresStorage, error) {
	var err error
	err = nil
	// Sanity checks
	if DatabaseUrl == "" && os.Getenv("DATABASE_URL") != "" {
		DatabaseUrl = os.Getenv("DATABASE_URL")
	}

	if DatabaseUrl == "" {
		glog.Error("Unable to connect to database, none was specified in the environment via DATABASE_URL or through the -database cli option.")
		return nil, errors.New("unable to connect to database, none was specified in the environment via DATABASE_URL or through the -database cli option")
	}

	glog.Infof("DATABASE_URL=%s", DatabaseUrl)

	db, err := sql.Open("postgres", DatabaseUrl)
	if err != nil {
		glog.Errorf("Unable to open database: %s\n", err.Error())
		return nil, errors.New("Unable to open database: " + err.Error())
	}

	pgStorage := PostgresStorage{
		db: db,
	}

	go cancelOnInterrupt(ctx, db)

	_, err = db.Exec(createScript)
	if err != nil {
		glog.Errorf("error creating database tables: %s\n", err)
		return nil, err
	}

	var cnt int
	err = db.QueryRow("select count(*) from services;").Scan(&cnt)

	if err != nil || cnt == 0 {
		_, err = db.Exec(initServicesScript)
		if err != nil {
			glog.Errorf("error initializing services: %s\n", err)
			return nil, err
		}
	}

	err = db.QueryRow("select count(*) from plans;").Scan(&cnt)
	if err != nil || cnt == 0 {
		_, err = db.Exec(initPlansScript)
		if err != nil {
			glog.Errorf("error initializing plans: %s\n", err)
			return nil, err
		}
	}

	return &pgStorage, nil
}

func getPlans(db *sql.DB, serviceId string) ([]osb.Plan, error) {
	rows, err := db.Query(plansQuery+"and services.service_id = $1 order by plans.name", serviceId)
	if err != nil {
		glog.Errorf("getPlans query failed: %s\n", err.Error())
		return nil, err
	}
	defer rows.Close()

	plans := make([]osb.Plan, 0)

	for rows.Next() {
		var id, name, serviceName, humanName, description, categories, costUnit string
		var cents int32
		var free, beta, depreciated bool

		err := rows.Scan(&id, &name, &serviceName, &humanName, &description, &categories, &free, &cents, &costUnit, &beta, &depreciated)
		if err != nil {
			glog.Errorf("Scan from plans query failed: %s\n", err.Error())
			return nil, errors.New("Scan from plans query failed: " + err.Error())
		}

		plans = append(plans, osb.Plan{
			ID:          id,
			Name:        name,
			Description: description,
			Free:        &free,
			Metadata: map[string]interface{}{
				"humanName":   humanName,
				"cents":       cents,
				"cost_unit":   costUnit,
				"beta":        beta,
				"depreciated": depreciated,
			},
		})
	}

	return plans, nil
}

func (p *PostgresStorage) GetServicesCatalog() ([]osb.Service, error) {
	var planUpdateable bool = true

	services := make([]osb.Service, 0)

	rows, err := p.db.Query(servicesQuery)
	if err != nil {
		return nil, errors.New("Unable to get services: " + err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var service_id, service_name, service_human_name, service_description, service_categories, service_image string
		var beta, deprecated bool

		err = rows.Scan(&service_id, &service_name, &service_human_name, &service_description, &service_categories, &service_image, &beta, &deprecated)
		if err != nil {
			glog.Errorf("Unable to get services: %s\n", err.Error())
			return nil, errors.New("Unable to scan services: " + err.Error())
		}

		plans, err := getPlans(p.db, service_id)
		if err != nil {
			glog.Errorf("Unable to get plans for %s: %s\n", service_name, err.Error())
			return nil, errors.New("Unable to get plans for " + service_name + ": " + err.Error())
		}

		osbPlans := make([]osb.Plan, 0)
		for _, plan := range plans {
			osbPlans = append(osbPlans, plan)
		}

		services = append(services, osb.Service{
			Name:                service_name,
			ID:                  service_id,
			Description:         service_description,
			Bindable:            true,
			BindingsRetrievable: true,
			PlanUpdatable:       &planUpdateable,
			Tags:                strings.Split(service_categories, ","),
			Metadata: map[string]interface{}{
				"name":  service_human_name,
				"image": service_image,
			},
			Plans: osbPlans,
		})
	}
	return services, nil
}

func (p *PostgresStorage) NewDistribution(distributionID string, planID string, billingCode string) (*Distribution, error) {
	var err error
	var cnt int

	var checkPlanScript = `
    select count(*) from plans
    where plan_id = $1
    and deleted_at is null
  `

	err = p.db.QueryRow(checkPlanScript, planID).Scan(&cnt)

	if err != nil && err.Error() == "sql: no rows in result set" {
		msg := fmt.Sprintf("NewDistribution: can not find plan: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	} else if err != nil {
		msg := fmt.Sprintf("NewDistribution: error finding plan: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	distribution := &Distribution{
		DistributionID: distributionID,
		PlanID:         planID,
	}

	insertDistScript := `insert into distributions
    (distribution_id, plan_id, billing_code) 
    values 
    ($1, $2, $3) returning distribution_id;`

	err = p.db.QueryRow(insertDistScript, distributionID, planID, billingCode).Scan(&distribution.DistributionID)
	if err != nil {
		msg := fmt.Sprintf("NewDistribution: error inserting distribution: %s", err.Error())
		glog.Error(msg)
		return nil, err
	}

	glog.Infof("NewDistribution: distribution id: %s", distribution.DistributionID)

	return distribution, nil
}

var checkDistScript = `
    select distribution_id from distributions
    where distribution_id = $1
    and deleted_at is null
  `

func (p *PostgresStorage) NewOrigin(distributionID string, bucketName string, originURL string, originPath string, billingCode string) (*Origin, error) {
	var err error

	distribution := &Distribution{}

	err = p.db.QueryRow(checkDistScript, distributionID).Scan(&distribution.DistributionID)

	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("NewOrigin: origin not found: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	case err != nil:
		msg := fmt.Sprintf("NewOrigin: error finding origin: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	origin := &Origin{
		DistributionID: distributionID,
		BucketName:     bucketName,
		OriginUrl:      originURL,
		OriginPath:     "/",
		BillingCode:    billingCode,
	}

	if err = p.db.QueryRow(`insert into origins
    (origin_id, distribution_id, bucket, bucket_url)
    values 
    (uuid_generate_v4(), $1, $2, $3) returning origin_id;`,
		distributionID, bucketName, originURL).Scan(&origin.OriginID); err != nil {
		msg := fmt.Sprintf("CreateOrigin: error inserting origin: %s", err.Error())
		glog.Error(msg)
		return nil, err
	}

	glog.Infof("CreateOrigin: originId: %s", origin.OriginID)

	return origin, nil
}

func (p *PostgresStorage) AddIAMUser(originID string, iAMUser string, accessKey string, secretKey string) error {
	var err error
	var getOriginScript = `
    select origin_id from origins
    where origin_id = $1
    and deleted_at is null
  `

	origin := &Origin{}

	err = p.db.QueryRow(getOriginScript, originID).Scan(&origin.OriginID)

	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("AddIAMUser: origin not found: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	case err != nil:
		msg := fmt.Sprintf("AddIAMUser: error finding origin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	updateScript := `
    update origins
      set iam_user = $1,
        access_key = $2,
        secret_key = $3
      where origin_id = $4
`
	_, err = p.db.Exec(updateScript, iAMUser, accessKey, secretKey, originID)

	if err != nil {
		msg := fmt.Sprintf("AddIAMUser: error updating origin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (p *PostgresStorage) AddOriginAccessIdentity(distributionID string, originAccessIdentity string) error {
	var err error

	distribution := &Distribution{}

	err = p.db.QueryRow(checkDistScript, distributionID).Scan(&distribution.DistributionID)

	updateScript := `
    update distributions
      set origin_access_identity = $2
    where distribution_id = $1
`

	_, err = p.db.Exec(updateScript, distributionID, originAccessIdentity)

	if err != nil {
		msg := fmt.Sprintf("AddOriginAccessIdentity: error updating distribution: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}
