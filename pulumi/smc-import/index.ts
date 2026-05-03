import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import { ApiGatewayRoute } from "../components/apiGateway";
import { Arch, GoLambda, HashFolder, AssumePolicy } from "../components/lambda";
import { MetaProps } from "../utils";

export interface SmcImportProps {
  meta: MetaProps;
  gcpConfigParam: aws.ssm.Parameter;
  sparkLambda: GoLambda;
  sparkApiGwExec: aws.iam.Role;
  sfnExec: aws.iam.Role;
  defaultTemplateFileId?: pulumi.Input<string>;
}

const FALLBACK_TEMPLATE_FILE_ID = "14VBEMHt03JNWXS8nKrg31JfpGdsiQwwl5mMIrFFjht0";

export default class SmcImport extends pulumi.ComponentResource {
  public readonly routes: ApiGatewayRoute[];

  constructor(
    name: string,
    args: SmcImportProps,
    opts?: pulumi.ComponentResourceOptions,
  ) {
    super("project:components:smc-import", name, {}, opts);

    const xray = true;
    const defaultTemplateFileId =
      args.defaultTemplateFileId ?? FALLBACK_TEMPLATE_FILE_ID;

    const smcRole = new aws.iam.Role(
      `${name}-Role`,
      {
        tags: args.meta.tags,
        assumeRolePolicy: AssumePolicy.json,
        inlinePolicies: [
          {
            name: "Logging",
            policy: aws.iam.getPolicyDocumentOutput(
              {
                statements: [
                  {
                    actions: [
                      "xray:PutTelemetryRecords",
                      "xray:PutTraceSegments",
                    ],
                    resources: ["*"],
                  },
                  {
                    actions: ["logs:CreateLogStream", "logs:PutLogEvents"],
                    resources: [
                      pulumi.interpolate`arn:aws:logs:${args.meta.region}:${args.meta.accountId}:log-group:/aws/lambda/${pulumi.getProject()}-${pulumi.getStack()}-${name}*`,
                    ],
                  },
                ],
              },
              { parent: this },
            ).json,
          },
          {
            name: "WorkPermissions",
            policy: aws.iam.getPolicyDocumentOutput(
              {
                statements: [
                  {
                    actions: ["ssm:GetParameter"],
                    resources: [args.gcpConfigParam.arn],
                  },
                ],
              },
              { parent: this },
            ).json,
          },
        ],
      },
      { parent: this },
    );
    const proxy = new GoLambda(
      `${name}-Proxy`,
      {
        tags: args.meta.tags,
        source: {
          code: "../bin/smc-import-smc-proxy.zip",
          hash: HashFolder("../code/smc-import/smc-proxy/"),
        },
        xray,
        role: smcRole,
        architecture: Arch.arm,
        timeout: 60,
        memory: 128,
        logs: { retention: 30 },
        env: {
          variables: {
            SSM_GCP_CONFIG: args.gcpConfigParam.name,
          },
        },
      },
      { parent: this },
    );
    const apiGwLambdaSMCProxyRole = new aws.iam.Role(
      `${name}-SMCProxyApiGwExec`,
      {
        tags: args.meta.tags,
        assumeRolePolicy: aws.iam.getPolicyDocumentOutput(
          {
            statements: [
              {
                effect: "Allow",
                principals: [
                  {
                    type: "Service",
                    identifiers: ["apigateway.amazonaws.com"],
                  },
                ],
                actions: ["sts:AssumeRole"],
              },
            ],
          },
          { parent: this },
        ).json,
        inlinePolicies: [ {
          policy: aws.iam.getPolicyDocumentOutput({
            statements: [{
              actions: [
                "lambda:InvokeFunction",
              ],
              resources: [proxy.lambda.arn],
            }]},
            { parent: this },
          ).json,
        }]},
      { parent: this },
    );

    const lambdaPrepare = new GoLambda(
      `${name}-Prepare`,
      {
        tags: args.meta.tags,
        source: {
          code: "../bin/smc-import-prepare.zip",
          hash: HashFolder("../code/smc-import/prepare/"),
        },
        xray,
        role: smcRole,
        architecture: Arch.arm,
        timeout: 60,
        memory: 128,
        logs: { retention: 30 },
        env: {
          variables: {
            SSM_GCP_CONFIG: args.gcpConfigParam.name,
          },
        },
      },
      { parent: this },
    );

    const lambdaCopyFiles = new GoLambda(
      `${name}-CopyFiles`,
      {
        tags: args.meta.tags,
        source: {
          code: "../bin/smc-import-copy-files.zip",
          hash: HashFolder("../code/smc-import/copy-files/"),
        },
        xray,
        role: smcRole,
        architecture: Arch.arm,
        timeout: 300,
        memory: 128,
        logs: { retention: 30 },
        env: {
          variables: {
            SSM_GCP_CONFIG: args.gcpConfigParam.name,
          },
        },
      },
      { parent: this },
    );

    const lambdaMakeTranslation = new GoLambda(
      `${name}-MakeTranslation`,
      {
        tags: args.meta.tags,
        source: {
          code: "../bin/smc-import-make-translation.zip",
          hash: HashFolder("../code/smc-import/make-translation/"),
        },
        xray,
        role: smcRole,
        architecture: Arch.arm,
        timeout: 120,
        memory: 256,
        logs: { retention: 30 },
        env: {
          variables: {
            SSM_GCP_CONFIG: args.gcpConfigParam.name,
          },
        },
      },
      { parent: this },
    );

    const stateMachine = new aws.sfn.StateMachine(
      `${name}`,
      {
        tags: args.meta.tags,
        roleArn: args.sfnExec.arn,
        tracingConfiguration: {
          enabled: xray,
        },
        definition: pulumi.jsonStringify({
          Comment: "SMC import: copy docs, srt and build translation document",
          StartAt: "Assign vars",
          QueryLanguage: "JSONata",
          States: {
            "Assign vars": {
              Type: "Pass",
              Assign: {
                jobId: "{% $states.input.jobId %}",
                sourceFolderId: "{% $states.input.sourceFolderId %}",
                sourceDriveId:
                  "{% $exists($states.input.sourceDriveId) ? $states.input.sourceDriveId : '' %}",
                destinationFolderId: "{% $states.input.destinationFolderId %}",
                templateFileId: pulumi.interpolate`{% $exists($states.input.templateFileId) ? $states.input.templateFileId : '${defaultTemplateFileId}' %}`,
              },
              Next: "Prepare",
            },
            Prepare: {
              Type: "Task",
              Resource: "arn:aws:states:::lambda:invoke",
              Output: "{% $states.result.Payload %}",
              Arguments: {
                FunctionName: pulumi.interpolate`${lambdaPrepare.lambda.arn}:$LATEST`,
                Payload: {
                  jobId: "{% $jobId %}",
                  sourceFolderId: "{% $sourceFolderId %}",
                  sourceDriveId: "{% $sourceDriveId %}",
                  destinationFolderId: "{% $destinationFolderId %}",
                },
              },
              Assign: {
                notation: "{% $states.result.Payload.notation %}",
                docFileIds: "{% $states.result.Payload.docFileIds %}",
                srtFileId: "{% $states.result.Payload.srtFileId %}",
                translationDocName:
                  "{% $states.result.Payload.translationDocName %}",
              },
              Retry: [
                {
                  ErrorEquals: ["Lambda.TooManyRequestsException"],
                  IntervalSeconds: 1,
                  MaxAttempts: 3,
                  BackoffRate: 2,
                  JitterStrategy: "FULL",
                },
              ],
              Catch: [
                {
                  ErrorEquals: ["States.ALL"],
                  Next: "Fail",
                  Output: {
                    err: "{% $states.errorOutput.Cause %}",
                  },
                },
              ],
              Next: "Copy Docs",
            },
            "Copy Docs": {
              Type: "Task",
              Resource: "arn:aws:states:::lambda:invoke",
              Output: "{% $states.result.Payload %}",
              Arguments: {
                FunctionName: pulumi.interpolate`${lambdaCopyFiles.lambda.arn}:$LATEST`,
                Payload: {
                  jobId: "{% $jobId %}",
                  fileIds: "{% $docFileIds %}",
                  destinationFolderId: "{% $destinationFolderId %}",
                },
              },
              Retry: [
                {
                  ErrorEquals: ["Lambda.TooManyRequestsException"],
                  IntervalSeconds: 1,
                  MaxAttempts: 3,
                  BackoffRate: 2,
                  JitterStrategy: "FULL",
                },
              ],
              Catch: [
                {
                  ErrorEquals: ["States.ALL"],
                  Next: "Fail",
                  Output: {
                    err: "{% $states.errorOutput.Cause %}",
                  },
                },
              ],
              Next: "Copy SRT",
            },
            "Copy SRT": {
              Type: "Task",
              Resource: "arn:aws:states:::lambda:invoke",
              Output: "{% $states.result.Payload %}",
              Arguments: {
                FunctionName: pulumi.interpolate`${lambdaCopyFiles.lambda.arn}:$LATEST`,
                Payload: {
                  jobId: "{% $jobId %}",
                  fileIds: "{% [$srtFileId] %}",
                  destinationFolderId: "{% $destinationFolderId %}",
                },
              },
              Retry: [
                {
                  ErrorEquals: ["Lambda.TooManyRequestsException"],
                  IntervalSeconds: 1,
                  MaxAttempts: 3,
                  BackoffRate: 2,
                  JitterStrategy: "FULL",
                },
              ],
              Catch: [
                {
                  ErrorEquals: ["States.ALL"],
                  Next: "Fail",
                  Output: {
                    err: "{% $states.errorOutput.Cause %}",
                  },
                },
              ],
              Next: "Make Translation",
            },
            "Make Translation": {
              Type: "Task",
              Resource: "arn:aws:states:::lambda:invoke",
              Output: "{% $states.result.Payload %}",
              Arguments: {
                FunctionName: pulumi.interpolate`${lambdaMakeTranslation.lambda.arn}:$LATEST`,
                Payload: {
                  jobId: "{% $jobId %}",
                  templateFileId: "{% $templateFileId %}",
                  srtFileId: "{% $srtFileId %}",
                  destinationFolderId: "{% $destinationFolderId %}",
                  translationDocName: "{% $translationDocName %}",
                },
              },
              Retry: [
                {
                  ErrorEquals: ["Lambda.TooManyRequestsException"],
                  IntervalSeconds: 1,
                  MaxAttempts: 3,
                  BackoffRate: 2,
                  JitterStrategy: "FULL",
                },
              ],
              Catch: [
                {
                  ErrorEquals: ["States.ALL"],
                  Next: "Fail",
                  Output: {
                    err: "{% $states.errorOutput.Cause %}",
                  },
                },
              ],
              End: true,
            },
            Fail: {
              Type: "Fail",
              Error: "error",
              Cause: "a stage has failed",
            },
          },
        }),
      },
      { parent: this },
    );

    this.routes = [
      {
        path: "/v1/smc-import/run",
        method: "POST",
        eventHandler: args.sparkLambda.lambda,
        execRole: args.sparkApiGwExec,
        requestTemplate: {
          "application/json": pulumi.jsonStringify({
            input: "$util.escapeJavaScript($input.json('$'))",
            stateMachineArn: stateMachine.arn,
            traceHeader: "$method.request.header.X-Amzn-Trace-Id",
            apiKeyId: "$context.identity.apiKeyId",
          }),
        },
      },
      {
        path: "/v1/smc-import/proxy-fetch",
        method: "GET",
        eventHandler: proxy.lambda,
        execRole: apiGwLambdaSMCProxyRole,
      },
    ];

    const sparkPolicy = new aws.iam.Policy(
      `${name}-SparkPolicy`,
      {
        policy: aws.iam.getPolicyDocumentOutput(
          {
            statements: [
              {
                actions: ["states:StartExecution", "states:StartSyncExecution"],
                resources: [stateMachine.arn],
              },
            ],
          },
          { parent: this },
        ).json,
      },
      { parent: this },
    );

    new aws.iam.PolicyAttachment(
      `${name}-SparkPolicy`,
      {
        roles: [args.sparkLambda.role],
        policyArn: sparkPolicy.arn,
      },
      { parent: this },
    );

    this.registerOutputs({
      routes: this.routes,
    });
  }
}
