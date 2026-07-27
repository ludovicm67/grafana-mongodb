import { test, expect } from '@grafana/plugin-e2e';
import type { PanelEditPage } from '@grafana/plugin-e2e';
import type { Page } from '@playwright/test';
import { testIds } from '../src/testIds';

const PROVISIONED_DATASOURCE = 'mongodb.yaml';

type QueryFields = {
  database: string;
  collection: string;
  queryText: string;
  timestampField?: string;
};

/**
 * Fills the query editor. The datasource only sends a query to the backend once
 * the database, the collection and the query text are all set.
 */
async function fillQuery(page: Page, { database, collection, queryText, timestampField }: QueryFields) {
  await page.getByTestId(testIds.queryEditor.database).fill(database);
  await page.getByTestId(testIds.queryEditor.collection).fill(collection);
  if (timestampField !== undefined) {
    await page.getByTestId(testIds.queryEditor.timestampField).fill(timestampField);
  }
  await page.getByTestId(testIds.queryEditor.queryText).fill(queryText);
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
    await page.getByTestId(testIds.queryEditor.timestampField).fill('timestamp');
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

    // The fixtures spread one document per minute over the last two hours, so
    // the default "last 6 hours" range always holds some of them.
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

    await page.getByTestId(testIds.queryEditor.database).fill('grafana');
    await expect(databaseError).toBeHidden();
    await expect(collectionError).toBeVisible();

    await page.getByTestId(testIds.queryEditor.collection).fill('fruits');
    await expect(collectionError).toBeHidden();

    // And the query runs once both are filled in.
    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.data).toContainText(['apple']);
  });
});
