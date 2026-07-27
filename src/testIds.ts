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
    database: 'data-testid query-editor-database',
    collection: 'data-testid query-editor-collection',
    timestampField: 'data-testid query-editor-timestamp-field',
    queryText: 'data-testid query-editor-query-text',
    runQuery: 'data-testid query-editor-run-query',
  },
};
