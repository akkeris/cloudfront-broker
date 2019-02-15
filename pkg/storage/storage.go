// Author: ned.hanks
// Date Created: ned.hanks
// Project: cloudfront-broker
package storage

import (
  "context"
  "database/sql"
  "errors"
  "os"
  "os/signal"
  "reflect"
  "strings"
  "syscall"

  "k8s.io/klog"

  "cloudfront-broker/pkg/utils"

  _ "github.com/lib/pq"
  osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func (i *InstanceSpec) Match(other *InstanceSpec) bool {
  return reflect.DeepEqual(i, other)
}

type Storage interface {
  GetServices() ([]osb.Service, error)
  GetPlans(string) ([]PlanSpec, error)
  GetPlanByID(string) (*PlanSpec, error)
}

type PostgresStorage struct {
  Storage
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
    klog.Error("Unable to connect to database, none was specified in the environment via DATABASE_URL or through the -database cli option.")
    return nil, errors.New("unable to connect to database, none was specified in the environment via DATABASE_URL or through the -database cli option")
  }

  klog.Infof("DATABASE_URL=%s", DatabaseUrl)

  db, err := sql.Open("postgres", DatabaseUrl)
  if err != nil {
    klog.Errorf("Unable to open database: %s\n", err.Error())
    return nil, errors.New("Unable to open database: " + err.Error())
  }

  pgStorage := PostgresStorage{
    db: db,
  }

  go cancelOnInterrupt(ctx, db)

  _, err = db.Exec(createScript)
  if err != nil {
    klog.Errorf("error creating database tables: %s\n", err)
    return nil, err
  }

  var cnt int
  err = db.QueryRow("select count(*) from services;").Scan(&cnt)

  if err != nil || cnt == 0 {
    _, err = db.Exec(initServicesScript)
    if err != nil {
      klog.Errorf("error initializing services: %s\n", err)
      return nil, err
    }
  }

  err = db.QueryRow("select count(*) from plans;").Scan(&cnt)
  if err != nil || cnt == 0 {
    _, err = db.Exec(initPlansScript)
    if err != nil {
      klog.Errorf("error initializing plans: %s\n", err)
      return nil, err
    }
  }

  return &pgStorage, nil
}

func (p *PostgresStorage) getPlans(subquery string, arg string) ([]PlanSpec, error) {
  rows, err := p.db.Query(plansQuery+subquery, arg)
  if err != nil {
    klog.Errorf("getPlans query failed: %s\n", err.Error())
    return nil, err
  }
  defer rows.Close()

  plans := make([]PlanSpec, 0)

  for rows.Next() {
    var id, name, serviceName, humanName, description, categories, costUnit string
    var cents int32
    var free, beta, depreciated bool

    err := rows.Scan(&id, &name, &serviceName, &humanName, &description, &categories, &free, &cents, &costUnit, &beta, &depreciated)
    if err != nil {
      klog.Errorf("Scan from plans query failed: %s\n", err.Error())
      return nil, errors.New("Scan from plans query failed: " + err.Error())
    }

    plans = append(plans, PlanSpec{
      basePlan: osb.Plan{
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
      },
      ID:          id,
      Name:        name,
      HumanName:   humanName,
      Description: description,
      Categories:  categories,
      Free:        free,
      CostCents:   cents,
      CostUnit:    costUnit,
      Beta:        beta,
      Depreciated: depreciated,
    })
  }

  return plans, nil
}

func (p *PostgresStorage) GetServices() ([]osb.Service, error) {
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
      klog.Errorf("Unable to get services: %s\n", err.Error())
      return nil, errors.New("Unable to get services: " + err.Error())
    }

    plans, err := p.GetPlans(service_id)
    if err != nil {
      klog.Errorf("Unable to get plans for %s: %s\n", service_name, err.Error())
      return nil, errors.New("Unable to get plans for " + service_name + ": " + err.Error())
    }

    osbPlans := make([]osb.Plan, 0)
    for _, plan := range plans {
      osbPlans = append(osbPlans, plan.basePlan)
    }
    services = append(services, osb.Service{
      Name:                service_name,
      ID:                  service_id,
      Description:         service_description,
      Bindable:            true,
      BindingsRetrievable: true,
      PlanUpdatable:       utils.TruePtr(),
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

func (p *PostgresStorage) GetPlans(serviceId string) ([]PlanSpec, error) {
  return p.getPlans(" and services.service::varchar(1024) = $1::varchar(1024) order by plans.name", serviceId)
}

func (p *PostgresStorage) GetPlanByID(planId string) (*PlanSpec, error) {
  plans, err := p.getPlans(" and plans.plan::varchar(1024) = $1::varchar(1024)", planId)

  if err != nil {
    return nil, err
  }

  if len(plans) == 0 {
    return nil, errors.New("plans not found")
  }

  return &plans[0], nil
}
