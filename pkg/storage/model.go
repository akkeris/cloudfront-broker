package storage

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// Service is the OSB Services table
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

// Plan is the OSB plans table
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

// Distribution is the distributions table
type Distribution struct {
	DistributionID       string
	PlanID               string
	CloudfrontID         sql.NullString
	CloudfrontURL        sql.NullString
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

// Origin is the origins table
type Origin struct {
	OriginID       string
	DistributionID string
	BucketName     string
	BucketURL      string
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

// Task is the tasks table
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
