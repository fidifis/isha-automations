# SMC Proxy

Lambda function (API Gateway trigger) that looks up a single video's metadata from a Google Sheets spreadsheet and returns a structured record. It is designed to be called by upstream automation steps that need to hydrate a video ID into its full catalog row before further processing.

## How it works

1. The caller sends a video ID plus the IDs and sheet names of the spreadsheet.
2. The proxy scans column A of the **English sheet** in batches of 1000 rows to locate the row index for that video ID.
3. It fetches the full row from the English sheet.
4. It repeats the scan on the **translation sheet** for the same video ID.
5. It fetches the full row from the translation sheet.
6. It extracts fields from both rows using the caller-supplied column indices and returns a single `Result` object.

### In-memory cache

Column A of each sheet is cached in the Lambda process memory after the first scan. Subsequent invocations on a warm Lambda instance skip the batch scan and resolve the row index directly from the cache. The cache is keyed by `<sheetId>|<sheetName>`.

## API

### POST /v1/smc-proxy/run

#### Request body

- `videoId`
    + The video identifier to look up. Must appear in column A of both the English and translation sheets.
    + Example: `"ifPmv4t8dkw"`
- `smcSpreadSheetId`
    + Google Sheets spreadsheet ID containing both the English and translation sheets.
    + Example: `"1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2upms"`
- `smcEnSheet`
    + Name of the sheet tab holding the English (source) rows.
    + Example: `"English"`
- `smcLangSheet`
    + Name of the sheet tab holding the translation rows.
    + Example: `"cze"`
- `columnMap`
    + Zero-based column indices that tell the proxy where to read each field. All indices are required.

    | Field             | Source sheet  | Description                        |
    |-------------------|---------------|------------------------------------|
    | `videoCode`       | Translation   | Unique video code / identifier     |
    | `videoType`       | English       | Category or type of the video      |
    | `title`           | English       | Display title                      |
    | `mediaLink`       | English       | Link to the media asset            |
    | `sourceLink`      | English       | Link to the source material        |
    | `destinationLink` | Translation   | Link to the translated destination |

#### Example request

```json
{
	"videoId": "ifPmv4t8dkw",
	"smcSpreadSheetId": "1BGxTfnvs3zezyJVTSXroy9N0l7j5QHbzPzRj_TSjO-c",
	"smcEnSheet": "English",
	"smcLangSheet": "cze",
	"columnMap": {
		"videoCode": 9,
		"videoType": 7,
		"title": 8,
		"mediaLink": 1,
		"sourceLink": 21,
		"destinationLink": 9
	}
}
```

#### Response

Success (HTTP 200):

```json
{
  "videoCode": "ifPmv4t8dkw",
  "videoType": "Video\n(3:20)",
  "title": "Example Title",
  "mediaLink": "https://...",
  "sourceLink": "https://...",
  "destinationLink": "https://..."
}
```

Common error cases:
- Video ID not found in either sheet.
- Column index out of bounds for a row (returns the sentinel string `"out_of_bounds"`).
- Failed Google Sheets API call (network error or permission issue).
