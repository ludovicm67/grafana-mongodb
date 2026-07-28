import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

/**
 * The MongoDB operations the datasource can run.
 */
export const QUERY_TYPES = ['find', 'aggregate', 'count', 'distinct'] as const;

export type MongoQueryType = (typeof QUERY_TYPES)[number];

export const DEFAULT_QUERY_TYPE: MongoQueryType = 'find';

export interface MongoQuery extends DataQuery {
  /** The operation to run. Queries saved before this existed are treated as a find. */
  queryType?: MongoQueryType;

  database?: string;
  collection?: string;

  /** The filter document, used by find, count and distinct. */
  queryText?: string;

  /** Aggregation pipeline, as an array of stages. Only used by aggregate. */
  pipeline?: string;

  /** Find only. */
  projection?: string;
  sort?: string;
  limit?: number;
  skip?: number;

  /** The field to collect the unique values of. Only used by distinct. */
  distinctField?: string;

  /** Name of the field carrying the document date. */
  timestampField?: string;
}

export const DEFAULT_QUERY: Partial<MongoQuery> = {
  queryType: DEFAULT_QUERY_TYPE,
  queryText: '{}',
};

/** The operation a query runs, falling back to find. */
export function queryTypeOf(query: MongoQuery): MongoQueryType {
  return query.queryType ?? DEFAULT_QUERY_TYPE;
}

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
