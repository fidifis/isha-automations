# Video Render API Reference

**WARNING! The unstable APIs will be removed in future. If you experience a failure please check this page if the api was removed!**
Future (stable) APIs will contain a special header to notify you in case of breaking changes.

## /v1

### POST /v1/video-render/reel

*The following API reference is subject to change without notice.*

Starts an asynchronous render job. The HTTP call returns 200 as soon as the workflow is accepted; the actual render runs in the background. On success the response body contains a server-generated `jobId` you can use for log/trace correlation.

#### Request body

- `videoDriveFolderId`
    + ID of the Google Drive folder that contains the source materials. The folder is expected to hold either a `Stems` sub-folder, or a shortcut/folder-link to one (e.g. an `OCD-...` link).
      See [README](./README.md) for how content is selected from stems.
    + Example: `1RSOpu3XrQfJ4NYLsAjR8bAmGVAT_F_e`
- `videoDriveId`
    + ID of the Shared Drive hosting `videoDriveFolderId`.
    + Example: `0AHp6cHlMm1PXUk9PVA`
- `videoFileId`
    + Optional. Drive file ID of a specific video to use instead of the auto-selected one. If omitted, the service picks a video from the Stems folder using the heuristic described in the README.
    + Example: `1abcXYZ...`
- `srtDriveFolderId`
    + ID of the Google Drive folder that contains the translation Google Document.
      The document is identified by the `SUB_` filename prefix and must contain `{{translation_start}}` / `{{translation_end}}` markers around the SRT body.
      See [README](./README.md#srt-extraction).
    + Example: `1t2JH0vmVPGcWk-sA2UCCeW-Uj5hhEZCY`
- `srtDriveId`
    + ID of the Shared Drive hosting `srtDriveFolderId`.
- `destinationFolderId`
    + Drive folder ID where the rendered `OUT_video.mp4` is uploaded.
- `deliveryWorkflow`
    + Required. Selects the result delivery channel. `googleSpreadsheet` or `none`.
- `deliveryParams`
    + Required. **JSON-encoded string** describing where to write the success status. See [Delivery params](#delivery-params) below.
- `errDeliveryParams`
    + Optional. **JSON-encoded string** with the same shape as `deliveryParams` used to report a failure. The string `$errmsg` inside any `value` is replaced by the error message produced by the failing stage. If omitted, errors are not delivered (only visible in CloudWatch / Step Functions).

#### Delivery params

Both `deliveryParams` and `errDeliveryParams` are JSON **strings** (i.e. the JSON object is serialized once before being put into the outer request body). Their decoded shape is:

```json
{
  "sheetId": "1A2B3C...",
  "setValues": [
    { "sheetName": "Videos", "column": 7, "row": 5, "value": "Done $jobid" }
  ]
}
```

- `sheetId` - Google Spreadsheet ID.
- `setValues` - list of cells to update. `column` and `row` are **0-based** (column `0` = A, row `0` = spreadsheet row 1).
- Inside any `value`:
    + `$jobid` is replaced by the server-generated job ID.
    + `$errmsg` (only meaningful in `errDeliveryParams`) is replaced by the error message.

#### Example request

```json
{
  "videoDriveFolderId": "1RSOpu3XrQfJ4NYLsAjR8bAmGVAT_F_e",
  "videoDriveId": "0AHp6cHlMm1PXUk9PVA",
  "srtDriveFolderId": "1abcSrtFolder...",
  "srtDriveId": "0AHp6cHlMm1PXUk9PVA",
  "destinationFolderId": "1t2JH0vmVPGcWk-sA2UCCeW-Uj5hhEZCY",
  "deliveryWorkflow": "googleSpreadsheet",
  "deliveryParams": "{\"sheetId\":\"1A2B3C\",\"setValues\":[{\"sheetName\":\"Videos\",\"column\":7,\"row\":5,\"value\":\"Done $jobid\"}]}",
  "errDeliveryParams": "{\"sheetId\":\"1A2B3C\",\"setValues\":[{\"sheetName\":\"Videos\",\"column\":7,\"row\":5,\"value\":\"Failed: $errmsg\"}]}"
}
```

#### Response

Success (HTTP 200):

```json
{ "jobId": "aB3xZ9qK" }
```

Error (HTTP 400):

```json
{ "error": "...", "execution": "<lambda request id>" }
```

Note that 200 only confirms the workflow was started. Subsequent stage failures (download, ffmpeg, upload, ...) surface through `errDeliveryParams` if configured, or in the Step Functions execution history.

## /unstable/v2

obsolete; undocumented

## /unstable/v1

obsolete; undocumented
