import React, { ChangeEvent } from 'react';
import {
  Button,
  InlineField,
  InlineFieldRow,
  Input,
  // QueryField,
  TextArea,
  // TypeaheadInput,
  // TypeaheadOutput,
  VerticalGroup,
} from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

// const onSuggest = async (input: TypeaheadInput): Promise<TypeaheadOutput> => {
//   const text = input.text;
//   if (text.endsWith('$')) {
//     const mongoOperators = [
//       { label: '$eq', documentation: 'Matches values that are equal to a specified value' },
//       { label: '$gt', documentation: 'Matches values that are greater than a specified value' },
//       { label: '$gte', documentation: 'Matches values that are greater than or equal to a specified value' },
//       { label: '$in', documentation: 'Matches any of the values specified in an array' },
//       { label: '$lt', documentation: 'Matches values that are less than a specified value' },
//       { label: '$lte', documentation: 'Matches values that are less than or equal to a specified value' },
//       { label: '$ne', documentation: 'Matches all values that are not equal to a specified value' },
//       { label: '$nin', documentation: 'Matches none of the values specified in an array' },
//       {
//         label: '$and',
//         documentation:
//           'Joins query clauses with a logical AND returns all documents that match the conditions of both clauses',
//       },
//       {
//         label: '$or',
//         documentation:
//           'Joins query clauses with a logical OR returns all documents that match the conditions of either clause',
//       },
//       {
//         label: '$not',
//         documentation:
//           'Inverts the effect of a query expression and returns documents that do not match the query expression',
//       },
//       {
//         label: '$nor',
//         documentation: 'Joins query clauses with a logical NOR returns all documents that fail to match both clauses',
//       },
//     ];

//     return { suggestions: [{ label: 'Operators', items: mongoOperators }] };
//   }

//   return { suggestions: [] };
// };

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
    <div className="gf-form">
      <VerticalGroup spacing="sm">
        <InlineFieldRow>
          <InlineField label="Database">
            <Input onChange={onDatabaseChange} value={database} />
          </InlineField>
          <InlineField label="Collection">
            <Input onChange={onCollectionChange} value={collection} />
          </InlineField>
          <InlineField label="Timestamp Field">
            <Input onChange={onTimestampFieldChange} value={timestampField} />
          </InlineField>
        </InlineFieldRow>
        <InlineFieldRow style={{ width: '100%' }}>
          <InlineField grow style={{ width: '100%' }}>
            <TextArea
              onChange={onQueryTextChange}
              // onTypeahead={onSuggest}
              value={queryText}
              placeholder="Enter a query…"
              rows={8}
            />
          </InlineField>
        </InlineFieldRow>
        <InlineFieldRow>
          <Button variant="secondary" onClick={onRunQuery}>
            Run Query
          </Button>
        </InlineFieldRow>
      </VerticalGroup>
    </div>
  );
}
