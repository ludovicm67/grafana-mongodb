import { CoreApp } from '@grafana/data';
import { DataSource } from './datasource';
import { DEFAULT_QUERY } from './types';

jest.mock('@grafana/runtime', () => ({
  DataSourceWithBackend: class {},
}));

describe('DataSource', () => {
  const datasource = new DataSource({} as never);

  it('starts from the default query', () => {
    expect(datasource.getDefaultQuery(CoreApp.PanelEditor)).toEqual(DEFAULT_QUERY);
  });

  describe('filterQuery', () => {
    const complete = { refId: 'A', database: 'grafana', collection: 'fruits', queryText: '{}' };

    it('runs a complete query', () => {
      expect(datasource.filterQuery(complete)).toBe(true);
    });

    it.each(['database', 'collection'] as const)('skips a query without a %s', (field) => {
      expect(datasource.filterQuery({ ...complete, [field]: '' })).toBe(false);
      expect(datasource.filterQuery({ ...complete, [field]: undefined })).toBe(false);
    });

    // The backend reads an empty query text as `{}`, so it must not block the request.
    it('runs a query without a query text', () => {
      expect(datasource.filterQuery({ ...complete, queryText: '' })).toBe(true);
      expect(datasource.filterQuery({ ...complete, queryText: undefined })).toBe(true);
    });
  });
});
