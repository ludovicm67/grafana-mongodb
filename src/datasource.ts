import { DataSourceInstanceSettings, CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { MongoQuery, MongoDataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<MongoQuery, MongoDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MongoDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<MongoQuery> {
    return DEFAULT_QUERY;
  }

  /**
   * Only send a query to the backend once it actually targets a collection,
   * so that a half filled editor does not produce a stream of errors.
   *
   * The query editor marks the missing fields as invalid, otherwise skipping
   * the request here would look like the query silently returned nothing.
   *
   * An empty query text is allowed on purpose, the backend reads it as `{}`.
   */
  filterQuery(query: MongoQuery): boolean {
    return Boolean(query.database && query.collection);
  }
}
