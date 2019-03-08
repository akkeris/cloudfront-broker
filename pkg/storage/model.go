// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package storage

import (
	"time"
)

type Service struct {
	ServiceID   string
	Name        string
	HumanName   string
	Description string
	Catagories  string
	Image       string
	Beta        bool
	Deprecated  bool
}

type Plan struct {
	PlanID      string
	ServiceID   string
	Name        string
	HumanName   string
	Description string
	Categories  string
	Free        bool
	CostCents   uint
	CostUnit    string
	Beta        bool
	Depreciated bool
}

type Origin struct {
	OriginID       string
	DistributionID string
	BucketName     string
	IAMUser        string
	AccessKey      string
	SecretKey      string
	OriginUrl      string
	OriginPath     string
	BillingCode    string
}

type Distribution struct {
	DistributionID  string
	PlanID          string
	CloudfrontID    string
	DistributionUrl string
	BillingCode     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       time.Time
}

type Task struct {
	TaskId         string
	DistributionId string
	Action         string
	Status         string
	Retries        int
	Result         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      time.Time
	FinishedAt     time.Time
	DeletedAt      time.Time
}
