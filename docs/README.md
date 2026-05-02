# Documentation

## Services

- [Daily Mystic Quotes (DMQ)](./dmq/README.md)
- [Video Render](./video-render/README.md)
- [SMC Import](./smc-import/README.md)

## Google integration

As Isha uses Google Drive as a content storage, most of (or rather all) services needs access to your Google Drive.

**To allow the systems to access your Google Services (like Drive) please give permissions to this account (email is below).** Do it like for any other team member and it should work fine.

Google Cloud Platform service account is used for accessing google. Under GCP project: "isha-automations-231309".

It has a federated trust policy, between the GCP project and production AWS account. No secrets.

The GCP service account:

- **email: awscloud@isha-automations-231309.iam.gserviceaccount.com**
- name: AWS
- id: 112086028889105414929

### Google folder IDs

To find a Google folder ID, open the folder in your web browser. Look at the URL—it will look something like this:
`https://drive.google.com/drive/folders/1RSOpu3XrQfJ4NYLsAjR8bAmGVAT_F_e`

The folder ID is the long string of letters, numbers, dashes, or underscores at the end of the URL.
In this example, the folder ID is:
`1RSOpu3XrQfJ4NYLsAjR8bAmGVAT_F_e`

### Google Drive ID

A Drive ID refers to the unique identifier of a Shared Drive - a type of Google Drive managed by an organization (Isha).

To find the Drive ID:

- Open a folder that is part of a Shared Drive.
- Navigate up through the folder structure until you reach the root (top-level) of the Shared Drive.
- At the top, you’ll see the name of the Shared Drive - this indicates you're in the right place.
- Copy the random string at the end of the URL. This is the Drive ID.

Shared Drive IDs are typically shorter than regular folder IDs, usually around 19 characters.

## API Authorization

To be able to make any API call you need to auth.
Each team gets an API key bound to a usage plan (rate limit + daily quota).
Provide it in the `x-api-key` header of every request.

```json
{
  "headers": {
    "x-api-key": "abcd123"
  }
}
```

The API runs on AWS API Gateway (REST) with API keys and usage plans. Keys are issued per team (e.g. `gr-cz`, `gr-demo`); contact the project administrators to get one.

## Deprecation header

Routes that have been superseded or are scheduled for removal include the
`x-deprecated-version` response header on every call (both 2xx and error
responses). Its value is a short human-readable message explaining the
deprecation - typically "use path X instead" or a removal date.

If your client sees this header, plan a migration: the old path will eventually
stop working. Stable (non-deprecated) routes do **not** emit this header, so
checking for its presence is a safe deprecation signal.

The Apps Script examples in this repo show one way of surfacing it:

```js
function deprecationAlert(response) {
  const headers = response.getHeaders();
  const deprecationMessage = headers['x-deprecated-version'];
  if (deprecationMessage !== undefined) {
    SpreadsheetApp.getUi().alert(
      "The API call you are using is deprecated. Migrate to the new version. Message: " + deprecationMessage,
    );
  }
}
```

## Fonts

This table tracks what fonts are used for what purpose

| Font              | S3 Key                | Purpose      |
| ----------------- | --------------------- | ------------ |
| Open Sans (bold)  | open_sans_bold.ttf    | Video render |
| Merriweather Sans | merriweather_sans.ttf | DMQ fallback |
