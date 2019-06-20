# Akkeris AWS Cloudfront Broker

[![CircleCI](https://circleci.com/gh/akkeris/cloudfront-broker.svg?style=svg)](https://circleci.com/gh/akkeris/cloudfront-broker) [![Codacy Badge](https://api.codacy.com/project/badge/Grade/bca1527edfdd4508aa67d71a813d3de5)](https://www.codacy.com/app/Akkeris/cloudfront-broker?utm_source=github.com&utm_medium=referral&utm_content=akkeris/cloudfront-broker&utm_campaign=Badge_Grade) [![Codacy Badge](https://api.codacy.com/project/badge/Coverage/bca1527edfdd4508aa67d71a813d3de5)](https://www.codacy.com/app/Akkeris/cloudfront-broker?utm_source=github.com&utm_medium=referral&utm_content=akkeris/cloudfront-broker&utm_campaign=Badge_Coverage)

Broker to create AWS **Cloudfront Distributions** for use as a content
distribution network(CDN).
Broker creates an AWS S3 bucket as the primary origin.

## Specifications of created distribution

### Cloudfront Distribution

-   HTTP -> HTTPS
-   cloudfront.net Certs

### S3 Bucket

-   Bucket policy to only allow associated cloudfront distribution read access
-   IAM api user for managing objects in S3 bucket

## Installing

### Settings

Environment Variables

**Required**

-   `DATABASE_URL` - A postgres database for holding broker information
-   `NAME_PREFIX` - Prefix added to name used for bucket and IAM user.
-   `AWS_ACCESS_KEY` - Access key with permissions for cloudfront and s3.
-   `AWS_SECRET_ACCESS_KEY` - Secret key for Access key
-   `REGION` - AWS Region to create S3 buckets

**Optional**

-   `PORT` - Port to listen on, Default 5443
-   `WAIT_SECONDS` - Number of seconds to wait between tasks run. Default 15
-   `MAX_RETRIES` - Max retries to wait for an AWS resource. Default 100

## Build and test

### Build executable

-   make

### Build docker image

-   make image

### test

-   make test
