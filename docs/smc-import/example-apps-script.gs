// **WARNING! The unstable APIs will be removed in future. If you experience a failure please check this page if the api was removed!**
// Future (stable) APIs will contain a speacial header to notify you in case of breaking changes.

const APIKEY = "redacted"; // API key. This key is uniqe for your team and has its usage restrictions. DO NOT SHARE THIS WITH ANYONE!
const API_URL = "api.isha-automations.fidifis.com"; // URL of API to call
const API_PATH = "/v1/smc-import/run"; // API path. If the call fails because the path was removed, check documentation for the newest path.

// Optional. Drive ID for the source folder when it lives on a Shared Drive.
// Leave empty if the source folder is on a personal "My Drive".
// Read documentation on how to get this IDs: https://github.com/Fidifis/isha-automations/blob/main/docs/README.md
const SOURCE_DRIVE_ID = "0AHp6cHlMm1PXUk9PVA";

// Optional. Override the default translation document template.
// Leave empty to use the deploy-time default.
const TEMPLATE_FILE_ID = "";


/**
 * Run the SMC import for the active row.
 * Expects two adjacent cells with Google Drive folder URLs:
 *   - one of them points to the SOURCE folder (English materials + SRT)
 *   - the other points to the DESTINATION folder (where copies + translation doc go)
 * Either order works: the active cell is treated as one side, the cell to its right
 * is checked first; if not a link, the cell to the left is used as source instead.
 */
function runSmcImport() {
  const sheet = SpreadsheetApp.getActiveSpreadsheet().getActiveSheet();
  const activeCell = sheet.getActiveCell();
  const activeIdx = cellA1ToIndex(activeCell.getA1Notation(), 1);

  // Resolve source and destination cells.
  let sourceCell = activeCell;
  let destinationCell = sheet.getRange(activeIdx.row, activeIdx.col + 1);

  if (!isLink(sourceCell.getValue())) {
    throw Error("No link in active cell");
  }
  if (!isLink(destinationCell.getValue())) {
    // Fall back: active cell is the destination, source is to the left.
    destinationCell = activeCell;
    sourceCell = sheet.getRange(activeIdx.row, activeIdx.col - 1);
    if (!isLink(sourceCell.getValue())) {
      throw Error("No link in adjacent cell");
    }
  }

  const sourceFolderId = getIdFromUrl(sourceCell.getValue());
  const destinationFolderId = getIdFromUrl(destinationCell.getValue());

  // Build payload. See https://github.com/Fidifis/isha-automations/blob/main/docs/smc-import/api.md
  const payload = {
    sourceFolderId: sourceFolderId,
    destinationFolderId: destinationFolderId,
  };
  if (SOURCE_DRIVE_ID) {
    payload.sourceDriveId = SOURCE_DRIVE_ID;
  }
  if (TEMPLATE_FILE_ID) {
    payload.templateFileId = TEMPLATE_FILE_ID;
  }

  const options = {
    method: "post",
    contentType: "application/json",
    headers: {
      "x-api-key": APIKEY,
    },
    payload: JSON.stringify(payload),
    muteHttpExceptions: true,
  };

  const response = UrlFetchApp.fetch(`https://${API_URL}${API_PATH}`, options);
  const status = response.getResponseCode();

  if (status !== 200) {
    throw Error("Got response status code " + status + " body: " + response.getContentText());
  }

  // Server-generated job ID for log/trace correlation.
  const body = JSON.parse(response.getContentText());
  Logger.log("SMC import started, jobId=" + body.jobId);

  deprecationAlert(response);
}


/**
 * @param {string} text text to check
 * @returns {boolean} true if it is a Google Drive link
 */
function isLink(text) {
  return text && text.toString().startsWith("https://");
}

/**
 * Extract the Drive folder/file ID from a sharing URL.
 * @param {string} url
 * @returns {string}
 */
function getIdFromUrl(url) {
  const sp = url.split("/");
  return sp[sp.length - 1].split("?")[0];
}

/**
 * Check if API returned deprecation flag. If true, you should migrate to new API, as the old one will eventually shut down.
 * @param {UrlFetchApp.HTTPResponse} response The response from UrlFetchApp
 */
function deprecationAlert(response) {
  const headers = response.getHeaders();
  const deprecationMessage = headers['x-deprecated-version'];

  if (deprecationMessage !== undefined) {
    SpreadsheetApp.getUi().alert("The API calls you are currently using are deprecated! Please migrate to new version. Deprecation message: " + deprecationMessage);
  }
}
