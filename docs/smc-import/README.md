# SMC Import

This service prepares the destination folder for a translation/subtitle workflow. Given a Google Drive *source* folder (with reference materials and an original SRT) and a *destination* folder (where the translated assets will live), the service:

1. Copies all Google Docs and Microsoft Word files from source to destination.
2. Finds the SRT file in the source folder and copies it to destination.
3. Copies a translation document template into destination, names it `SUB_<notation>` and replaces the `{{english body}}` marker inside with the contents of the source SRT.

It replaces the original Google Sheets Apps Script (`runCopyJob`) with a serverless workflow.

The system runs **asynchronously**.
The HTTP request returns immediately (200) once the workflow is started; the actual copying and document edit happen in background. If the destination folder ends up empty or incomplete, the workflow likely failed - check the Step Functions execution or contact the project administrators.

## Notation extraction

The destination folder name is split by `_` and the **second to last** segment is used as the *notation*. The translation document is named `SUB_<notation>`.

Example: `eng_intro_v123_2024_final` → notation `2024` → translation doc `SUB_2024`.

## SRT file selection

The service lists the source folder and picks the SRT file by name:

- A file with both `SRT_` prefix **and** `.srt` suffix is preferred.
- If none match exactly, a file with either `SRT_` prefix **or** `.srt` suffix is used as a fallback.
- If nothing matches, the workflow fails.

## Translation template

A pre-existing Google Doc is used as the translation template. The default template ID is configured at deploy time. You may override it per request via `templateFileId`. The template **must** contain the literal marker `{{english body}}`; it gets replaced by the SRT contents (CRLF line endings normalized to LF).

## Documents copied

The "copy docs" step copies every file in the source folder whose MIME type is one of:

- `application/vnd.google-apps.document` (Google Docs)
- `application/vnd.openxmlformats-officedocument.wordprocessingml.document` (`.docx`)
- `application/msword` (`.doc`)

Other file types are ignored, except for the SRT picked separately.

# API Reference

read at [api.md](./api.md)

**WARNING! The unstable APIs will be removed in future. If you experience a failure please check this page if the api was removed!**
Future (stable) APIs will contain a special header to notify you in case of breaking changes.

# Example

Example of code in Google Sheets Apps Script: [example-apps-script.gs](./example-apps-script.gs)
