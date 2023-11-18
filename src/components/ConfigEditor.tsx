import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;

  const onUriChange = (event: ChangeEvent<HTMLInputElement>) => {
    const jsonData = {
      ...options.jsonData,
      uri: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  const onUsernameChange = (event: ChangeEvent<HTMLInputElement>) => {
    const jsonData = {
      ...options.jsonData,
      username: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  const onDatabaseChange = (event: ChangeEvent<HTMLInputElement>) => {
    const jsonData = {
      ...options.jsonData,
      database: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  // Secure field (only sent to the backend)
  const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        password: event.target.value,
      },
    });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        password: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        password: '',
      },
    });
  };

  const { jsonData, secureJsonFields } = options;
  const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;

  return (
    <div className="gf-form-group">
      <InlineField label="MongoDB URI" labelWidth={16}>
        <Input onChange={onUriChange} value={jsonData.uri || ''} placeholder="mongodb://localhost:27017" width={40} />
      </InlineField>
      <InlineField label="Database" labelWidth={16}>
        <Input onChange={onDatabaseChange} value={jsonData.database || ''} placeholder="db" width={40} />
      </InlineField>
      <InlineField label="Username" labelWidth={16}>
        <Input onChange={onUsernameChange} value={jsonData.username || ''} placeholder="admin" width={40} />
      </InlineField>
      <InlineField label="Password" labelWidth={16}>
        <SecretInput
          isConfigured={(secureJsonFields && secureJsonFields.password) as boolean}
          onReset={onResetPassword}
          onChange={onPasswordChange}
          value={secureJsonData.password || ''}
          placeholder="example"
          width={40}
          autoComplete="new-password"
        />
      </InlineField>
    </div>
  );
}
