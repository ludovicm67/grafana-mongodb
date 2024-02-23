# Grafana MongoDB Data Source plugin

This plugin allows you to connect to a MongoDB database and visualize the data in Grafana.

## Installation

If you are using the official Grafana Docker image, you can install this plugin by configuring the following environment variable:

- "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=ludovicm67-mongodb-datasource"
- "GF_INSTALL_PLUGINS=${PATH_TO_ZIP_ARCHIVE};ludovicm67-mongodb-datasource"

where `${PATH_TO_ZIP_ARCHIVE}` is the path to the zip archive of the plugin.

You can browse the [latest releases of the plugin](https://github.com/ludovicm67/grafana-mongodb/releases) to find the link to the zip archive.

## Development

Run the following commands to get started:

- `npm install` to install frontend dependencies.
- `npm run dev` to build (and watch) the plugin frontend code.
- `mage -v` to build the plugin backend code. Rerun this command every time you edit your backend files.
- `docker-compose up` to start a grafana development server. Restart this command after each time you run mage to run your new backend code.
- Open http://localhost:3000 in your browser to create a dashboard to begin developing your plugin.
