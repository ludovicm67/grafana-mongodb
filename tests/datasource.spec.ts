import { test, expect } from '@grafana/plugin-e2e';
import { testIds } from '../src/testIds';

const PROVISIONED_DATASOURCE = 'mongodb.yaml';

test.describe('datasource configuration', () => {
  test('the provisioned datasource can reach MongoDB', async ({
    readProvisionedDataSource,
    gotoDataSourceConfigPage,
  }) => {
    const datasource = await readProvisionedDataSource({ fileName: PROVISIONED_DATASOURCE });
    const configPage = await gotoDataSourceConfigPage(datasource.uid);

    await expect(configPage.saveAndTest()).toBeOK();
    await expect(configPage).toHaveAlert('success', { hasText: 'MongoDB connection successful' });
  });

  test('an unreachable MongoDB instance is reported as an error', async ({ createDataSourceConfigPage, page }) => {
    const configPage = await createDataSourceConfigPage({ type: 'ludovicm67-mongodb-datasource' });

    // Port 1 is reserved, so nothing can be listening on it. The explicit
    // timeout keeps the test quick and checks that connection string options
    // are passed through to the driver.
    await page.getByTestId(testIds.configEditor.uri).fill('mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=2000');

    await expect(configPage.saveAndTest()).not.toBeOK();
    await expect(configPage).toHaveAlert('error', { hasText: /MongoDB ping failed/ });
  });
});
