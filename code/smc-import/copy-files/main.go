package main

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"

	"google.golang.org/api/drive/v3"

	"lambdalib/clientInit"
	"lambdalib/configRead"
)

var (
	log      *zap.SugaredLogger
	driveSvc *drive.Service
)

type Event struct {
	JobId               string   `json:"jobId"`
	FileIds             []string `json:"fileIds"`
	DestinationFolderId string   `json:"destinationFolderId"`
}

type Result struct {
	JobId          string   `json:"jobId"`
	CopiedFileIds  []string `json:"copiedFileIds"`
}

func main() {
	lambda.Start(HandleRequest)
}
func init() {
	logConfig := zap.NewProductionConfig()
	logConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, _ := logConfig.Build()
	defer logger.Sync()
	log = logger.Sugar()

	ctx := context.Background()

	var err error
	var cfg *aws.Config

	ssmc, _, err := clientInit.InitSSM(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	gcpConfig, err := configRead.GcpConfig(ctx, ssmc)
	if err != nil {
		log.Fatal(err)
	}

	driveSvc, _, err = clientInit.InitGDrive(ctx, clientInit.GInit{ConfigJson: &gcpConfig})
	if err != nil {
		log.Fatal("Error initializig Google service client: ", err)
	}
}

func copyFile(ctx context.Context, fileId string, destFolderId string) (*drive.File, error) {
	copied, err := driveSvc.Files.Copy(fileId, &drive.File{
		Parents: []string{destFolderId},
	}).
		Context(ctx).
		SupportsAllDrives(true).
		Fields("id, name").
		Do()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("Error copying file %s to folder %s", fileId, destFolderId), err)
	}
	return copied, nil
}

func HandleRequest(ctx context.Context, event Event) (Result, error) {
	log.Infof("jobid=%s files=%d dest=%s", event.JobId, len(event.FileIds), event.DestinationFolderId)

	copied := make([]string, 0, len(event.FileIds))
	for _, id := range event.FileIds {
		f, err := copyFile(ctx, id, event.DestinationFolderId)
		if err != nil {
			return Result{}, err
		}
		log.Debugf("copied %s -> %s (%s)", id, f.Id, f.Name)
		copied = append(copied, f.Id)
	}

	return Result{
		JobId:         event.JobId,
		CopiedFileIds: copied,
	}, nil
}
