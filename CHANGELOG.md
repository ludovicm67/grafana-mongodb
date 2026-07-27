# @ludovicm67/mongodb-datasource

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
