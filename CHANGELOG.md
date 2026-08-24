# @ludovicm67/mongodb-datasource

## 1.2.1

### Patch Changes

- fb116e6: Bring every dependency up to its latest usable release.

  - Update the `@grafana/*` packages to 13.2 and move to React 19, which Grafana bundles since
    13.2 and which those packages now require. The plugin keeps using Grafana's own React at
    runtime, so nothing changes for the Grafana versions it runs on.
  - Update the Go toolchain to 1.27 and every Go dependency, including `grafana-plugin-sdk-go`,
    the OpenTelemetry modules, `google.golang.org/grpc`, `golang.org/x/crypto` and `golang.org/x/net`.
  - Update Changesets to v3 and the release workflow to `changesets/action` v2.
  - Run the development stack and the end to end tests on Grafana 13.2.

## 1.2.0

### Minor Changes

- f1a32d1: Support more ways of querying MongoDB than a plain `find`.

  - Add a **Query Type** selector to the query editor, with **Find**, **Aggregate**, **Count** and
    **Distinct**. Existing queries have no type stored and keep running as a find.
  - **Find** gains a projection, a sort, a limit and a skip. The sort keeps the order of its keys.
  - **Aggregate** runs an aggregation pipeline, written as an array of stages.
  - **Count** returns the number of matching documents as a single value, ready for a stat panel.
  - **Distinct** returns the unique values of a field, optionally restricted by a filter.
  - The query editor only shows the inputs that apply to the selected type, and keeps the filter and
    the pipeline apart so switching type does not discard what was typed.
  - **Timestamp Field** now works for every type. Find and aggregate filter the documents they read
    back, while count and distinct push the range into the query, matching both the numeric and the
    date representation of a timestamp.
  - Report an unknown query type, a pipeline that is not an array, a distinct query without a field,
    and a negative limit or skip as clear errors.

- f1a32d1: Suggest the database, collection and field names in the query editor.

  - The backend now answers resource calls listing the databases of the instance, the collections of
    a database, and the field names found in a sample of the documents of a collection.
  - **Database**, **Collection**, **Timestamp Field** and the distinct **Field** became dropdowns that
    load those names, chained so that each one narrows the next.
  - The dropdowns still accept any value, since a collection may not exist yet, a user may not be
    allowed to list them, and a dashboard variable is a valid entry. When listing fails, the reason is
    shown in the dropdown rather than blocking the input.

## 1.1.1

### Patch Changes

- 138ae9a: Bring every dependency up to its latest usable release.

  - Update the indirect Go dependencies, including `golang.org/x/crypto`, `golang.org/x/net`
    and `prometheus/client_golang`. Both direct dependencies were already current.
  - Update `@testing-library/jest-dom` to v7.
  - Drop the unused `@babel/core` dependency, the build and the tests both run on SWC.
  - Document the dependencies that are held back by upstream constraints, and narrow the
    Dependabot ignore rules to the blocked majors so minor and patch updates still come through.

## 1.1.0

### Minor Changes

- 7da4b5e: Update the whole toolchain and harden the datasource.

  - Update the `@grafana/create-plugin` scaffolding to v7 and the `@grafana/*` packages to v13.
  - Replace the removed `@grafana/e2e` / Cypress setup with Playwright and `@grafana/plugin-e2e`.
  - Update the backend to the MongoDB Go driver v2 and the latest `grafana-plugin-sdk-go`.
  - Reuse a single pooled MongoDB client per datasource instead of dialing on every query, and
    release it on dispose.
  - Fail fast when MongoDB is unreachable, instead of blocking for the driver default of 30 seconds.
    The timeouts stay overridable through `serverSelectionTimeoutMS` and `connectTimeoutMS` in the URI.
  - Parse queries as MongoDB extended JSON, which adds support for the `$oid` and `$date` wrappers.
  - Expose the configured timestamp field as a real time field, so it can be used on a time axis.
  - Reject queries without a database or a collection with a clear error, and mark those fields
    as invalid in the query editor so an incomplete query no longer looks like an empty result.
  - Replace the deprecated `VerticalGroup` component with `Stack`.
  - Raise the minimum supported Grafana version to 11.0.0.

## 1.0.0

### Major Changes

- 0529a2e: Initial release
