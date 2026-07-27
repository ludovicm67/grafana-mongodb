import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export interface MongoQuery extends DataQuery {
  queryText?: string;
  database?: string;
  collection?: string;
  timestampField?: string;
}

export const DEFAULT_QUERY: Partial<MongoQuery> = {
  queryText: '{}',
};

/**
 * These are options configured for each DataSource instance
 */
export interface MongoDataSourceOptions extends DataSourceJsonData {
  uri?: string;
  username?: string;
  database?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MongoSecureJsonData {
  password?: string;
}
