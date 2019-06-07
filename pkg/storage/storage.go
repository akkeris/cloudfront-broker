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
	"time"

	"github.com/golang/glog"
	"github.com/lib/pq"

	_ "github.com/lib/pq"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

const (
	DistributionNotFound = "DistributionNotFound"
	DistributionFound    = "DistributionFound"
	OriginNotFound       = "OriginNotFound"
)

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

func nullStringValue(ns sql.NullString) string {
	var blank string

	if ns.Valid {
		return ns.String
	}

	return blank
}

func (p *PostgresStorage) NullString(ns sql.NullString) *string {
	var r string
	/*
		if s.Valid {
			return &s.String
		}

		return nil
	*/

	r = nullStringValue(ns)
	return &r
}

func SetNullString(s string) sql.NullString {
	ns := sql.NullString{}

	switch {
	case s == "":
		ns.String = ""
		ns.Valid = false
	default:
		ns.String = s
		ns.Valid = true
	}
	return ns
}

func SetNullTime(t *time.Time) pq.NullTime {
	nt := pq.NullTime{}
	switch {
	case t == nil:
		nt.Valid = false
	default:
		nt.Time = *t
		nt.Valid = true
	}
	return nt
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

func getCatalogPlans(db *sql.DB, serviceId string) ([]osb.Plan, error) {
	rows, err := db.Query(plansQuery+"and services.service_id = $1 order by plans.name", serviceId)
	if err != nil {
		glog.Errorf("getPlans query failed: %s\n", err.Error())
		return nil, err
	}
	defer rows.Close()

	plans := make([]osb.Plan, 0)

	for rows.Next() {
		var planID, name, costUnit string
		var humanName, description, catagories sql.NullString
		var cents int32
		var free, beta, depreciated bool

		err := rows.Scan(&planID, &name, &name, &humanName, &description, &catagories, &free, &cents, &costUnit, &beta, &depreciated)
		if err != nil {
			glog.Errorf("Scan from plans query failed: %s\n", err.Error())
			return nil, errors.New("Scan from plans query failed: " + err.Error())
		}

		plans = append(plans, osb.Plan{
			ID:          planID,
			Name:        name,
			Description: nullStringValue(description),
			Free:        &free,
			Metadata: map[string]interface{}{
				"humanName":   nullStringValue(humanName),
				"cents":       cents,
				"cost_unit":   costUnit,
				"beta":        beta,
				"depreciated": depreciated,
				"catagories":  nullStringValue(catagories),
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
		msg := fmt.Sprintf("GetServiceCatalog: Unable to get services: " + err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}
	defer rows.Close()

	for rows.Next() {
		var serviceID, serviceName string
		var serviceDescription, serviceHumanName, serviceCatagories, serviceImage sql.NullString
		var beta, deprecated bool

		err = rows.Scan(&serviceID, &serviceName, &serviceHumanName, &serviceDescription, &serviceCatagories, &serviceImage, &beta, &deprecated)
		if err != nil {
			glog.Errorf("Unable to get services: %s\n", err.Error())
			return nil, errors.New("Unable to scan services: " + err.Error())
		}

		plans, err := getCatalogPlans(p.db, serviceID)
		if err != nil {
			glog.Errorf("Unable to get plans for %s: %s\n", serviceName, err.Error())
			return nil, errors.New("Unable to get plans for " + serviceName + ": " + err.Error())
		}

		osbPlans := make([]osb.Plan, 0)
		for _, plan := range plans {
			osbPlans = append(osbPlans, plan)
		}

		services = append(services, osb.Service{
			Name:                serviceName,
			ID:                  serviceID,
			Description:         nullStringValue(serviceDescription),
			Bindable:            false,
			BindingsRetrievable: true,
			PlanUpdatable:       &planUpdateable,
			Tags:                strings.Split(nullStringValue(serviceCatagories), ","),
			Metadata: map[string]interface{}{
				"name":  nullStringValue(serviceHumanName),
				"image": nullStringValue(serviceImage),
			},
			Plans: osbPlans,
		})
	}
	return services, nil
}

func (p *PostgresStorage) GetDistributionWithDeleted(distributionID string) (*Distribution, error) {
	distribution := &Distribution{}

	err := p.db.QueryRow(selectDistScript, distributionID).Scan(
		&distribution.DistributionID,
		&distribution.PlanID,
		&distribution.CloudfrontID,
		&distribution.CloudfrontUrl,
		&distribution.OriginAccessIdentity,
		&distribution.CloudfrontUrl,
		&distribution.Status,
		&distribution.BillingCode,
		&distribution.CallerReference,
		&distribution.CreatedAt,
		&distribution.UpdatedAt,
		&distribution.DeletedAt,
	)

	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("GetDistribution: distribution not found: %s", err.Error())
		glog.Info(msg)
		return nil, errors.New(DistributionNotFound)
	case err != nil:
		msg := fmt.Sprintf("GetDistribution: error finding distribution: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return distribution, nil

}

func (p *PostgresStorage) GetDistribution(distributionID string) (*Distribution, error) {
	selectDist := selectDistScript + "and deleted_at is null"

	distribution := &Distribution{}

	err := p.db.QueryRow(selectDist, distributionID).Scan(
		&distribution.DistributionID,
		&distribution.PlanID,
		&distribution.CloudfrontID,
		&distribution.CloudfrontUrl,
		&distribution.OriginAccessIdentity,
		&distribution.CloudfrontUrl,
		&distribution.Status,
		&distribution.BillingCode,
		&distribution.CallerReference,
		&distribution.CreatedAt,
		&distribution.UpdatedAt,
		&distribution.DeletedAt,
	)

	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("GetDistribution: distribution not found: %s", err.Error())
		glog.Info(msg)
		return nil, errors.New(DistributionNotFound)
	case err != nil:
		msg := fmt.Sprintf("GetDistribution: error finding distribution: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return distribution, nil

}

func (p *PostgresStorage) NewDistribution(distributionID string, planID string, billingCode string, callerReference string, status string) error {
	var err error
	var cnt int

	err = p.db.QueryRow(checkPlanScript, planID).Scan(&cnt)

	if err != nil && err.Error() == "sql: no rows in result set" {
		msg := fmt.Sprintf("NewDistribution: can not find plan: %s", err.Error())
		glog.Error(msg)
		errors.New(msg)
	} else if err != nil {
		msg := fmt.Sprintf("NewDistribution: error finding plan: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	if _, err = p.GetDistribution(distributionID); err == nil {
		msg := "NewDistribution: found distribution"
		glog.Error(msg)
		return errors.New(DistributionFound)
	}

	distribution := &Distribution{
		PlanID: planID,
		BillingCode: sql.NullString{
			String: billingCode,
			Valid:  true},
	}

	insertDistScript := `insert into distributions
    (distribution_id, plan_id, billing_code, caller_reference, status) 
    values 
    ($1, $2, $3, $4, $5) returning distribution_id;`

	err = p.db.QueryRow(insertDistScript, distributionID, planID, billingCode, callerReference, status).Scan(&distribution.DistributionID)
	if err != nil {
		msg := fmt.Sprintf("NewDistribution: error inserting distribution: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	glog.Infof("NewDistribution: distribution id: %s", distribution.DistributionID)

	return nil
}

func (p *PostgresStorage) UpdateDistributionStatus(distributionID string, status string, delete bool) error {
	d := &Distribution{}

	var deletedAt pq.NullTime

	if delete {
		n := time.Now()
		deletedAt = SetNullTime(&n)
	} else {
		deletedAt = SetNullTime(nil)
	}

	err := p.db.QueryRow(updateDistributionScript, &distributionID, &status, &deletedAt).Scan(&d.DistributionID, &d.Status)

	if err != nil && err.Error() == "sql: no rows in result set" {
		msg := fmt.Sprintf("UpdateDistributionStatus: distribution not found: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	} else if err != nil {
		msg := fmt.Sprintf("UpdateDistributionStatus: error updating distribution: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (p *PostgresStorage) UpdateDeleteDistribution(distributionID string) error {
	var distDeleted string

	err := p.db.QueryRow(updateDistributionDeleted, &distributionID).Scan(&distDeleted)

	if err != nil {
		msg := fmt.Sprintf("DeleteDistribution: error setting deleted_at: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (p *PostgresStorage) UpdateDistributionCloudfront(distributionID string, cloudfrontID string, cloudfrontURL string) (*Distribution, error) {
	var err error
	d := &Distribution{}

	cloudfrontIDStr := &sql.NullString{
		String: cloudfrontID,
		Valid:  true,
	}

	cloudfrontURLStr := &sql.NullString{
		String: cloudfrontURL,
		Valid:  true,
	}

	err = p.db.QueryRow(updateDistributionWithCloudfrontScript, &distributionID, cloudfrontIDStr, cloudfrontURLStr).Scan(
		&d.PlanID, &d.CloudfrontID, &d.CloudfrontUrl, &d.OriginAccessIdentity, &d.Claimed, &d.Status, &d.BillingCode)

	if err != nil && err.Error() == "sql: no rows in result set" {
		msg := fmt.Sprintf("UpdateDistributionCloudfront: distribution not found: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	} else if err != nil {
		msg := fmt.Sprintf("UpdateDistributionCloudfront: error updating distribution: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return d, nil
}

func (p *PostgresStorage) AddOrigin(distributionID string, bucketName string, bucketURL string, originPath string, billingCode string) (*Origin, error) {
	glog.Info("===== AddOrigin =====")

	origin := &Origin{
		DistributionID: distributionID,
		BucketName:     bucketName,
		BucketUrl:      bucketURL,
		OriginPath:     originPath,
		BillingCode:    sql.NullString{String: billingCode, Valid: true},
	}

	err := p.db.QueryRow(insertOriginScript,
		distributionID, bucketName, bucketURL, origin.BillingCode).Scan(&origin.OriginID)

	if err != nil {
		msg := fmt.Sprintf("AddOrigin: error inserting origin: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	glog.Infof("AddOrigin: originId: %s", origin.OriginID)

	return origin, nil
}

func (p *PostgresStorage) GetOriginByID(originID string) (*Origin, error) {
	var selectOriginById string = selectOriginScript + "where origin_id = $1 and deleted_at is null"

	return p.GetOrigin(selectOriginById, originID)
}

func (p *PostgresStorage) GetOriginByDistributionID(distributionID string) (*Origin, error) {
	var selectOriginById string = selectOriginScript + "where distribution_id = $1 and deleted_at is null"

	return p.GetOrigin(selectOriginById, distributionID)
}

func (p *PostgresStorage) GetOrigin(selectOrigin string, selectKey string) (*Origin, error) {

	origin := &Origin{}

	err := p.db.QueryRow(selectOrigin, selectKey).Scan(
		&origin.OriginID,
		&origin.DistributionID,
		&origin.BucketName,
		&origin.BucketUrl,
		&origin.OriginPath,
		&origin.IAMUser,
		&origin.AccessKey,
		&origin.SecretKey,
	)

	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("GetOrigin: origin not found: %s", err.Error())
		glog.Info(msg)
		return nil, errors.New(OriginNotFound)
	case err != nil:
		msg := fmt.Sprintf("GetOrigin: error finding origin: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return origin, nil
}

func (p *PostgresStorage) UpdateDeleteOrigin(distributionID string, originID string) (*Origin, error) {
	origin := &Origin{}

	err := p.db.QueryRow(updateOriginScript, originID, distributionID).Scan(
		&origin.OriginID,
		&origin.DistributionID,
	)
	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("UpdateDeleteOrigin: origin not found: %s", err.Error())
		glog.Info(msg)
		return nil, errors.New(OriginNotFound)
	case err != nil:
		msg := fmt.Sprintf("UpdateDeleteOrigin: error updating deleted at: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return origin, nil
}

func (p *PostgresStorage) AddIAMUser(originID string, iAMUser string) error {
	var err error

	_, err = p.GetOriginByID(originID)

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

	_, err = p.db.Exec(updateOriginWithIAMScript, originID, iAMUser)

	if err != nil {
		msg := fmt.Sprintf("AddIAMUser: error updating origin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (p *PostgresStorage) AddAccessKey(originID string, accessKey string, secretKey string) error {
	var err error

	_, err = p.GetOriginByID(originID)

	switch {
	case err == sql.ErrNoRows:
		msg := fmt.Sprintf("AddAccessKey: origin not found: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	case err != nil:
		msg := fmt.Sprintf("AddAccessKey: error finding origin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	_, err = p.db.Exec(updateOriginWithAccessKeyScript, originID, accessKey, secretKey)

	if err != nil {
		msg := fmt.Sprintf("AddAccessKey: error updating origin: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (p *PostgresStorage) UpdateDistributionWIthOriginAccessIdentity(distributionID string, originAccessIdentity string) error {
	var err error

	_, err = p.GetDistribution(distributionID)

	if err != nil {
		msg := fmt.Sprintf("UpdateDistributionWIthOriginAccessIdentity: distribution not found: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	_, err = p.db.Exec(updateDistWithOAIScript, distributionID, originAccessIdentity)

	if err != nil {
		msg := fmt.Sprintf("UpdateDistributionWIthOriginAccessIdentity: error updating distribution: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (p *PostgresStorage) deleteItDistribution(distributionID string) error {
	delScript := "delete from distributions where distribution_id = $1"

	_, err := p.db.Exec(delScript, distributionID)

	return err
}

func (p *PostgresStorage) deleteItOrigin(originID string) error {
	delScript := "delete from origins where origin_id = $1"

	_, err := p.db.Exec(delScript, originID)

	return err
}

func (p *PostgresStorage) deleteItTask(taskID string) error {
	delScript := "delete from tasks where task_id = $1"

	_, err := p.db.Exec(delScript, taskID)

	return err
}
