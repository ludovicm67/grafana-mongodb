---
'@ludovicm67/mongodb-datasource': patch
---

Update dependencies and move the development stack to MongoDB 9.

- Update the `@grafana/*` packages to 13.3, `grafana-plugin-sdk-go` to 0.298, `tslib` to 2.9 and
  the development dependencies.
- Run the development stack and the CI on MongoDB 9. MongoDB 8 images refuse to start on the
  kernel now used by the GitHub runners (see SERVER-131779), and 9.0.4 is the first published
  image with the fix. Set `MONGODB_VERSION` to use another version locally.
- Run the development stack and the end to end tests on Grafana 13.3.
