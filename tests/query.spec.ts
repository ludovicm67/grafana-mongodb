import { test, expect } from '@grafana/plugin-e2e';
import type { PanelEditPage } from '@grafana/plugin-e2e';
import type { Page } from '@playwright/test';
import { testIds } from '../src/testIds';

const PROVISIONED_DATASOURCE = 'mongodb.yaml';

type QueryFields = {
  queryType?: 'Find' | 'Aggregate' | 'Count' | 'Distinct';
  database: string;
  collection: string;
  queryText?: string;
  pipeline?: string;
  projection?: string;
  sort?: string;
  limit?: string;
  skip?: string;
  distinctField?: string;
  timestampField?: string;
};

/**
 * Sets one of the name dropdowns. They suggest what exists but accept anything,
 * so the value is typed and committed rather than picked from the list.
 */
async function fillCombobox(page: Page, testId: string, value: string) {
  const input = page.getByTestId(testId);
  await input.click();
  await input.fill('');
  // The value is typed key by key, so that the dropdown narrows down to it.
  // A single fill() would leave the whole list showing, and pressing Enter
  // would then pick whichever entry happens to be first. The delay leaves the
  // combobox time to re-render between keystrokes, otherwise it drops some.
  await input.pressSequentially(value, { delay: 30 });
  await expect(input).toHaveValue(value);

  // Wait for the dropdown to offer the value, either as an entry of its own or
  // as a custom one. Pressing Enter before that commits nothing, and the input
  // falls back to the value already stored in the query.
  await expect(page.getByRole('option').filter({ hasText: value }).first()).toBeVisible();

  await page.keyboard.press('Enter');
  await expect(input).toHaveValue(value);
}

/**
 * Fills the query editor. The datasource only sends a query to the backend once
 * the database and the collection are set.
 */
async function fillQuery(page: Page, fields: QueryFields) {
  // The query type comes first, it decides which inputs are on screen.
  if (fields.queryType) {
    await page.getByLabel(fields.queryType, { exact: true }).click();
  }

  await fillCombobox(page, testIds.queryEditor.database, fields.database);
  await fillCombobox(page, testIds.queryEditor.collection, fields.collection);

  if (fields.distinctField !== undefined) {
    await fillCombobox(page, testIds.queryEditor.distinctField, fields.distinctField);
  }
  if (fields.timestampField !== undefined) {
    await fillCombobox(page, testIds.queryEditor.timestampField, fields.timestampField);
  }

  const plain: Array<[string | undefined, string]> = [
    [fields.projection, testIds.queryEditor.projection],
    [fields.sort, testIds.queryEditor.sort],
    [fields.limit, testIds.queryEditor.limit],
    [fields.skip, testIds.queryEditor.skip],
    [fields.pipeline, testIds.queryEditor.pipeline],
    [fields.queryText, testIds.queryEditor.queryText],
  ];

  for (const [value, testId] of plain) {
    if (value !== undefined) {
      await page.getByTestId(testId).fill(value);
    }
  }
}

/**
 * Opens a table panel backed by the provisioned MongoDB datasource.
 *
 * The visualization is picked first so that the suggestions pane, which covers
 * the query editor while the panel is still loading, is closed before the
 * datasource picker is used.
 */
async function setupPanel(panelEditPage: PanelEditPage, page: Page, datasourceName: string) {
  await panelEditPage.setVisualization('Table');
  await panelEditPage.datasource.set(datasourceName);

  // The query editor only renders once the datasource module is loaded.
  await expect(page.getByTestId(testIds.queryEditor.database)).toBeVisible();
}

