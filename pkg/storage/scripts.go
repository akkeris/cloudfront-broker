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
      IF NOT exists (select 1 from pg_type where typname = 'costunit') then
          CREATE TYPE costunit as enum('year', 'month', 'day', 'hour', 'minute', 'second', 'cycle', 'byte', 'megabyte', 'gigabyte', 'terabyte', 'petabyte', 'op', 'unit');
      end if;

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
        human_name  text,
        description text,
        categories  varchar(1024),
        image       varchar(1024),

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
        human_name  text,
        description text,
        categories  text,
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
        bind_id         uuid                              UNIQUE,
        cloudfront_id   varchar(200)                      UNIQUE,
        cloudfront_url  varchar(200),
        origin_access_identity varchar(200),
        caller_reference varchar(200)                     NOT NULL,
        etag varchar(200),
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

        bucket_name     varchar(1024)            NOT NULL UNIQUE,
        bucket_url varchar(1024)            NOT NULL,
        origin_path varchar(1024)           NOT NULL DEFAULT '/',
        iam_user   alpha_numeric,
        access_key varchar(128),
        secret_key varchar(128),
        billing_code varchar(128),

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
        task_id         uuid  NOT NULL PRIMARY KEY,
        distribution_id uuid REFERENCES distributions ("distribution_id") NOT NULL,
        status          varchar(128),
        action          varchar(128) NOT NULL DEFAULT 'new',
        retries         int NOT NULL DEFAULT 0,
        result          text,
        metadata        text,
        operation_key   varchar(128),

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

const checkPlanScript string = `
  select count(*) from plans
  where plan_id = $1
  and deleted_at is null
`

const selectDistScript string = `
  select 
    distribution_id, 
    plan_id, 
    cloudfront_id, 
    cloudfront_url, 
    origin_access_identity, 
    claimed, 
    status, 
    billing_code, 
    caller_reference,
    created_at,
    updated_at,
    deleted_at
  from distributions
  where distribution_id = $1
`

const updateDistributionScript string = `
  update distributions
  set status = $2,
    deleted_at = $3
  where distribution_id = $1
  returning distribution_id, status
`

const updateDistributionWithCloudfrontScript string = `
  update distributions
  set cloudfront_id = $2,
    cloudfront_url = $3
  where distribution_id = $1
  returning plan_id, cloudfront_id, cloudfront_url, origin_access_identity, claimed, status, billing_code
`

const updateDistributionDeletedScript string = `
  update distributions
  set deleted_at = now()
  where distribution_id = $1
  returning distribution_id
`

const updateDistWithOAIScript string = `
  update distributions
    set origin_access_identity = $2
  where distribution_id = $1
`

const insertOriginScript string = `
insert into origins
  (origin_id, distribution_id, bucket_name, bucket_url, billing_code)
values 
  (uuid_generate_v4(), $1, $2, $3, $4) returning origin_id;
`

const selectOriginScript string = `
  select origin_id, distribution_id, bucket_name, bucket_url, origin_path, iam_user, access_key, secret_key
  from origins 
`

const updateOriginDeletedScript string = `
  update origins
  set deleted_at = now()
  where origin_id = $1
  and distribution_id = $2
  returning origin_id, distribution_id;
`

const updateOriginWithIAMScript string = `
  update origins
    set iam_user = $2
    where origin_id = $1
`

const updateOriginWithAccessKeyScript string = `
  update origins
    set access_key = $2,
        secret_key = $3
    where origin_id = $1
`

const insertTaskScript string = `
  insert into tasks
  (task_id, distribution_id, status, action, operation_key, retries, started_at)
  values 
  (uuid_generate_v4(), $1, $2, $3, $4, $5, $6) returning task_id
`

const selectTaskScript string = `
  select task_id, distribution_id, operation_key, status, action, retries, metadata, result
  from tasks
  where distribution_id = $1
  and deleted_at is null
  order by created_at desc
`

const popNextTaskScript string = `
  update tasks set
    status = $1,
    updated_at = now() 
  where task_id in ( 
    select task_id
    from tasks 
    where status in ('new', 'pending') 
    and deleted_at is null 
    and finished_at is null 
    order by updated_at asc limit 1 )
  returning task_id, distribution_id, operation_key, status, action, retries, metadata, result, started_at, updated_at
`

const updateTaskActionScript string = `
  update tasks set
    action = $2,
    status = $3, 
    retries = $4,
    result = $5,
    metadata = $6,
    finished_at = $7,
    started_at = $8,
    updated_at = now()
  where task_id = $1
  and finished_at is null
  and deleted_at is null
  returning task_id, distribution_id, action, status, retries, result, metadata, created_at, updated_at, started_at, finished_at
`
