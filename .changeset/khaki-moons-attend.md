---
'@ludovicm67/mongodb-datasource': minor
---

Support more ways of querying MongoDB than a plain `find`.

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
