package main

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"

	"google.golang.org/api/sheets/v4"

	"lambdalib/apiGwResponse"
	"lambdalib/clientInit"
	"lambdalib/configRead"
)

var (
	log      *zap.SugaredLogger
	sheetSvc *sheets.Service
	sheetCache map[string]map[string]int
)

const (
	batchSize = 1000
)

type Event struct {
	VideoId string `json:"videoId"`
	SheetId string `json:"smcSpreadSheetId"`
	EnSheet string `json:"smcEnSheet"`
	TranslationSheet string `json:"smcLangSheet"`
	ColumnMap ColumnMap `json:"columnMap"`
}

type ColumnMap struct {
	VideoCode       int `json:"videoCode"`
	VideoType         int `json:"videoType"`
	Title           int `json:"title"`
	MediaLink       int `json:"mediaLink"`
	SourceLink      int `json:"sourceLink"`
	DestinationLink int `json:"destinationLink"`
}

type Result struct {
	VideoCode       string `json:"videoCode"`
	VideoType         string `json:"videoType"`
	Title           string `json:"title"`
	MediaLink       string `json:"mediaLink"`
	SourceLink      string `json:"sourceLink"`
	DestinationLink string `json:"destinationLink"`
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

	sheetCache = make(map[string]map[string]int)

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

	sheetSvc, _, err = clientInit.InitGSheet(ctx, clientInit.GInit{ConfigJson: &gcpConfig})
	if err != nil {
		log.Fatal("Error initializing Google service client: ", err)
	}
}

func findRowById(sheetId string, sheet string, videoId string) (int, error) {
    enIdCol := "A"
    offset := 1 // Sheets rows are 1-indexed

    cacheKey := sheetId + "|" + sheet
    if cache, ok := sheetCache[cacheKey]; ok {
        row, found := cache[videoId]
        if found {
	        log.Debugf("video %s found in memory cache")
	        return row, nil
        }
    }

    cache := make(map[string]int)

    for {
        rangeStr := fmt.Sprintf("%s!%s%d:%s%d", sheet, enIdCol, offset, enIdCol, offset+batchSize-1)
        response, err := sheetSvc.Spreadsheets.Values.Get(sheetId, rangeStr).Do()
        if err != nil {
            return -1, err
        }

        values := response.Values
        log.Debugf("fetched %d rows starting at %d", len(values), offset)

        for i, row := range values {
            if len(row) > 0 {
                if cell, ok := row[0].(string); ok {
                    rowIndex := offset + i // zero based index
                    cache[cell] = rowIndex
                    if cell == videoId {
                        sheetCache[cacheKey] = cache
                        log.Debug("Sheet cache partly rebuilt. Video hit.")
                        return rowIndex, nil
                    }
                }
            }
        }

        // Fewer rows returned than requested = end of sheet
        if len(values) < batchSize {
            break
        }

        offset += batchSize
    }

    sheetCache[cacheKey] = cache
    log.Debug("Sheet cache fully rebuilt. Video miss.")

    return -1, fmt.Errorf("video id %s not found in %s %s", videoId, sheet, sheetId)
}

func getStrValue(row []interface{}, i int) string {
	if len(row) <= i {
		return "out_of_bounds"
	}
	val, ok := row[i].(string)
	if !ok {
		return "conversion_err"
	}
	return val
}

func HandleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var request Event
	err := json.Unmarshal([]byte(event.Body), &request)
	if err != nil {
		return apiGwResponse.ErrResponse(err.Error(), ctx)
	}

	videoRow, err := findRowById(request.SheetId, request.EnSheet,request.VideoId)
	if err != nil {
		return apiGwResponse.ErrResponse(err.Error(), ctx)
	}

	valueRange := fmt.Sprintf("%s!%d:%d", request.EnSheet, videoRow, videoRow)
	result, err := sheetSvc.Spreadsheets.Values.Get(request.SheetId, valueRange).Do()
	if err != nil {
		return apiGwResponse.ErrResponse(err.Error(), ctx)
	}

	if len(result.Values) == 0 {
		return apiGwResponse.ErrResponse("GetRange returned 0 values", ctx)
	}

	row := result.Values[0]

	trVideoRow, err := findRowById(request.SheetId, request.TranslationSheet, request.VideoId)
	if err != nil {
		return apiGwResponse.ErrResponse(err.Error(), ctx)
	}
	trValueRange := fmt.Sprintf("%s!%d:%d", request.TranslationSheet, videoRow, trVideoRow)
	trResult, err := sheetSvc.Spreadsheets.Values.Get(request.SheetId, trValueRange).Do()
	if err != nil {
		return apiGwResponse.ErrResponse(err.Error(), ctx)
	}
	trRow := trResult.Values[0]

	final := Result{
		VideoCode: getStrValue(trRow, request.ColumnMap.VideoCode),
		VideoType: getStrValue(row, request.ColumnMap.VideoType),
		Title: getStrValue(row, request.ColumnMap.Title),
		MediaLink: getStrValue(row, request.ColumnMap.MediaLink),
		SourceLink: getStrValue(row, request.ColumnMap.SourceLink),
		DestinationLink: getStrValue(trRow, request.ColumnMap.DestinationLink),
	}

	return apiGwResponse.OkResponse(final)
}
