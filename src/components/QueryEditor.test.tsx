import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryEditor } from './QueryEditor';
import { testIds } from '../testIds';
import type { DataSource } from '../datasource';
import type { MongoQuery } from '../types';

const query: MongoQuery = {
  refId: 'A',
  queryType: 'find',
  database: 'grafana',
  collection: 'fruits',
  queryText: '{}',
};

type Listings = {
  getDatabases: jest.Mock;
  getCollections: jest.Mock;
  getFields: jest.Mock;
};

/**
 * The editor only uses the datasource to list the names it offers in its
 * dropdowns, so a stub with those three methods is enough.
 */
function makeDatasource(overrides: Partial<Listings> = {}): DataSource & Listings {
  return {
    getDatabases: jest.fn().mockResolvedValue(['admin', 'grafana']),
    getCollections: jest.fn().mockResolvedValue(['fruits', 'logs']),
    getFields: jest.fn().mockResolvedValue(['_id', 'level', 'timestamp']),
    ...overrides,
  } as unknown as DataSource & Listings;
}

function setup(overrides: Partial<MongoQuery> = {}, datasource: DataSource & Listings = makeDatasource()) {
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

  return { onChange, onRunQuery, datasource };
}

/** Waits for the listings triggered on mount to settle. */
async function settled(datasource: Listings) {
  await waitFor(() => expect(datasource.getDatabases).toHaveBeenCalled());
}

/** Picks an existing entry out of one of the name dropdowns. */
async function pickOption(testId: string, name: string) {
  await userEvent.click(screen.getByTestId(testId));
  await userEvent.click(await screen.findByRole('option', { name }));
}

