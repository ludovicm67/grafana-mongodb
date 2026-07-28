import React, { ChangeEvent, useCallback } from 'react';
import {
  Button,
  Combobox,
  ComboboxOption,
  InlineField,
  InlineFieldRow,
  Input,
  RadioButtonGroup,
  Stack,
  TextArea,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MongoDataSourceOptions, MongoQuery, MongoQueryType, queryTypeOf } from '../types';
import { testIds } from '../testIds';
import { RemoteOptions, useRemoteOptions } from './useRemoteOptions';

type Props = QueryEditorProps<DataSource, MongoQuery, MongoDataSourceOptions>;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<MongoQueryType>> = [
  { label: 'Find', value: 'find', description: 'Return the documents matching a filter' },
  { label: 'Aggregate', value: 'aggregate', description: 'Run an aggregation pipeline' },
  { label: 'Count', value: 'count', description: 'Return how many documents match a filter' },
  { label: 'Distinct', value: 'distinct', description: 'Return the unique values of a field' },
];

/**
 * Width of the labels starting a row, shared so that they line up. It is wide
 * enough for "Timestamp Field", the longest of them, to stay on a single line.
 */
const LABEL_WIDTH = 20;

/** Turns the content of a number input into a query value. */
function toNumber(value: string): number | undefined {
  if (value.trim() === '') {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isNaN(parsed) ? undefined : parsed;
}

/**
 * Names are suggested, never imposed: MongoDB creates a collection on write, a
 * user may not be allowed to list them, and a dashboard variable is a perfectly
 * good value. So the dropdowns always accept a custom value, and a listing
 * failure only turns into a hint under the input.
 */
function noOptionsMessage({ error }: RemoteOptions, subject: string): string {
  return error ? `Could not list the ${subject}: ${error}` : `No ${subject} found`;
}

export function QueryEditor({ datasource, query, onChange, onRunQuery }: Props) {
  const queryType = queryTypeOf(query);
  const { queryText, pipeline, projection, sort, limit, skip, distinctField, database, collection, timestampField } =
    query;

  const isFind = queryType === 'find';
  const isAggregate = queryType === 'aggregate';
  const isDistinct = queryType === 'distinct';

  // Find, count and distinct all take a filter, aggregate takes a pipeline.
  const usesFilter = !isAggregate;

  const onQueryTypeChange = (value: MongoQueryType) => {
    onChange({ ...query, queryType: value });
  };

  const onTextChange = (field: 'queryText' | 'pipeline') => (event: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...query, [field]: event.target.value });
  };

  const onFieldChange = (field: 'projection' | 'sort') => (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, [field]: event.target.value });
  };

  const onNumberChange = (field: 'limit' | 'skip') => (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, [field]: toNumber(event.target.value) });
  };

  const onNameChange =
    (field: 'database' | 'collection' | 'timestampField' | 'distinctField') =>
    (option: ComboboxOption<string> | null) => {
      onChange({ ...query, [field]: option?.value ?? '' });
    };

  // The three listings are chained: the collections depend on the database, and
  // the fields on both. Each loader is memoized, its identity is what tells the
  // hook to load again.
  const databases = useRemoteOptions(useCallback(() => datasource.getDatabases(), [datasource]));
  const collections = useRemoteOptions(useCallback(() => datasource.getCollections(database), [datasource, database]));
  const fields = useRemoteOptions(
    useCallback(() => datasource.getFields(database, collection), [datasource, database, collection])
  );

  return (
    <Stack direction="column" gap={1}>
      <InlineFieldRow>
        <InlineField label="Query Type" labelWidth={LABEL_WIDTH} tooltip="Which MongoDB operation to run.">
          <RadioButtonGroup
            data-testid={testIds.queryEditor.queryType}
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField
          label="Database"
          labelWidth={LABEL_WIDTH}
          required
          invalid={!database}
          error={database ? undefined : 'A database is required to run the query'}
        >
          <Combobox
            data-testid={testIds.queryEditor.database}
            options={databases.options}
            loading={databases.loading}
            value={database ?? null}
            onChange={onNameChange('database')}
            noOptionsMessage={noOptionsMessage(databases, 'databases')}
            placeholder="Select or type a database"
            createCustomValue
            isClearable
            width={30}
          />
        </InlineField>
        <InlineField
          label="Collection"
          required
          invalid={!collection}
          error={collection ? undefined : 'A collection is required to run the query'}
        >
          <Combobox
            data-testid={testIds.queryEditor.collection}
            options={collections.options}
            loading={collections.loading}
            value={collection ?? null}
            onChange={onNameChange('collection')}
            noOptionsMessage={noOptionsMessage(collections, 'collections')}
            placeholder="Select or type a collection"
            createCustomValue
            isClearable
            width={30}
          />
        </InlineField>
        {isDistinct && (
          <InlineField
            label="Field"
            tooltip="The field to collect the unique values of."
            required
            invalid={!distinctField}
            error={distinctField ? undefined : 'A field is required for a distinct query'}
          >
            <Combobox
              data-testid={testIds.queryEditor.distinctField}
              options={fields.options}
              loading={fields.loading}
              value={distinctField ?? null}
              onChange={onNameChange('distinctField')}
              noOptionsMessage={noOptionsMessage(fields, 'fields')}
              placeholder="Select or type a field"
              createCustomValue
              isClearable
              width={30}
            />
          </InlineField>
        )}
      </InlineFieldRow>

      {isFind && (
        <InlineFieldRow>
          <InlineField
            label="Projection"
            labelWidth={LABEL_WIDTH}
            tooltip='Which fields to return, for example {"name": 1, "_id": 0}.'
          >
            <Input
              data-testid={testIds.queryEditor.projection}
              onChange={onFieldChange('projection')}
              value={projection ?? ''}
              placeholder='{"name": 1}'
              width={26}
            />
          </InlineField>
          <InlineField label="Sort" tooltip='How to order the documents, for example {"timestamp": -1}.'>
            <Input
              data-testid={testIds.queryEditor.sort}
              onChange={onFieldChange('sort')}
              value={sort ?? ''}
              placeholder='{"timestamp": -1}'
              width={26}
            />
          </InlineField>
          <InlineField label="Limit" tooltip="Maximum number of documents to return. Leave empty for no limit.">
            <Input
              data-testid={testIds.queryEditor.limit}
              type="number"
              min={0}
              onChange={onNumberChange('limit')}
              value={limit ?? ''}
              placeholder="0"
              width={12}
            />
          </InlineField>
          <InlineField label="Skip" tooltip="How many documents to skip before returning results.">
            <Input
              data-testid={testIds.queryEditor.skip}
              type="number"
              min={0}
              onChange={onNumberChange('skip')}
              value={skip ?? ''}
              placeholder="0"
              width={12}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      <InlineFieldRow>
        <InlineField
          label="Timestamp Field"
          labelWidth={LABEL_WIDTH}
          tooltip={
            usesFilter && !isFind
              ? 'Optional. Name of the field holding the document date. The dashboard time range is added to the query.'
              : 'Optional. Name of the field holding the document date, used to keep only the documents that fall inside the dashboard time range.'
          }
        >
          <Combobox
            data-testid={testIds.queryEditor.timestampField}
            options={fields.options}
            loading={fields.loading}
            value={timestampField ?? null}
            onChange={onNameChange('timestampField')}
            noOptionsMessage={noOptionsMessage(fields, 'fields')}
            placeholder="Select or type a field"
            createCustomValue
            isClearable
            width={30}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        {isAggregate ? (
          <InlineField
            grow
            label="Pipeline"
            labelWidth={LABEL_WIDTH}
            tooltip="An array of aggregation stages, written as extended JSON."
          >
            <TextArea
              data-testid={testIds.queryEditor.pipeline}
              onChange={onTextChange('pipeline')}
              onBlur={onRunQuery}
              value={pipeline ?? ''}
              placeholder={'[\n  { "$group": { "_id": "$level", "total": { "$sum": 1 } } }\n]'}
              rows={8}
            />
          </InlineField>
        ) : (
          <InlineField
            grow
            label="Filter"
            labelWidth={LABEL_WIDTH}
            tooltip="A filter document, written as extended JSON."
          >
            <TextArea
              data-testid={testIds.queryEditor.queryText}
              onChange={onTextChange('queryText')}
              onBlur={onRunQuery}
              value={queryText ?? ''}
              placeholder="Enter a filter…"
              rows={8}
            />
          </InlineField>
        )}
      </InlineFieldRow>

      <InlineFieldRow>
        <Button data-testid={testIds.queryEditor.runQuery} variant="secondary" onClick={onRunQuery}>
          Run Query
        </Button>
      </InlineFieldRow>
    </Stack>
  );
}
