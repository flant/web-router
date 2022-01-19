# Web Version Router (v-router)

**STAGE: Development**

Go HTTP-server for software documentation websites. 

Adds the ability to have several documentation versions on the site: generates page partials for a menu, routes traffic, etc.

## Channel names

The following channel names used (from less stable to more stable): Alpha, Beta, EarlyAccess, Stable, RockSolid.

The `VROUTER_USE_LATEST_CHANNEL` env adds  `latest` channel the the channels list.

## Configuration
web-router uses the following environment variables:
- `VROUTER_PATH_CHANNELS_FILE` — file in [appropriate format](#channels-file-format) containing information about versions and channels  
- `VROUTER_PATH_STATIC` — path for static files to serve
- `VROUTER_PATH_TPLS` — directory inside the `VROUTER_PATHSTATIC`, where templates resides. It is also a URL-location. Default — `/includes`. 
- `VROUTER_LOG_FORMAT` — Log format to use (json|text|color). Default — text.
- `VROUTER_LOG_LEVEL` — Logging level (`info`, `debug`, `trace`)
- `VROUTER_LISTEN_PORT` —  IP port to listen on (default - '8080')
- `VROUTER_LISTEN_ADDRESS` — IP ddress to listen on (default - '0.0.0.0')
- `VROUTER_LOCATION_VERSIONS` —  URL-location where versions will be accessed (default - `/documentation`).
- `VROUTER_DEFAULT_GROUP` —  The default group name according to the used channel file. E.g. - "v1" or "1" (the leading 'v' can be ommited).
- `VROUTER_DEFAULT_CHANNEL` —  The default channel name. E.g. - "stable".
- `VROUTER_SHOW_LATEST_CHANNEL` —  Whether to show the 'latest' channel in the menu (default - `false`).
- `VROUTER_URL_VALIDATION` — Whether to use URL checking before redirect (use false on test environments or protected with authentication).
- `VROUTER_I18N_TYPE` — Localization method. Can be `domain` or `location` (default - `location`).
  - `location` - Versioned pages URL is like `/<LANGUAGE><VROUTER_LOCATIONVERSIONS>/`. E.g `/en/documentation/`.
  - `domain` - Versioned pages URL is like `<LANGUAGE>.somedomain/<VROUTER_LOCATIONVERSIONS>/`. E.g `ru.product.my/documentation/`.

### Templates

All the templates should be placed in the `/includes`

### Channels file format

A file, containing information about which version is assigned to which channel, is the channel file. It can be YAML or JSON formatted.

Specify a path to the channels file in the `VROUTER_PATHCHANNELSFILE` environment variable. The default path to the channels file is 'channels.yaml' (relative to the directory where web-router starts).

YAML Example:
```yaml 
groups:
 - name: "1.1"
   channels:
    - name: alpha
      version: 1.1.23+fix50
    - name: beta
      version: 1.1.23+fix25
    - name: ea
      version: 1.1.22+fix40
    - name: stable
      version: 1.1.21+fix40
    - name: rock-solid
      version: 1.1.21
 - name: "1.2"
   channels:
    - name: alpha
      version: 1.2.34 # Feature X was implemented
    - name: beta
      version: 1.2.33 # Feature Y was implemented
    - name: ea
      version: 1.2.27+fix3
```

JSON example:
```json
{
  "groups": [
    {
      "name": "1.1",
      "channels": [
        {
          "name": "alpha",
          "version": "1.1.23+fix50"
        },
        {
          "name": "beta",
          "version": "1.1.23+fix25"
        },
        {
          "name": "ea",
          "version": "1.1.22+fix40"
        },
        {
          "name": "stable",
          "version": "1.1.21+fix40"
        },
        {
          "name": "rock-solid",
          "version": "1.1.21"
        }
      ]
    },
    {
      "name": "1.2",
      "channels": [
        {
          "name": "alpha",
          "version": "1.2.34"
        },
        {
          "name": "beta",
          "version": "1.2.33"
        },
        {
          "name": "ea",
          "version": "1.2.27+fix3"
        }
      ]
    }
  ]
}
```

## Healthchecks, probes and status information

- `/health` — normal response is JSON: `{"status": "ok"}`
- `/status` — retrieves content of a [channel file](#channels-file-format) used

## How to debug

Compile:
```
go build -gcflags "all=-N -l" -v -o server ./cmd/web-router
```

Run:
```
dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./server
```

Connect to localhost:2345
