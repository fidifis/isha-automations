# Isha Automations

This repo contains infrastructure code for automation systems.
We are running on [AWS](https://aws.amazon.com/), infrastructure is defined by [Pulumi](https://www.pulumi.com/) written in TypeScript.
We are using serverless AWS Lambda functions and Step Functions. Code is written in Go.

# Documentation

[docs](./docs/README.md)

# pulumi setup

pulumi login 's3://isha-automations-state-956941652442-eu-north-1-an?region=eu-north-1&awssdk=v2&profile=isha-automations'

pulumi install

# DMQ Maker

Code for DMQ Maker (and the Lambda function) is at [this repo](https://github.com/Fidifis/DMQMaker)
