import { DataSourceInstanceSettings, CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { MongoQuery, MongoDataSourceOptions, DEFAULT_QUERY, queryTypeOf } from './types';

export class DataSource extends DataSourceWithBackend<MongoQuery, MongoDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MongoDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<MongoQuery> {
    return DEFAULT_QUERY;
  }

  /**
   * Only send a query to the backend once it has everything it needs, so that a
   * half filled editor does not produce a stream of errors.
   *
   * The query editor marks the missing fields as invalid, otherwise skipping
   * the request here would look like the query silently returned nothing.
   *
   * An empty filter or pipeline is allowed on purpose: the backend reads them
   * as `{}` and `[]`, which match every document.
   */
  filterQuery(query: MongoQuery): boolean {
    if (!query.database || !query.collection) {
      return false;
    }

    if (queryTypeOf(query) === 'distinct') {
      return Boolean(query.distinctField);
    }

    return true;
  }

  /** Names of the databases of the instance, for the query editor dropdown. */
  async getDatabases(): Promise<string[]> {
    return this.getResource<string[]>('databases');
  }

  /** Names of the collections of a database. */
  async getCollections(database?: string): Promise<string[]> {
    if (!database) {
      return [];
    }
    return this.getResource<string[]>('collections', { database });
  }

  /**
   * Field names found in a sample of the documents of a collection. MongoDB has
   * no schema, so this is a hint rather than an exhaustive list, and the editor
   * still accepts a field that is not part of it.
   */
  async getFields(database?: string, collection?: string): Promise<string[]> {
    if (!database || !collection) {
      return [];
    }
    return this.getResource<string[]>('fields', { database, collection });
  }
}
