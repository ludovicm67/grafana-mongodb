---
'@ludovicm67/mongodb-datasource': patch
---

Bring every dependency up to its latest usable release.

- Update the indirect Go dependencies, including `golang.org/x/crypto`, `golang.org/x/net`
  and `prometheus/client_golang`. Both direct dependencies were already current.
- Update `@testing-library/jest-dom` to v7.
- Drop the unused `@babel/core` dependency, the build and the tests both run on SWC.
- Document the dependencies that are held back by upstream constraints, and narrow the
  Dependabot ignore rules to the blocked majors so minor and patch updates still come through.
