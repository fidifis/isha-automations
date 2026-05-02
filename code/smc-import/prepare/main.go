package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

const (
	mimeGoogleDoc   = "application/vnd.google-apps.document"
	mimeMsWord      = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	mimeMsWordOld   = "application/msword"
	srtFilePrefix   = "SRT_"
	srtFileSuffix   = ".srt"
	subFilePrefix   = "SUB_"
)

type Event struct {
	JobId               string `json:"jobId"`
	SourceFolderId      string `json:"sourceFolderId"`
	SourceDriveId       string `json:"sourceDriveId"`
	DestinationFolderId string `json:"destinationFolderId"`
	DestinationDriveId  string `json:"destinationDriveId"`
}

type Result struct {
	JobId               string   `json:"jobId"`
	Notation            string   `json:"notation"`
	DocFileIds          []string `json:"docFileIds"`
	SrtFileId           string   `json:"srtFileId"`
	TranslationDocName  string   `json:"translationDocName"`
	DestinationFolderId string   `json:"destinationFolderId"`
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

func listFolder(ctx context.Context, folderId string, driveId string) ([]*drive.File, error) {
	listCall := driveSvc.Files.List().
		Context(ctx).
		Q(fmt.Sprintf("'%s' in parents and trashed = false", folderId)).
		Fields("nextPageToken, files(id, name, mimeType)").
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true)
	if driveId != "" {
		listCall = listCall.Corpora("drive").DriveId(driveId)
	}
	files, err := listCall.Do()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("Error listing folder: %s", folderId), err)
	}
	if files.NextPageToken != "" {
		log.Warn("Next Page Token is present. Pagination is not implemented. This may cause an absence of materials.")
	}
	return files.Files, nil
}

func pickDocs(files []*drive.File) []string {
	var ids []string
	for _, f := range files {
		switch f.MimeType {
		case mimeGoogleDoc, mimeMsWord, mimeMsWordOld:
			ids = append(ids, f.Id)
		}
	}
	return ids
}

func pickSrt(files []*drive.File) string {
	var candidate string
	for _, f := range files {
		hasPrefix := strings.HasPrefix(f.Name, srtFilePrefix)
		hasSuffix := strings.HasSuffix(f.Name, srtFileSuffix)
		if hasPrefix && hasSuffix {
			return f.Id
		}
		if hasPrefix || hasSuffix {
			candidate = f.Id
		}
	}
	return candidate
}

func notationFromName(folderName string) (string, error) {
	parts := strings.Split(folderName, "_")
	if len(parts) < 2 {
		return "", fmt.Errorf("Folder name has unexpected format: %s", folderName)
	}
	return parts[len(parts)-2], nil
}

func getFolderName(ctx context.Context, folderId string) (string, error) {
	f, err := driveSvc.Files.Get(folderId).
		Context(ctx).
		Fields("name").
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return "", errors.Join(fmt.Errorf("Error getting folder metadata: %s", folderId), err)
	}
	return f.Name, nil
}

func HandleRequest(ctx context.Context, event Event) (Result, error) {
	log.Infof("jobid=%s", event.JobId)

	destName, err := getFolderName(ctx, event.DestinationFolderId)
	if err != nil {
		return Result{}, err
	}
	notation, err := notationFromName(destName)
	if err != nil {
		return Result{}, err
	}
	log.Debugf("destination folder=%s notation=%s", destName, notation)

	files, err := listFolder(ctx, event.SourceFolderId, event.SourceDriveId)
	if err != nil {
		return Result{}, err
	}

	docIds := pickDocs(files)
	srtId := pickSrt(files)
	if srtId == "" {
		return Result{}, fmt.Errorf("SRT file not found in source folder: %s", event.SourceFolderId)
	}
	log.Debugf("found docs=%d srt=%s", len(docIds), srtId)

	return Result{
		JobId:               event.JobId,
		Notation:            notation,
		DocFileIds:          docIds,
		SrtFileId:           srtId,
		TranslationDocName:  subFilePrefix + notation,
		DestinationFolderId: event.DestinationFolderId,
	}, nil
}
