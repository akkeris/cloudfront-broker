package storage

const servicesQuery string = `
select
    service_id,
    name,
    human_name,
    description,
    categories,
    image,
    beta,
    depreciated
from services where deleted_at is null `

const plansQuery string = `
select 
    plans.plan_id,
    plans.name,
    services.name,
    plans.human_name,
    plans.description,
    plans.categories,
    plans.free,
    plans.cost_cents,
    plans.cost_unit,
    plans.beta,
    plans.depreciated
from plans join services on services.service_id = plans.service_id
where services.deleted_at is null and plans.deleted_at is null
`

const taskQuery string = `
select
  task.task,
  task.distributionId,
  task.action,
  task.status,
  task.retries,
  task.metadata,
  task.result,
  task.created_at,
  task.updated_at,
  task.started_at,
  task.finished_at,
from tasks join distributions on distributions.distributin_id = task.distribution_id
where distributions.deleted_at is null and tasks.deleted_at is null
`

const createScript string = `
DO
  $$
    BEGIN
      CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

      IF NOT exists(SELECT 1 FROM pg_type WHERE typname = 'alpha_numeric')
      THEN
        CREATE DOMAIN alpha_numeric AS varchar(128) CHECK (value ~ '^[A-z0-9\-]+$');
      END IF;

      IF NOT exists(SELECT 1 FROM pg_type WHERE typname = 'cents')
      THEN
        CREATE DOMAIN cents AS int CHECK (value >= 0);
      END IF;

      CREATE OR REPLACE FUNCTION mark_updated_column()
        RETURNS trigger AS
      $emp_stamp$
      BEGIN
        NEW.updated_at = now();
        RETURN NEW;
      END;
      $emp_stamp$
        LANGUAGE plpgsql;

      CREATE TABLE IF NOT EXISTS services
      (
        service_id  uuid                     NOT NULL PRIMARY KEY,
        name        alpha_numeric            NOT NULL UNIQUE,
        human_name  text                     NOT NULL,
        description text                     NOT NULL,
        categories  varchar(1024)            NOT NULL DEFAULT '',
        image       varchar(1024)            NOT NULL DEFAULT '',

        beta        boolean                  NOT NULL DEFAULT FALSE,
        depreciated boolean                  NOT NULL DEFAULT FALSE,

        created_at  timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
        updated_at  timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
        deleted_at  timestamp WITH TIME ZONE
      );

      DROP TRIGGER IF EXISTS services_updated
        ON services;

      CREATE TRIGGER services_updated
        BEFORE UPDATE
        ON services
        FOR EACH ROW
      EXECUTE PROCEDURE mark_updated_column();

      CREATE TABLE IF NOT EXISTS plans
      (
        plan_id     uuid                                    NOT NULL PRIMARY KEY,
        service_id  uuid REFERENCES services ("service_id") NOT NULL,
        name        alpha_numeric                           NOT NULL UNIQUE,
        human_name  text                                    NOT NULL,
        description text                                    NOT NULL,
        categories  text                                    NOT NULL DEFAULT '',
        free        boolean                                 NOT NULL DEFAULT FALSE,
        cost_cents  cents                                   NOT NULL DEFAULT 1000,
        cost_unit   costunit                                NOT NULL DEFAULT 'month',
        attributes  json                                    NOT NULL DEFAULT '{}',

        beta        boolean                                 NOT NULL DEFAULT FALSE,
        depreciated boolean                                 NOT NULL DEFAULT FALSE,

        created_at  timestamp WITH TIME ZONE                NOT NULL DEFAULT now(),
        updated_at  timestamp WITH TIME ZONE                NOT NULL DEFAULT now(),
        deleted_at  timestamp WITH TIME ZONE
      );

      DROP TRIGGER IF EXISTS plans_updated
        ON plans;

      CREATE TRIGGER plans_updated
        BEFORE UPDATE
        ON plans
        FOR EACH ROW
      EXECUTE PROCEDURE mark_updated_column();

      CREATE TABLE IF NOT EXISTS distributions
      (
        distribution_id uuid  NOT NULL PRIMARY KEY,

        plan_id         uuid REFERENCES plans ("plan_id") NOT NULL,
        cloudfront_id   varchar(200)                      UNIQUE,
        cloudfront_url  varchar(200),
        origin_access_identity varchar(200),
        claimed         boolean                           NOT NULL DEFAULT FALSE,
        status          varchar(1024)                     NOT NULL DEFAULT 'new',
        billing_code    varchar(200),

        created_at      timestamp WITH TIME ZONE          NOT NULL DEFAULT now(),
        updated_at      timestamp WITH TIME ZONE          NOT NULL DEFAULT now(),
        deleted_at      timestamp WITH TIME ZONE
      );

      DROP TRIGGER IF EXISTS distributions_updated
        ON distributions;

      CREATE TRIGGER distributions_updated
        BEFORE UPDATE
        ON distributions
        FOR EACH ROW
      EXECUTE PROCEDURE mark_updated_column();

      CREATE TABLE IF NOT EXISTS origins
      (
        origin_id  uuid                     NOT NULL PRIMARY KEY,
        distribution_id uuid REFERENCES distributions ("distribution_id"),

        bucket     varchar(1024)            NOT NULL UNIQUE,
        bucket_url varchar(1024)            NOT NULL,
        iam_user   alpha_numeric,
        access_key varchar(128),
        secret_key varchar(128),

        created_at timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
        updated_at timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
        deleted_at timestamp WITH TIME ZONE
      );

      DROP TRIGGER IF EXISTS origins_updated
        ON origins;

      CREATE TRIGGER origins_updated
        BEFORE UPDATE
        ON origins
        FOR EACH ROW
      EXECUTE PROCEDURE mark_updated_column();

      CREATE TABLE IF NOT EXISTS tasks
      (
        task            uuid                                                       NOT NULL PRIMARY KEY,
        distribution_id uuid REFERENCES distributions ("distribution_id") NOT NULL,
        action          varchar(1024)                                              NOT NULL,
        state           varchar(128)                                               NOT NULL DEFAULT 'new',
        retries         int                                                        NOT NULL DEFAULT 0,
        result          text,
        metadata        text,

        created_at      timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
        updated_at      timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
        started_at      timestamp WITH TIME ZONE,
        finished_at     timestamp WITH TIME ZONE,
        deleted_at      timestamp WITH TIME ZONE
      );

      DROP TRIGGER IF EXISTS tasks_updated
        ON tasks;

      CREATE TRIGGER tasks_updated
        BEFORE UPDATE
        ON tasks
        FOR EACH ROW
      EXECUTE PROCEDURE mark_updated_column();
    END
    $$
`

const initServicesScript string = `
INSERT INTO services (service_id, name, human_name, description, categories, beta, depreciated)
VALUES ('3b8d2e75-ca9f-463f-84e4-4b85513f1bc8',
'distribution',
'Akkeris Cloudfront',
'Create a Cloudfront Distribution',
'Cloudfront Distribution, CDN',
FALSE,
FALSE);
`

const initPlansScript string = `
INSERT INTO plans (plan_id, service_id, name, human_name, description, categories)
VALUES ('5eac120c-5303-4f55-8a62-46cde1b52d0b',
'3b8d2e75-ca9f-463f-84e4-4b85513f1bc8',
'dist',
'Cloudfront Distribution',
'Create/Update a Cloudfront Distribution',
'cloudfront, cdn');
`
