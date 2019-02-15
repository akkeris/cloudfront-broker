package storage

const servicesQuery string = `
select
    service,
    name,
    human_name,
    description,
    categories,
    image,
    beta,
    depreciated
from services where deleted = false `

const plansQuery string = `
select 
    plans.plan,
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
from plans join services on services.service = plans.service
where services.deleted = false and plans.deleted = false
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

IF NOT exists(SELECT 1 FROM pg_type WHERE typname = 'distribution_type')
THEN
CREATE TYPE distribution_type AS ENUM ('distribution', 'alias');
END IF;

IF NOT exists(SELECT 1 FROM pg_type WHERE typname = 'cents')
THEN
CREATE DOMAIN cents AS int CHECK (value >= 0);
END IF;

IF NOT exists(SELECT 1 FROM pg_type WHERE typname = 'costunit')
THEN
CREATE TYPE costunit AS ENUM ('year', 'month', 'day', 'hour', 'minute', 'second', 'cycle', 'byte', 'megabyte', 'gigabyte', 'terabyte', 'petabyte', 'op', 'unit');
END IF;

IF NOT exists(SELECT 1 FROM pg_type WHERE typname = 'task_status')
THEN
CREATE TYPE task_status AS ENUM ('pending', 'started', 'finished', 'failed');
END IF;

CREATE OR REPLACE FUNCTION mark_updated_column()
RETURNS trigger AS $emp_stamp$
BEGIN
NEW.updated = now();
RETURN NEW;
END;
$emp_stamp$
LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS services
(
service     uuid                     NOT NULL PRIMARY KEY,
name        alpha_numeric            NOT NULL,
human_name  text                     NOT NULL,
description text                     NOT NULL,
categories  varchar(1024)            NOT NULL DEFAULT 'Content Delivery Network, CDN',
image       varchar(1024)            NOT NULL DEFAULT '',

beta        boolean                  NOT NULL DEFAULT FALSE,
depreciated boolean                  NOT NULL DEFAULT FALSE,
deleted     boolean                  NOT NULL DEFAULT FALSE,

created     timestamp WITH TIME ZONE NOT NULL DEFAULT now(),
updated     timestamp WITH TIME ZONE NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS services_updated
ON services;

CREATE TRIGGER services_updated
BEFORE UPDATE
ON services
FOR EACH ROW EXECUTE PROCEDURE mark_updated_column();

CREATE TABLE IF NOT EXISTS plans
(
plan        uuid                                 NOT NULL PRIMARY KEY,
service     uuid REFERENCES services ("service") NOT NULL,
name        alpha_numeric                        NOT NULL,
human_name  text                                 NOT NULL,
description text                                 NOT NULL,
type        distribution_type                    NOT NULL DEFAULT 'distribution',
categories  text                                 NOT NULL DEFAULT '',
free        boolean                              NOT NULL DEFAULT FALSE,
cost_cents  cents                                NOT NULL DEFAULT 1000,
cost_unit   costunit                             NOT NULL DEFAULT 'month',
attributes  json                                 NOT NULL DEFAULT '{}',

beta        boolean                              NOT NULL DEFAULT FALSE,
depreciated boolean                              NOT NULL DEFAULT FALSE,
deleted     boolean                              NOT NULL DEFAULT FALSE,

created     timestamp WITH TIME ZONE             NOT NULL DEFAULT now(),
updated     timestamp WITH TIME ZONE             NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS plans_updated
ON plans;

CREATE TRIGGER plans_updated
BEFORE UPDATE
ON plans
FOR EACH ROW EXECUTE PROCEDURE mark_updated_column();

CREATE TABLE IF NOT EXISTS origins
(
id          uuid                                NOT NULL PRIMARY KEY,
bucket      varchar(1024)                       NOT NULL,
hostname    varchar(1024)                       NOT NULL,
access_key  varchar(128)                        NOT NULL,
secret_key  varchar(128)                        NOT NULL,
created     timestamp WITH TIME ZONE            NOT NULL DEFAULT now(),
updated     timestamp WITH TIME ZONE            NOT NULL DEFAULT now()

);

DROP TRIGGER IF EXISTS origins_updated
ON origins;

CREATE TRIGGER origins_updated
BEFORE UPDATE
ON origins
FOR EACH ROW EXECUTE PROCEDURE mark_updated_column();

CREATE TABLE IF NOT EXISTS distributions
(
id      varchar(1024)                     NOT NULL PRIMARY KEY,
name    varchar(200)                      NOT NULL,
plan    uuid REFERENCES plans ("plan")    NOT NULL,
claimed boolean                           NOT NULL DEFAULT FALSE,
status  varchar(1024)                     NOT NULL DEFAULT 'unknown',
url     varchar(128)                      NOT NULL DEFAULT '',
origin  uuid REFERENCES origins ("id")    NOT NULL,
created timestamp WITH TIME ZONE          NOT NULL DEFAULT now(),
updated timestamp WITH TIME ZONE          NOT NULL DEFAULT now(),
deleted bool                              NOT NULL DEFAULT FALSE
);

DROP TRIGGER IF EXISTS distributions_updated
ON distributions;

CREATE TRIGGER distributions_updated
BEFORE UPDATE
ON distributions
FOR EACH ROW EXECUTE PROCEDURE mark_updated_column();

CREATE TABLE IF NOT EXISTS tasks
(
task     uuid                                          NOT NULL PRIMARY KEY,
distributions varchar(1024) REFERENCES distributions ("id") NOT NULL,
action   varchar(1024)                                 NOT NULL,
status   task_status                                   NOT NULL DEFAULT 'pending',
retries  int                                           NOT NULL DEFAULT 0,
metadata text                                          NOT NULL DEFAULT '',
result   text                                          NOT NULL DEFAULT '',
created  timestamp WITH TIME ZONE                      NOT NULL DEFAULT now(),
updated  timestamp WITH TIME ZONE                      NOT NULL DEFAULT now(),
started  timestamp WITH TIME ZONE,
finished timestamp WITH TIME ZONE,
deleted  bool                                          NOT NULL DEFAULT FALSE
);

DROP TRIGGER IF EXISTS tasks_updated
ON tasks;

CREATE TRIGGER tasks_updated
BEFORE UPDATE
ON tasks
FOR EACH ROW EXECUTE PROCEDURE mark_updated_column();
END
$$
`

const initServicesScript string = `
INSERT INTO services (service, name, human_name, description, categories, beta, depreciated)
VALUES ('3b8d2e75-ca9f-463f-84e4-4b85513f1bc8',
'distribution',
'Akkeris Cloudfront',
'Create a Cloudfront Distribution',
'Cloudfront Distribution, CDN',
FALSE,
FALSE);
`

const initPlansScript string = `
INSERT INTO plans (plan, service, name, human_name, description, categories)
VALUES ('5eac120c-5303-4f55-8a62-46cde1b52d0b',
'3b8d2e75-ca9f-463f-84e4-4b85513f1bc8',
'dist',
'Cloudfront Distribution',
'Create/Update a Cloudfront Distribution',
'cloudfront, cdn');
`
