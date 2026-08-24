---
'@ludovicm67/mongodb-datasource': patch
---

Bring every dependency up to its latest usable release.

- Update the `@grafana/*` packages to 13.2 and move to React 19, which Grafana bundles since
  13.2 and which those packages now require. The plugin keeps using Grafana's own React at
  runtime, so nothing changes for the Grafana versions it runs on.
- Update the Go toolchain to 1.27 and every Go dependency, including `grafana-plugin-sdk-go`,
  the OpenTelemetry modules, `google.golang.org/grpc`, `golang.org/x/crypto` and `golang.org/x/net`.
- Update Changesets to v3 and the release workflow to `changesets/action` v2.
- Run the development stack and the end to end tests on Grafana 13.2.
