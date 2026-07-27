import React, { ChangeEvent } from 'react';
import { Button, InlineField, InlineFieldRow, Input, Stack, TextArea } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MongoDataSourceOptions, MongoQuery } from '../types';
import { testIds } from '../testIds';

type Props = QueryEditorProps<DataSource, MongoQuery, MongoDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onQueryTextChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...query, queryText: event.target.value });
  };

  const onCollectionChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, collection: event.target.value });
  };

  const onTimestampFieldChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, timestampField: event.target.value });
  };

  const onDatabaseChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, database: event.target.value });
  };

  const { queryText, database, collection, timestampField } = query;

  return (
    <Stack direction="column" gap={1}>
      <InlineFieldRow>
        <InlineField
          label="Database"
          required
          invalid={!database}
          error={database ? undefined : 'A database is required to run the query'}
        >
          <Input data-testid={testIds.queryEditor.database} onChange={onDatabaseChange} value={database ?? ''} />
        </InlineField>
        <InlineField
          label="Collection"
          required
          invalid={!collection}
          error={collection ? undefined : 'A collection is required to run the query'}
        >
          <Input data-testid={testIds.queryEditor.collection} onChange={onCollectionChange} value={collection ?? ''} />
        </InlineField>
        <InlineField
          label="Timestamp Field"
          tooltip="Optional. Name of the field holding a UNIX timestamp in milliseconds, used to keep only the documents that fall inside the dashboard time range."
        >
          <Input
            data-testid={testIds.queryEditor.timestampField}
            onChange={onTimestampFieldChange}
            value={timestampField ?? ''}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField grow label="Query" labelWidth={16}>
          <TextArea
            data-testid={testIds.queryEditor.queryText}
            onChange={onQueryTextChange}
            onBlur={onRunQuery}
            value={queryText ?? ''}
            placeholder="Enter a query…"
            rows={8}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <Button data-testid={testIds.queryEditor.runQuery} variant="secondary" onClick={onRunQuery}>
          Run Query
        </Button>
      </InlineFieldRow>
    </Stack>
  );
}
