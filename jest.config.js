// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';

const { grafanaESModules, nodeModulesToTransform } = require('./.config/jest/utils');

// The @grafana/* 13.2 packages depend on @react-hookz/web and @ver0/deep-equal, which
// only ship an ES module build. They are already part of the list in the create-plugin
// repository, but not in the scaffolding released so far. See "ESM errors with Jest"
// in .config/README.md.
const extraESModules = ['@react-hookz/web', '@ver0/deep-equal'];

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...require('./.config/jest.config'),
  transformIgnorePatterns: [nodeModulesToTransform([...grafanaESModules, ...extraESModules])],
};
