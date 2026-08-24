# Grafana MongoDB Data Source plugin

This plugin allows you to connect to a MongoDB database and visualize the data in Grafana.

Requires Grafana 11.0.0 or newer.

## Installation

If you are using the official Grafana Docker image, you can install this plugin by configuring the following environment variables:

- `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS`: `ludovicm67-mongodb-datasource`
- `GF_INSTALL_PLUGINS`: `${PATH_TO_ZIP_ARCHIVE};ludovicm67-mongodb-datasource`

where `${PATH_TO_ZIP_ARCHIVE}` is the path to the zip archive of the plugin.

You can browse the [latest releases of the plugin](https://github.com/ludovicm67/grafana-mongodb/releases) to find the link to the zip archive.

## Usage

Configure the datasource with the URI of your MongoDB instance (for example `mongodb://localhost:27017`),
and optionally a username and a password.

Every query targets a **Database** and a **Collection**. Both are required, and the query is only
sent once they are filled in. Until then the editor marks them as invalid.

The **Database**, **Collection**, **Timestamp Field** and **Field** inputs suggest what exists on
the instance: the databases, then the collections of the selected database, then the field names
found in a sample of its documents. The suggestions are only a convenience, and any value can still
be typed, which matters when a collection does not exist yet, when the user is not allowed to list
them, or when a dashboard variable is used. If listing fails, the reason is shown in the dropdown
and the input keeps working.

Filters and pipelines are written as
[extended JSON](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/), which is
regular JSON plus the MongoDB wrappers. Query operators (`$gt`, `$and`, `$regex`, …) and the
`$oid` / `$date` wrappers are supported, and `//` and `/* … */` comments are stripped before the
query is sent.

### Query types

**Find** returns the documents matching a filter. It has four extra options:

| Option     | Example                   | Effect                                                         |
| ---------- | ------------------------- | -------------------------------------------------------------- |
| Projection | `{ "name": 1, "_id": 0 }` | Which fields to return                                         |
| Sort       | `{ "timestamp": -1 }`     | How to order the documents. The order of the keys is preserved |
| Limit      | `100`                     | Maximum number of documents. Empty or `0` means no limit       |
| Skip       | `20`                      | How many documents to skip first                               |

**Aggregate** runs an [aggregation pipeline](https://www.mongodb.com/docs/manual/aggregation/),
written as an array of stages:

```json
[
  { "$match": { "level": "error" } },
  { "$group": { "_id": "$service", "total": { "$sum": 1 } } },
  { "$sort": { "total": -1 } }
]
```

**Count** returns how many documents match a filter, as a single value. Useful for a stat panel.

**Distinct** returns the unique values of a field, optionally restricted by a filter.

### Time range

**Timestamp Field** is optional on every query type. It names a field holding either a UNIX
timestamp in milliseconds or a BSON date, and restricts the query to the dashboard time range.

- For **Find** and **Aggregate**, the returned documents are filtered, and the field is exposed
  as a real time field so it can be used on a time axis.
- For **Count** and **Distinct** there is no document list to filter, so the range is pushed into
  the query itself. Both the numeric and the date representation are matched, so it works whichever
  way the documents store their date.

Every field other than the timestamp is returned as text.

## Development

### Requirements

- Node.js, the version is pinned in [`.nvmrc`](./.nvmrc)
- Go, the version is pinned in [`go.mod`](./go.mod)
- [Mage](https://magefile.org/)
- Docker, with the Compose plugin

### Getting started

```sh
npm install          # install the frontend dependencies
npm run dev          # build the frontend and watch for changes
mage -v              # build the backend, rerun after every backend change
npm run server       # start Grafana and MongoDB in the background
```

Grafana is then available on <http://localhost:3000>, with:

- the MongoDB datasource already provisioned,
- an example dashboard, "MongoDB example",
- a MongoDB instance seeded from [`tests/fixtures/mongodb-init.js`](./tests/fixtures/mongodb-init.js)
  with a `grafana` database holding a `fruits` and a `logs` collection.

Useful companion commands:

```sh
npm run server:logs  # follow the Grafana and MongoDB logs
npm run server:down  # stop everything and drop the seeded data
```

The `logs` fixtures are dated relative to the moment the MongoDB container starts, and the seed
script only runs on a fresh container. A stack left running for hours therefore ends up with
documents that fall outside the default dashboard time range. `npm run server:down && npm run server`
seeds them again.

Grafana has to be restarted after a backend rebuild, so that it picks up the new binary:

```sh
mage -v && docker compose restart grafana
```

The host ports can be changed through `GRAFANA_PORT`, `GRAFANA_DEBUG_PORT` and `MONGODB_PORT`,
which is handy when another stack is already using them:

```sh
GRAFANA_PORT=3001 npm run server
```

The Grafana and MongoDB versions can be pinned through `GRAFANA_VERSION` and `MONGODB_VERSION`.

### Tests

```sh
npm run test:ci      # frontend unit tests
npm run typecheck    # frontend and end to end type checking
npm run lint         # frontend linting
go test ./pkg/...    # backend unit tests
```

The backend also has integration tests that run against a real MongoDB. They are skipped
unless `MONGODB_URI` is set, so the command above stays runnable without any infrastructure:

```sh
docker compose up -d mongodb
MONGODB_URI="mongodb://root:example@localhost:27017" go test ./pkg/...
```

The end to end tests use [Playwright](https://playwright.dev/) through
[`@grafana/plugin-e2e`](https://github.com/grafana/plugin-tools/tree/main/packages/plugin-e2e).
They drive a real Grafana, so the whole stack has to be running first:

```sh
npm run build && mage -v     # the tests run against ./dist
npm run server
npx playwright install chromium
npm run e2e                  # or `npm run e2e:ui` for the interactive runner
```

If Grafana is not on the default port, point the tests at it with `GRAFANA_URL`:

```sh
GRAFANA_URL=http://localhost:3001 npm run e2e
```

All of this runs on every pull request, see [`.github/workflows/ci.yaml`](./.github/workflows/ci.yaml).

### Updating the build configuration

The `.config/` directory is generated by [`@grafana/create-plugin`](https://github.com/grafana/plugin-tools)
and must not be edited by hand. Refresh it with:

```sh
npm run update
```

### Dependencies held back

Every dependency is on its latest release except the ones below, each blocked by something
upstream. Dependabot is configured to skip the updates that cannot be merged, so a red
Dependabot PR means something has genuinely changed.

| Dependency                      | Held at | Blocked by                                                                                                                                                 |
| ------------------------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `typescript`                    | 6       | `@typescript-eslint` 8, the latest release, caps TypeScript at `<6.1.0`.                                                                                   |
| `eslint`                        | 9       | `eslint-plugin-react` 7.37.5, the latest release, supports at most `eslint@^9.7`. ESLint 10 removed `context.getFilename()`, which the plugin still calls. |
| `@grafana/eslint-config`        | 9       | v10 dropped the `./flat.js` entry point that the generated `.config/eslint.config.mjs` imports.                                                            |
| `@stylistic/eslint-plugin-ts`   | 4       | Required as a peer dependency by `@grafana/eslint-config` 9. Superseded upstream by `@stylistic/eslint-plugin`, which v10 uses instead.                    |
| `webpack-subresource-integrity` | 5.1     | 5.2 is a release candidate only.                                                                                                                           |

The three ESLint entries unblock together once `create-plugin` adopts `@grafana/eslint-config` v10.

`react` and `react-dom` follow the version bundled by Grafana, since the plugin uses Grafana's own React
at runtime: React 19 since Grafana 13.2, which is also what the `@grafana/*` packages now require.

On the Go side, both direct dependencies (`grafana-plugin-sdk-go` and `mongo-driver/v2`) and every
indirect entry in `go.mod` are current. `go list -m -u all` still reports updates for modules that
are in the wider module graph but not part of this build, which is expected.
