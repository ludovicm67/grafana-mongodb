import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryEditor } from './QueryEditor';
import { testIds } from '../testIds';
import type { DataSource } from '../datasource';
import type { MongoQuery } from '../types';

const query: MongoQuery = { refId: 'A', database: 'grafana', collection: 'fruits', queryText: '{}' };

// The editor never touches the datasource instance, it only reports changes
// back through its callbacks.
const datasource = {} as DataSource;

function setup(overrides: Partial<MongoQuery> = {}) {
  const onChange = jest.fn();
  const onRunQuery = jest.fn();

  render(
    <QueryEditor
      datasource={datasource}
      query={{ ...query, ...overrides }}
      onChange={onChange}
      onRunQuery={onRunQuery}
    />
  );

  return { onChange, onRunQuery };
}

describe('QueryEditor', () => {
  it('shows the current query', () => {
    setup();

    expect(screen.getByTestId(testIds.queryEditor.database)).toHaveValue('grafana');
    expect(screen.getByTestId(testIds.queryEditor.collection)).toHaveValue('fruits');
    expect(screen.getByTestId(testIds.queryEditor.queryText)).toHaveValue('{}');
    expect(screen.getByTestId(testIds.queryEditor.timestampField)).toHaveValue('');
  });

  // The inputs are controlled by the `query` prop, which does not change during
  // the test, so a single change event is used instead of typing.
  it.each([
    ['database', testIds.queryEditor.database, 'logs'],
    ['collection', testIds.queryEditor.collection, 'events'],
    ['timestampField', testIds.queryEditor.timestampField, 'ts'],
    ['queryText', testIds.queryEditor.queryText, '{ "level": "error" }'],
  ])('reports a change of the %s field', (field, testId, value) => {
    const { onChange } = setup();

    fireEvent.change(screen.getByTestId(testId), { target: { value } });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ ...query, [field]: value });
  });

  it('runs the query when the button is clicked', async () => {
    const { onRunQuery } = setup();

    await userEvent.click(screen.getByTestId(testIds.queryEditor.runQuery));

    expect(onRunQuery).toHaveBeenCalledTimes(1);
  });

  it('runs the query when the query text loses focus', async () => {
    const { onRunQuery } = setup();

    await userEvent.click(screen.getByTestId(testIds.queryEditor.queryText));
    await userEvent.tab();

    expect(onRunQuery).toHaveBeenCalledTimes(1);
  });

  it('renders an empty editor without crashing on undefined fields', () => {
    render(<QueryEditor datasource={datasource} query={{ refId: 'A' }} onChange={jest.fn()} onRunQuery={jest.fn()} />);

    expect(screen.getByTestId(testIds.queryEditor.database)).toHaveValue('');
    expect(screen.getByTestId(testIds.queryEditor.queryText)).toHaveValue('');
  });

  // Without a database and a collection the datasource skips the request, so
  // the editor has to say why rather than letting the panel show "No data".
  describe('validation', () => {
    it('flags a missing database', () => {
      setup({ database: '' });

      expect(screen.getByText('A database is required to run the query')).toBeInTheDocument();
      expect(screen.queryByText('A collection is required to run the query')).not.toBeInTheDocument();
    });

    it('flags a missing collection', () => {
      setup({ collection: '' });

      expect(screen.getByText('A collection is required to run the query')).toBeInTheDocument();
      expect(screen.queryByText('A database is required to run the query')).not.toBeInTheDocument();
    });

    it('flags both when the editor is empty', () => {
      setup({ database: undefined, collection: undefined });

      expect(screen.getByText('A database is required to run the query')).toBeInTheDocument();
      expect(screen.getByText('A collection is required to run the query')).toBeInTheDocument();
    });

    it('shows no error once both are set', () => {
      setup();

      expect(screen.queryByText(/is required to run the query/)).not.toBeInTheDocument();
    });
  });
});