test.describe('querying MongoDB through Grafana', () => {
  test('returns every document of a collection', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, { database: 'grafana', collection: 'fruits', queryText: '{}' });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.getErrorIcon()).not.toBeVisible();
    await expect(panelEditPage.panel.data).toContainText(['apple', 'banana', 'kiwi']);
  });

  test('applies MongoDB query operators', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      database: 'grafana',
      collection: 'fruits',
      // Comments are stripped before the query is sent to MongoDB.
      queryText: '// only the fruits we have plenty of\n{ "quantity": { "$gt": 4 } }',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.data).toContainText(['apple', 'banana']);
    await expect(panelEditPage.panel.data).not.toContainText(['kiwi']);
  });

  test('keeps only the documents inside the dashboard time range', async ({
    panelEditPage,
    page,
    readProvisionedDataSource,
  }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    // The fixtures hold a single document dated 2020, which is always outside
    // the default "last 6 hours" range of a new panel.
    await fillQuery(page, { database: 'grafana', collection: 'logs', queryText: '{ "level": "ancient" }' });

    // Without a timestamp field the whole collection is returned.
    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.data).toContainText(['ancient log']);

    // Setting the timestamp field filters that document out.
    await fillCombobox(page, testIds.queryEditor.timestampField, 'timestamp');
    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.getErrorIcon()).not.toBeVisible();
    await expect(panelEditPage.panel.data).toHaveCount(0);
  });

  test('returns the documents of the current time range', async ({
    panelEditPage,
    page,
    readProvisionedDataSource,
  }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    // The `logs` fixtures are dated relative to the moment the MongoDB
    // container started, so a long running development stack pushes them out of
    // the default "last 6 hours". The range is widened here to keep the test
    // about the timestamp field rather than about the age of the container.
    // The narrow case is covered by the "ancient log" test above, whose
    // document has a fixed date.
    //
    // It is set before the query is filled in, so that no query is in flight
    // when the panel is refreshed below.
    await panelEditPage.timeRange.set({ from: 'now-5y', to: 'now' });

    await fillQuery(page, {
      database: 'grafana',
      collection: 'logs',
      queryText: '{ "level": "error" }',
      timestampField: 'timestamp',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.getErrorIcon()).not.toBeVisible();
    await expect(panelEditPage.panel.fieldNames).toContainText(['timestamp']);
    await expect(panelEditPage.panel.data).toContainText(['error']);
  });

  test('reports a backend error for a malformed query', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, { database: 'grafana', collection: 'fruits', queryText: '{ "unbalanced":' });

    await expect(panelEditPage.refreshPanel()).not.toBeOK();
  });

  // A query without a database or a collection is never sent to the backend,
  // so the editor is the only place that can tell the user what is missing.
  test('flags a missing database and collection in the editor', async ({
    panelEditPage,
    page,
    readProvisionedDataSource,
  }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    const databaseError = page.getByText('A database is required to run the query');
    const collectionError = page.getByText('A collection is required to run the query');

    // A brand new query has neither, so both fields complain straight away.
    await expect(databaseError).toBeVisible();
    await expect(collectionError).toBeVisible();

    await fillCombobox(page, testIds.queryEditor.database, 'grafana');
    await expect(databaseError).toBeHidden();
    await expect(collectionError).toBeVisible();

    await fillCombobox(page, testIds.queryEditor.collection, 'fruits');
    await expect(collectionError).toBeHidden();

    // And the query runs once both are filled in.
    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.data).toContainText(['apple']);
  });
});

test.describe('find options', () => {
  test('limits the returned fields with a projection', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      database: 'grafana',
      collection: 'fruits',
      queryText: '{}',
      projection: '{ "name": 1, "_id": 0 }',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.fieldNames).toHaveText(['name']);
  });

  test('orders the documents with a sort', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      database: 'grafana',
      collection: 'fruits',
      queryText: '{}',
      projection: '{ "name": 1, "_id": 0 }',
      sort: '{ "quantity": -1 }',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    // banana has 12, apple 5, kiwi 3.
    await expect(panelEditPage.panel.data).toHaveText(['banana', 'apple', 'kiwi']);
  });

  test('bounds the results with a limit and a skip', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      database: 'grafana',
      collection: 'fruits',
      queryText: '{}',
      projection: '{ "name": 1, "_id": 0 }',
      sort: '{ "quantity": -1 }',
      skip: '1',
      limit: '1',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.data).toHaveText(['apple']);
  });
});

