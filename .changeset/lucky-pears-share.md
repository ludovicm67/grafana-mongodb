---
'@ludovicm67/mongodb-datasource': minor
---

Suggest the database, collection and field names in the query editor.

- The backend now answers resource calls listing the databases of the instance, the collections of
  a database, and the field names found in a sample of the documents of a collection.
- **Database**, **Collection**, **Timestamp Field** and the distinct **Field** became dropdowns that
  load those names, chained so that each one narrows the next.
- The dropdowns still accept any value, since a collection may not exist yet, a user may not be
  allowed to list them, and a dashboard variable is a valid entry. When listing fails, the reason is
  shown in the dropdown rather than blocking the input.
