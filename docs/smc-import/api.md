# SMC Import API Reference

**WARNING! The unstable APIs will be removed in future. If you experience a failure please check this page if the api was removed!**
Future (stable) APIs will contain a special header to notify you in case of breaking changes.

## /v1

### POST /v1/smc-import/run

*The following API reference is subject to change without notice.*

Starts an asynchronous import workflow. Returns 200 as soon as the state machine execution is accepted; the response body contains a server-generated `jobId` for log/trace correlation.

#### Request body

- `sourceFolderId`
    + Google Drive folder ID containing the original (English) materials: docs, Word files and the SRT.
    + Example: `1RSOpu3XrQfJ4NYLsAjR8bAmGVAT_F_e`
- `sourceDriveId`
    + Optional. Shared Drive ID hosting `sourceFolderId`. Required only if the source folder lives in a Shared Drive.
    + Example: `0AHp6cHlMm1PXUk9PVA`
- `destinationFolderId`
    + Google Drive folder ID where the copies and translation document are placed.
      The folder name is parsed for the *notation* used in the translation document name (split by `_`, second-to-last segment).
    + Example: `1t2JH0vmVPGcWk-sA2UCCeW-Uj5hhEZCY`
- `destinationDriveId`
    + Optional. Reserved for future use; currently the destination is accessed via the folder ID alone.
- `templateFileId`
    + Optional. Google Doc ID of the translation template. The document must contain the marker `{{english body}}` which is replaced by the SRT contents.
    + Defaults to the deploy-time configured template if omitted.
    + Example: `14VBEMHt03JNWXS8nKrg31JfpGdsiQwwl5mMIrFFjht0`

#### Example

```json
{
  "sourceFolderId": "1RSOpu3XrQfJ4NYLsAjR8bAmGVAT_F_e",
  "sourceDriveId": "0AHp6cHlMm1PXUk9PVA",
  "destinationFolderId": "1t2JH0vmVPGcWk-sA2UCCeW-Uj5hhEZCY"
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

#### Side effects (in destination folder)

- One copy of every Google Doc / `.doc` / `.docx` file from the source folder.
- One copy of the source SRT file (original name kept).
- One Google Doc named `SUB_<notation>` based on the configured template, with `{{english body}}` replaced by the SRT body.