test.describe('other query types', () => {
  test('aggregates with a pipeline', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      queryType: 'Aggregate',
      database: 'grafana',
      collection: 'fruits',
      pipeline: '[{ "$group": { "_id": null, "total": { "$sum": "$quantity" } } }]',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.getErrorIcon()).not.toBeVisible();
    // 5 + 12 + 3
    await expect(panelEditPage.panel.data).toContainText(['20']);
  });

  test('reports a pipeline that is not an array', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      queryType: 'Aggregate',
      database: 'grafana',
      collection: 'fruits',
      pipeline: '{ "$match": {} }',
    });

    await expect(panelEditPage.refreshPanel()).not.toBeOK();
  });

  test('counts the matching documents', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      queryType: 'Count',
      database: 'grafana',
      collection: 'fruits',
      queryText: '{ "quantity": { "$gt": 4 } }',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.fieldNames).toHaveText(['count']);
    await expect(panelEditPage.panel.data).toHaveText(['2']);
  });

  test('lists the unique values of a field', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, {
      queryType: 'Distinct',
      database: 'grafana',
      collection: 'logs',
      distinctField: 'level',
      queryText: '{}',
    });

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.fieldNames).toHaveText(['level']);
    await expect(panelEditPage.panel.data).toHaveText(['ancient', 'error', 'info', 'warn']);
  });

  // Distinct is the one type that needs an extra field, so the editor has to
  // ask for it rather than letting the panel look empty.
  test('flags a distinct query without a field', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillQuery(page, { queryType: 'Distinct', database: 'grafana', collection: 'logs' });

    const fieldError = page.getByText('A field is required for a distinct query');
    await expect(fieldError).toBeVisible();

    await fillCombobox(page, testIds.queryEditor.distinctField, 'level');
    await expect(fieldError).toBeHidden();
    await expect(panelEditPage.refreshPanel()).toBeOK();
  });

  test('switching type swaps the filter for a pipeline', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await expect(page.getByTestId(testIds.queryEditor.queryText)).toBeVisible();
    await expect(page.getByTestId(testIds.queryEditor.projection)).toBeVisible();

    await page.getByLabel('Aggregate', { exact: true }).click();

    await expect(page.getByTestId(testIds.queryEditor.pipeline)).toBeVisible();
    await expect(page.getByTestId(testIds.queryEditor.queryText)).toBeHidden();
    // The projection, sort, limit and skip only apply to a find.
    await expect(page.getByTestId(testIds.queryEditor.projection)).toBeHidden();
  });
});

test.describe('name suggestions', () => {
  test('suggests the databases of the instance', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await page.getByTestId(testIds.queryEditor.database).click();

    // The seeded instance holds the `grafana` database, next to the internal ones.
    await expect(page.getByRole('option', { name: 'grafana', exact: true })).toBeVisible();
  });

  test('suggests the collections of the selected database', async ({
    panelEditPage,
    page,
    readProvisionedDataSource,
  }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillCombobox(page, testIds.queryEditor.database, 'grafana');
    await page.getByTestId(testIds.queryEditor.collection).click();

    await expect(page.getByRole('option', { name: 'fruits', exact: true })).toBeVisible();
    await expect(page.getByRole('option', { name: 'logs', exact: true })).toBeVisible();
  });

  test('suggests the fields of the selected collection', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillCombobox(page, testIds.queryEditor.database, 'grafana');
    await fillCombobox(page, testIds.queryEditor.collection, 'logs');
    await page.getByTestId(testIds.queryEditor.timestampField).click();

    // The fields are read from a sample of the documents.
    await expect(page.getByRole('option', { name: 'timestamp', exact: true })).toBeVisible();
    await expect(page.getByRole('option', { name: 'level', exact: true })).toBeVisible();
  });

  test('picking the suggestions runs the query', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await page.getByTestId(testIds.queryEditor.database).click();
    await page.getByRole('option', { name: 'grafana', exact: true }).click();

    await page.getByTestId(testIds.queryEditor.collection).click();
    await page.getByRole('option', { name: 'fruits', exact: true }).click();

    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.data).toContainText(['apple']);
  });

  // MongoDB creates a collection on write, so a name that does not exist yet
  // has to stay usable.
  test('accepts a collection that does not exist yet', async ({ panelEditPage, page, readProvisionedDataSource }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    await setupPanel(panelEditPage, page, datasource.name);

    await fillCombobox(page, testIds.queryEditor.database, 'grafana');
    await fillCombobox(page, testIds.queryEditor.collection, 'not_created_yet');
    await page.getByTestId(testIds.queryEditor.queryText).fill('{}');

    // Querying an unknown collection is not an error, it simply returns nothing.
    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.getErrorIcon()).not.toBeVisible();
  });
});
