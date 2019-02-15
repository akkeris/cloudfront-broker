// Author: ned.hanks
// Date Created: ned.hanks
// Project: 
package storage

import (
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
  ID          string `json:"plan"`
  Name        string `json:"name"`
  HumanName   string `json:"human_name"`
  Description string `json:"description"`
  Categories  string `json:"categories"`
  Free        bool   `json:"free"`
  CostCents   int32  `json:"cost_cents"`
  CostUnit    string `json:"cost_unit"`
  Beta        bool   `json:"beta"`
  Depreciated bool   `json:"depreciated"`
}

type OriginSpec struct {
  Bucket      string `json:"S3_BUCKET"`
  Location    string `json:"S3_LOCATION"`
  AccessKey   string `json:"S3_ACCESS_KEY"`
  SecretKey   string `json:"S3_SECRET_KEY"`
  OriginUrl   string `json:"origin_url,omitempty"`
  OriginPath  string `json:"origin_path,omitempty"`
  BillingCode string `json:"billing_code"`
}

type InstanceSpec struct {
  ID              string       `json:"id"`
  ServiceID       string       `json:"service_id"`
  PlanID          string       `json:"plan_id"`
  CloudfrontID    string       `json:"cloudfront_id"`
  DistributionUrl string       `json:"distribution_url"`
  BillingCode     string       `json:"billing_code"`
  Origins         []OriginSpec `json:"origins"`
  Cents           int32        `json:"cents"`
  CostUnit        string       `json:"cost_unit"`
  Beta            bool         `json:"beta"`
  Depreciated     bool         `json:"depreciated"`
  Deleted         bool         `json:"deleted"`
}
