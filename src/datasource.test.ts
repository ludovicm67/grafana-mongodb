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

    // The backend reads an empty filter as `{}` and an empty pipeline as `[]`,
    // so neither must block the request.
    it('runs a query without a query text', () => {
      expect(datasource.filterQuery({ ...complete, queryText: '' })).toBe(true);
      expect(datasource.filterQuery({ ...complete, queryText: undefined })).toBe(true);
    });

    it('runs an aggregation without a pipeline', () => {
      expect(datasource.filterQuery({ ...complete, queryType: 'aggregate', pipeline: '' })).toBe(true);
    });

    // Distinct is the one type that cannot run without an extra field.
    it('skips a distinct query without a field', () => {
      expect(datasource.filterQuery({ ...complete, queryType: 'distinct' })).toBe(false);
      expect(datasource.filterQuery({ ...complete, queryType: 'distinct', distinctField: '' })).toBe(false);
    });

    it('runs a distinct query with a field', () => {
      expect(datasource.filterQuery({ ...complete, queryType: 'distinct', distinctField: 'level' })).toBe(true);
    });
  });
});
