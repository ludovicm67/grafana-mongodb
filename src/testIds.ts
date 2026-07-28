/**
 * Shared `data-testid` values, used by the components and by the Playwright e2e tests.
 */
export const testIds = {
  configEditor: {
    uri: 'data-testid config-editor-uri',
    username: 'data-testid config-editor-username',
    password: 'data-testid config-editor-password',
  },
  queryEditor: {
    queryType: 'data-testid query-editor-query-type',
    database: 'data-testid query-editor-database',
    collection: 'data-testid query-editor-collection',
    timestampField: 'data-testid query-editor-timestamp-field',
    queryText: 'data-testid query-editor-query-text',
    pipeline: 'data-testid query-editor-pipeline',
    projection: 'data-testid query-editor-projection',
    sort: 'data-testid query-editor-sort',
    limit: 'data-testid query-editor-limit',
    skip: 'data-testid query-editor-skip',
    distinctField: 'data-testid query-editor-distinct-field',
    runQuery: 'data-testid query-editor-run-query',
  },
};