describe('QueryEditor', () => {
  it('shows the current query', async () => {
    const { datasource } = setup();
    await settled(datasource);

    expect(screen.getByTestId(testIds.queryEditor.database)).toHaveValue('grafana');
    expect(screen.getByTestId(testIds.queryEditor.collection)).toHaveValue('fruits');
    expect(screen.getByTestId(testIds.queryEditor.queryText)).toHaveValue('{}');
    expect(screen.getByTestId(testIds.queryEditor.timestampField)).toHaveValue('');
  });

  // The text areas are controlled by the `query` prop, which does not change
  // during the test, so a single change event is used instead of typing.
  it('reports a change of the filter', async () => {
    const { onChange, datasource } = setup();
    await settled(datasource);

    fireEvent.change(screen.getByTestId(testIds.queryEditor.queryText), { target: { value: '{ "level": "error" }' } });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ ...query, queryText: '{ "level": "error" }' });
  });

  it('runs the query when the button is clicked', async () => {
    const { onRunQuery, datasource } = setup();
    await settled(datasource);

    await userEvent.click(screen.getByTestId(testIds.queryEditor.runQuery));

    expect(onRunQuery).toHaveBeenCalledTimes(1);
  });

  it('runs the query when the filter loses focus', async () => {
    const { onRunQuery, datasource } = setup();
    await settled(datasource);

    await userEvent.click(screen.getByTestId(testIds.queryEditor.queryText));
    await userEvent.tab();

    expect(onRunQuery).toHaveBeenCalledTimes(1);
  });

  it('renders an empty editor without crashing on undefined fields', async () => {
    const datasource = makeDatasource();
    render(<QueryEditor datasource={datasource} query={{ refId: 'A' }} onChange={jest.fn()} onRunQuery={jest.fn()} />);
    await settled(datasource);

    expect(screen.getByTestId(testIds.queryEditor.database)).toHaveValue('');
    expect(screen.getByTestId(testIds.queryEditor.queryText)).toHaveValue('');
  });

  describe('name dropdowns', () => {
    it('offers the databases of the instance', async () => {
      const { datasource } = setup();
      await settled(datasource);

      await userEvent.click(screen.getByTestId(testIds.queryEditor.database));

      expect(await screen.findByRole('option', { name: 'admin' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'grafana' })).toBeInTheDocument();
    });

    it('reports the selected database', async () => {
      const { onChange, datasource } = setup();
      await settled(datasource);

      await pickOption(testIds.queryEditor.database, 'admin');

      expect(onChange).toHaveBeenCalledWith({ ...query, database: 'admin' });
    });

    it('lists the collections of the selected database', async () => {
      const { datasource } = setup();
      await settled(datasource);

      expect(datasource.getCollections).toHaveBeenCalledWith('grafana');
    });

    it('reports the selected collection', async () => {
      const { onChange, datasource } = setup();
      await settled(datasource);

      await pickOption(testIds.queryEditor.collection, 'logs');

      expect(onChange).toHaveBeenCalledWith({ ...query, collection: 'logs' });
    });

    it('lists the fields of the selected collection', async () => {
      const { datasource } = setup();
      await settled(datasource);

      expect(datasource.getFields).toHaveBeenCalledWith('grafana', 'fruits');
    });

    it('reports the selected timestamp field', async () => {
      const { onChange, datasource } = setup();
      await settled(datasource);

      await pickOption(testIds.queryEditor.timestampField, 'timestamp');

      expect(onChange).toHaveBeenCalledWith({ ...query, timestampField: 'timestamp' });
    });

    // MongoDB creates a collection on write and a user may not be allowed to
    // list anything, so an unknown name has to stay usable.
    it('accepts a name that is not in the list', async () => {
      // Starting without a collection, so that what is typed is the whole value.
      const { onChange, datasource } = setup({ collection: undefined });
      await settled(datasource);

      const input = screen.getByTestId(testIds.queryEditor.collection);
      await userEvent.click(input);
      await userEvent.type(input, 'brand_new{Enter}');

      expect(onChange).toHaveBeenCalledWith({ ...query, collection: 'brand_new' });
    });

    it('keeps working when the names cannot be listed', async () => {
      const datasource = makeDatasource({
        getDatabases: jest.fn().mockRejectedValue({ data: { error: 'not authorized' } }),
      });
      setup({}, datasource);
      await settled(datasource);

      // The editor still renders, and the reason is offered in the dropdown.
      await userEvent.click(screen.getByTestId(testIds.queryEditor.database));
      expect(await screen.findByText(/not authorized/)).toBeInTheDocument();
    });
  });

  describe('query type', () => {
    it('offers every supported operation', () => {
      setup();

      for (const label of ['Find', 'Aggregate', 'Count', 'Distinct']) {
        expect(screen.getByLabelText(label)).toBeInTheDocument();
      }
    });

    it('treats a query without a type as a find', () => {
      setup({ queryType: undefined });

      expect(screen.getByLabelText('Find')).toBeChecked();
      expect(screen.getByTestId(testIds.queryEditor.queryText)).toBeInTheDocument();
    });

    it('reports a change of the query type', async () => {
      const { onChange } = setup();

      await userEvent.click(screen.getByLabelText('Aggregate'));

      expect(onChange).toHaveBeenCalledWith({ ...query, queryType: 'aggregate' });
    });
  });

  describe('find', () => {
    it('shows the projection, sort, limit and skip options', () => {
      setup({ queryType: 'find' });

      expect(screen.getByTestId(testIds.queryEditor.projection)).toBeInTheDocument();
      expect(screen.getByTestId(testIds.queryEditor.sort)).toBeInTheDocument();
      expect(screen.getByTestId(testIds.queryEditor.limit)).toBeInTheDocument();
      expect(screen.getByTestId(testIds.queryEditor.skip)).toBeInTheDocument();
    });

    it.each([
      ['projection', testIds.queryEditor.projection, '{"name": 1}'],
      ['sort', testIds.queryEditor.sort, '{"value": -1}'],
    ])('reports a change of the %s option', (field, testId, value) => {
      const { onChange } = setup({ queryType: 'find' });

      fireEvent.change(screen.getByTestId(testId), { target: { value } });

      expect(onChange).toHaveBeenCalledWith({ ...query, [field]: value });
    });

    it.each([
      ['limit', testIds.queryEditor.limit],
      ['skip', testIds.queryEditor.skip],
    ])('reports the %s as a number', (field, testId) => {
      const { onChange } = setup({ queryType: 'find' });

      fireEvent.change(screen.getByTestId(testId), { target: { value: '25' } });

      expect(onChange).toHaveBeenCalledWith({ ...query, [field]: 25 });
    });

    it.each([
      ['limit', testIds.queryEditor.limit],
      ['skip', testIds.queryEditor.skip],
    ])('clears the %s when the input is emptied', (field, testId) => {
      const { onChange } = setup({ queryType: 'find', [field]: 10 });

      fireEvent.change(screen.getByTestId(testId), { target: { value: '' } });

      expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ [field]: undefined }));
    });
  });

  describe('aggregate', () => {
    it('swaps the filter for a pipeline', () => {
      setup({ queryType: 'aggregate', pipeline: '[{"$match": {}}]' });

      expect(screen.getByTestId(testIds.queryEditor.pipeline)).toHaveValue('[{"$match": {}}]');
      expect(screen.queryByTestId(testIds.queryEditor.queryText)).not.toBeInTheDocument();
    });

    it('hides the find only options', () => {
      setup({ queryType: 'aggregate' });

      expect(screen.queryByTestId(testIds.queryEditor.projection)).not.toBeInTheDocument();
      expect(screen.queryByTestId(testIds.queryEditor.sort)).not.toBeInTheDocument();
      expect(screen.queryByTestId(testIds.queryEditor.limit)).not.toBeInTheDocument();
    });

    it('reports a change of the pipeline', () => {
      const { onChange } = setup({ queryType: 'aggregate' });

      fireEvent.change(screen.getByTestId(testIds.queryEditor.pipeline), {
        target: { value: '[{"$count": "total"}]' },
      });

      expect(onChange).toHaveBeenCalledWith({ ...query, queryType: 'aggregate', pipeline: '[{"$count": "total"}]' });
    });

    // Switching type must not throw away what was typed for the other one.
    it('keeps the filter and the pipeline apart', () => {
      setup({ queryType: 'aggregate', queryText: '{"level": "info"}', pipeline: '[{"$match": {}}]' });

      expect(screen.getByTestId(testIds.queryEditor.pipeline)).toHaveValue('[{"$match": {}}]');
    });
  });

  describe('count', () => {
    it('keeps the filter but hides the find only options', () => {
      setup({ queryType: 'count' });

      expect(screen.getByTestId(testIds.queryEditor.queryText)).toBeInTheDocument();
      expect(screen.queryByTestId(testIds.queryEditor.projection)).not.toBeInTheDocument();
      expect(screen.queryByTestId(testIds.queryEditor.distinctField)).not.toBeInTheDocument();
    });
  });

  describe('distinct', () => {
    it('asks for the field to collect', async () => {
      const { datasource } = setup({ queryType: 'distinct', distinctField: 'level' });
      await settled(datasource);

      expect(screen.getByTestId(testIds.queryEditor.distinctField)).toHaveValue('level');
      expect(screen.getByTestId(testIds.queryEditor.queryText)).toBeInTheDocument();
    });

    it('reports the selected field', async () => {
      const { onChange, datasource } = setup({ queryType: 'distinct' });
      await settled(datasource);

      await pickOption(testIds.queryEditor.distinctField, 'level');

      expect(onChange).toHaveBeenCalledWith({ ...query, queryType: 'distinct', distinctField: 'level' });
    });

    it('flags a missing field', async () => {
      const { datasource } = setup({ queryType: 'distinct' });
      await settled(datasource);

      expect(screen.getByText('A field is required for a distinct query')).toBeInTheDocument();
    });
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
