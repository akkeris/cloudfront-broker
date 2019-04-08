// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package storage

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type Service struct {
	ServiceID   string
	Name        string
	HumanName   sql.NullString
	Description sql.NullString
	Catagories  sql.NullString
	Image       sql.NullString
	Beta        bool
	Deprecated  bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   pq.NullTime
}

type Plan struct {
	PlanID      string
	ServiceID   string
	Name        string
	HumanName   sql.NullString
	Description sql.NullString
	Catagories  sql.NullString
	Free        bool
	CostCents   uint
	CostUnit    string
	Beta        bool
	Depreciated bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   pq.NullTime
}

type Distribution struct {
	DistributionID       string
	PlanID               string
	CloudfrontID         sql.NullString
	CloudfrontUrl        sql.NullString
	OriginAccessIdentity sql.NullString
	Claimed              bool
	BillingCode          sql.NullString
	Status               string
	CallerReference      string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            pq.NullTime

	Origins *[]Origin
	Task    *[]Task
}

type Origin struct {
	OriginID       string
	DistributionID string
	BucketName     string
	BucketUrl      string
	OriginPath     string
	IAMUser        sql.NullString
	AccessKey      sql.NullString
	SecretKey      sql.NullString
	BillingCode    sql.NullString
	Etag           sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      pq.NullTime
}

type Task struct {
	TaskID         string
	DistributionID string
	Action         string
	Status         string
	OperationKey   sql.NullString
	Retries        int
	Result         sql.NullString
	Metadata       sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      pq.NullTime
	FinishedAt     pq.NullTime
	DeletedAt      pq.NullTime
}
