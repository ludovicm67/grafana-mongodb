import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { DataSourceSettings } from '@grafana/data';
import { ConfigEditor } from './ConfigEditor';
import { testIds } from '../testIds';
import type { MongoDataSourceOptions, MongoSecureJsonData } from '../types';

type Settings = DataSourceSettings<MongoDataSourceOptions, MongoSecureJsonData>;

type Options = {
  jsonData?: MongoDataSourceOptions;
  secureJsonData?: MongoSecureJsonData;
  secureJsonFields?: Record<string, boolean>;
};

function setup({ jsonData = {}, secureJsonData = {}, secureJsonFields = {} }: Options = {}) {
  const onOptionsChange = jest.fn();
  // The editor only reads these three keys out of the datasource settings.
  const options = { jsonData, secureJsonData, secureJsonFields } as Settings;

  render(<ConfigEditor options={options} onOptionsChange={onOptionsChange} />);

  return { onOptionsChange, options };
}

describe('ConfigEditor', () => {
  it('renders the stored settings', () => {
    setup({ jsonData: { uri: 'mongodb://localhost:27017', username: 'admin' } });

    expect(screen.getByTestId(testIds.configEditor.uri)).toHaveValue('mongodb://localhost:27017');
    expect(screen.getByTestId(testIds.configEditor.username)).toHaveValue('admin');
  });

  it('reports a change of the URI', () => {
    const { onOptionsChange, options } = setup();

    fireEvent.change(screen.getByTestId(testIds.configEditor.uri), {
      target: { value: 'mongodb://mongodb:27017' },
    });

    expect(onOptionsChange).toHaveBeenCalledWith({
      ...options,
      jsonData: { uri: 'mongodb://mongodb:27017' },
    });
  });

  it('reports a change of the username', () => {
    const { onOptionsChange, options } = setup({ jsonData: { uri: 'mongodb://mongodb:27017' } });

    fireEvent.change(screen.getByTestId(testIds.configEditor.username), { target: { value: 'root' } });

    expect(onOptionsChange).toHaveBeenCalledWith({
      ...options,
      jsonData: { uri: 'mongodb://mongodb:27017', username: 'root' },
    });
  });

  it('keeps the password out of the plain settings', () => {
    const { onOptionsChange } = setup();

    fireEvent.change(screen.getByTestId(testIds.configEditor.password), { target: { value: 'hunter2' } });

    const [updated] = onOptionsChange.mock.calls[0];
    expect(updated.secureJsonData).toEqual({ password: 'hunter2' });
    expect(updated.jsonData).not.toHaveProperty('password');
  });

  it('hides an already configured password and allows resetting it', async () => {
    const { onOptionsChange } = setup({ secureJsonFields: { password: true } });

    // A configured secret is not sent back to the frontend, the field shows a
    // placeholder and a reset button instead.
    await userEvent.click(screen.getByRole('button', { name: /reset/i }));

    const [updated] = onOptionsChange.mock.calls[0];
    expect(updated.secureJsonFields.password).toBe(false);
    expect(updated.secureJsonData.password).toBe('');
  });
});
