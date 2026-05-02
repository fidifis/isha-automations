package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.uber.org/zap"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"

	"lambdalib/clientInit"
	"lambdalib/configRead"
)

var (
	log      *zap.SugaredLogger
	driveSvc *drive.Service
	docsSvc  *docs.Service
)

const englishBodyMarker = "{{english body}}"

type Event struct {
	JobId               string `json:"jobId"`
	TemplateFileId      string `json:"templateFileId"`
	SrtFileId           string `json:"srtFileId"`
	DestinationFolderId string `json:"destinationFolderId"`
	TranslationDocName  string `json:"translationDocName"`
}

type Result struct {
	JobId             string `json:"jobId"`
	TranslationFileId string `json:"translationFileId"`
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
		log.Fatal("Error initializig Drive client: ", err)
	}
	docsSvc, _, err = clientInit.InitGDoc(ctx, clientInit.GInit{ConfigJson: &gcpConfig})
	if err != nil {
		log.Fatal("Error initializig Docs client: ", err)
	}
}

func copyTemplate(ctx context.Context, templateId string, destFolderId string, name string) (string, error) {
	copied, err := driveSvc.Files.Copy(templateId, &drive.File{
		Name:    name,
		Parents: []string{destFolderId},
	}).
		Context(ctx).
		SupportsAllDrives(true).
		Fields("id").
		Do()
	if err != nil {
		return "", errors.Join(fmt.Errorf("Error copying template %s to folder %s", templateId, destFolderId), err)
	}
	return copied.Id, nil
}

func downloadSrt(ctx context.Context, fileId string) (string, error) {
	resp, err := driveSvc.Files.Get(fileId).Context(ctx).SupportsAllDrives(true).Download()
	if err != nil {
		return "", errors.Join(fmt.Errorf("Unable to download srt file: %s", fileId), err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Join(fmt.Errorf("Error reading srt file body: %s", fileId), err)
	}
	return strings.ReplaceAll(string(data), "\r", ""), nil
}

func replaceMarker(ctx context.Context, docId string, marker string, content string) error {
	req := &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{
			{
				ReplaceAllText: &docs.ReplaceAllTextRequest{
					ContainsText: &docs.SubstringMatchCriteria{
						Text:      marker,
						MatchCase: true,
					},
					ReplaceText: content,
				},
			},
		},
	}
	_, err := docsSvc.Documents.BatchUpdate(docId, req).Context(ctx).Do()
	if err != nil {
		return errors.Join(fmt.Errorf("Error updating document %s", docId), err)
	}
	return nil
}

func HandleRequest(ctx context.Context, event Event) (Result, error) {
	log.Infof("jobid=%s template=%s srt=%s name=%s", event.JobId, event.TemplateFileId, event.SrtFileId, event.TranslationDocName)

	if event.TemplateFileId == "" {
		return Result{}, errors.New("templateFileId is required")
	}

	srt, err := downloadSrt(ctx, event.SrtFileId)
	if err != nil {
		return Result{}, err
	}

	docId, err := copyTemplate(ctx, event.TemplateFileId, event.DestinationFolderId, event.TranslationDocName)
	if err != nil {
		return Result{}, err
	}
	log.Debugf("template copied as %s", docId)

	if err := replaceMarker(ctx, docId, englishBodyMarker, srt); err != nil {
		return Result{}, err
	}

	return Result{
		JobId:             event.JobId,
		TranslationFileId: docId,
	}, nil
}
