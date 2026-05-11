# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

AWS serverless automation infrastructure for Isha Foundation workflows. Infrastructure is defined with Pulumi (TypeScript), Lambda functions are written in Go, deployed to AWS (`eu-north-1`). State is stored in S3: `s3://fidifis-iac-states?region=eu-west-1&awssdk=v2&profile=fidifis-isha-automations`.

## Environment Setup

Nix is optional but recommended. The dev shell provides: `pulumi`, `pulumi-language-nodejs`, `nodejs`, `go`, `dotnet-sdk`.

**With Nix:**
```bash
nix develop   # or direnv allow (uses .envrc which runs `use flake`)
```

**Without Nix** — install the following tools manually:
- [Go](https://go.dev/dl/) ≥ 1.21
- [Node.js](https://nodejs.org/) ≥ 20 (for Pulumi TypeScript)
- [Pulumi CLI](https://www.pulumi.com/docs/install/)
- Docker or Podman (required for `build-lambda.sh` and `download-assets.sh`)

## Build Commands

**Build all Go Lambda ZIPs** (requires Docker or Podman):
```bash
./build-lambda.sh
```
Output ZIPs go to `bin/`. The build script runs two passes:
1. `scripts/build-go.sh` — builds every `go.mod` directory as an arm64 Linux binary (skips `build.json` entries with `"type": "lib"`)
2. `scripts/build-special.sh` — runs any `build.sh` scripts found in `code/`

Without a container engine, run the scripts directly — the container is only a convenience wrapper for dependencies:
- Go lambdas: `./scripts/build-go.sh` (needs `go`, `zip`, `jq`; CGO is disabled so cross-compilation works natively)
- ffmpeg layer: `./code/video-render/ffmpeg-layer/build.sh bin/video-render-ffmpeg-layer.zip` (needs `curl`, `tar` with xz support, `zip`)

**Download font/asset files** (requires Docker or Podman):
```bash
./download-assets.sh
```

**Pulumi setup** (first time):
```bash
cd pulumi
pulumi login 's3://fidifis-iac-states?region=eu-west-1&awssdk=v2&profile=fidifis-isha-automations'
npm install
```

**Deploy / preview**:
```bash
cd pulumi
pulumi preview --stack dev    # or live
pulumi up --stack dev
```

## Architecture

### Request Flow

API Gateway (REST, regional, dual-stack) → **Spark Lambda** → Step Functions state machine → individual Lambda steps

The **Spark Lambda** (`code/spark/`) sits between API Gateway and every Step Functions workflow. It:
1. Receives the API Gateway request (with `stateMachineArn`, `input`, `apiKeyId`, `traceHeader`)
2. Injects a random `jobId` into the payload
3. Calls `sfn.StartExecution`
4. Returns `{ jobId }` to the caller

All workflows are started asynchronously this way. The `jobId` threads through the entire state machine for tracing.

### Pulumi Structure

```
pulumi/
  index.ts          — root: wires CommonRes → feature modules → RestApiGateway
  commonRes.ts       — shared S3 buckets, SSM param, Spark Lambda, IAM roles
  helperLambda.ts    — s3-gdrive-transfer Lambda (shared by multiple workflows)
  components/
    lambda.ts        — GoLambda component resource (wraps Function + Role + LogGroup + Policy)
    apiGateway.ts    — RestApiGateway component resource
  dmqs/             — DMQ Maker workflow
  video-render/     — Video render workflow (subtitle burn-in)
  smc-import/       — SMC catalog import workflow
```

Each feature module (dmqs, video-render, smc-import) exports a `routes: ApiGatewayRoute[]` array that gets merged into the single REST API.

### Shared Resources (CommonRes)

| Resource | Purpose |
|---|---|
| `codeBucket` | Lambda deployment ZIPs |
| `assetsBucket` | Static assets (fonts, etc.) |
| `procFilesBucket` | Temporary processing files (90-day lifecycle) |
| `gcpConfigParam` | SSM param: GCP workload federation config (`clientLibConfig.json`) |
| `sparkLambda` | Entry-point Lambda for all SFN workflows |
| `sparkApiGwExec` | IAM role: API GW → invoke Spark Lambda |
| `sfnExec` | IAM role: Step Functions → invoke any project Lambda |

### GCP Integration

All Lambdas that touch Google Drive read GCP federation config from SSM (`SSM_GCP_CONFIG` env var). No long-lived secrets; it uses workload identity federation between AWS and GCP project `isha-automations-231309`. Service account email: `awscloud@isha-automations-231309.iam.gserviceaccount.com`.

### Go Lambda Conventions

- All Lambdas target `linux/arm64` (except those needing the ffmpeg layer, which use `x86_64`)
- Binary is named `bootstrap` (AWS Lambda custom runtime)
- Shared library code lives in `code/lib/` and is referenced as a local Go module (`lambdalib/...`)
- `code/lib/build.json` has `"type": "lib"` so the build script skips it

### API Authorization

API keys per team (`gr-cz`, `gr-demo`), enforced via API Gateway usage plans. Pass key in `x-api-key` header. Rate limit: 1 req/s burst 10, quota 50/day.

### Stacks

- `dev` — development environment
- `live` — production

AWS profile used: `fidifis-isha-automations`; region: `eu-north-1`.

## Documentation (`docs/`)

`docs/` contains user-facing documentation: HTTP API references for each service, Google integration details (service account, how to find folder/drive IDs), API key auth, the deprecation header convention, and Google Sheets Apps Script examples. There is also a Yaak API collection (`isha-api.yaak.json`) with ready-to-use request examples.

Keep `docs/` in sync whenever you change routes, request/response shapes, workflow behaviour, or delivery options.
