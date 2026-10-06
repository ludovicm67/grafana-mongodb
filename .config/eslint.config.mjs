import { defineConfig } from 'eslint/config';
import { fixupConfigRules } from '@eslint/compat';
import grafanaConfig from '@grafana/eslint-config';

export default defineConfig([
  // eslint-plugin-react 7.37.5 still calls context methods removed in ESLint 10
  // (e.g. context.getFilename()), so its rules are wrapped with the compat layer.
  ...fixupConfigRules(grafanaConfig),
  {
    rules: {
      'react/prop-types': 'off',
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],

    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
      },
    },

    rules: {
      '@typescript-eslint/no-deprecated': 'warn',
    },
  },
  {
    files: ['./tests/**/*'],

    rules: {
      'react-hooks/rules-of-hooks': 'off',
    },
  },
]);
