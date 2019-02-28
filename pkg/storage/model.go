// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package storage

import (
	"time"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

type ServicesSpec struct {
	Service     string `json:"service"`
	Name        string `json:"name"`
	HumanName   string `json:"human_name"`
	Description string `json:"description"`
	Catagories  string `json:"catagories"`
	Image       string `json:"image"`
	Beta        bool   `json:"beta"`
	Deprecated  bool   `json:"deprecated"`
}

type PlanSpec struct {
	basePlan    osb.Plan `json:"-"`
	ID          string   `json:"plan"`
	Name        string   `json:"name"`
	HumanName   string   `json:"human_name"`
	Description string   `json:"description"`
	Categories  string   `json:"categories"`
	Free        bool     `json:"free"`
	CostCents   int32    `json:"cost_cents"`
	CostUnit    string   `json:"cost_unit"`
	Beta        bool     `json:"beta"`
	Depreciated bool     `json:"depreciated"`
}

type OriginSpec struct {
	Bucket      string `json:"S3_BUCKET"`
	Location    string `json:"S3_LOCATION"`
	IAMUser     string `json:"-"`
	AccessKey   string `json:"S3_ACCESS_KEY"`
	SecretKey   string `json:"S3_SECRET_KEY"`
	OriginUrl   string `json:"origin_url,omitempty"`
	OriginPath  string `json:"origin_path,omitempty"`
	BillingCode string `json:"billing_code"`
}

type InstanceSpec struct {
	ID              string       `json:"id"`
	ServiceId       string       `json:"service_id"`
	PlanId          string       `json:"plan_id"`
	CloudfrontId    string       `json:"cloudfront_id"`
	DistributionUrl string       `json:"distribution_url"`
	BillingCode     string       `json:"billing_code"`
	Origins         []OriginSpec `json:"origins"`
	IAMUser         string       `json:"iam_user"`
	AccessKey       string       `json:"access_key"`
	SecretKey       string       `json:"secret_key"`
	Cents           int32        `json:"cents"`
	CostUnit        string       `json:"cost_unit"`
	Beta            bool         `json:"beta"`
	Depreciated     bool         `json:"depreciated"`
	Deleted         bool         `json:"deleted"`
	OperationKey    string
}

type TaskSpec struct {
	TaskId         string
	DistributionId string
	Action         string
	Status         string
	Retries        int
	Result         string
	Created        time.Time
	Updated        time.Time
	Started        time.Time
	Finished       time.Time
	Deleted        bool
}
