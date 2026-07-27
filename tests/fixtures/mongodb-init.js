/**
 * Seeds the MongoDB instance started by `docker compose up`.
 *
 * Scripts placed in /docker-entrypoint-initdb.d only run when the data
 * directory is empty, so this runs once per fresh container. It is used both by
 * the local development dashboards and by the Playwright end to end tests.
 */

/* global db, print */

const target = db.getSiblingDB('grafana');

// `fruits` holds a small, fixed set of documents with no timestamp. The end to
// end tests assert on it, so its content has to stay stable.
target.fruits.drop();
target.fruits.insertMany([
  { name: 'apple', color: 'red', quantity: 5 },
  { name: 'banana', color: 'yellow', quantity: 12 },
  { name: 'kiwi', color: 'green', quantity: 3 },
]);

// `logs` holds time series style documents, with the timestamp stored as UNIX
// milliseconds, which is the format the datasource expects for its
// "Timestamp Field" option. They are spread over the last two hours so that
// they show up in the default dashboard time range.
target.logs.drop();

const levels = ['info', 'warn', 'error'];
const now = Date.now();
const logs = [];

for (let i = 0; i < 120; i++) {
  logs.push({
    timestamp: now - i * 60 * 1000,
    level: levels[i % levels.length],
    message: 'Log message #' + i,
    value: Math.round(Math.sin(i / 6) * 100) / 10,
  });
}

// One document far enough in the past that it never falls inside a default
// dashboard time range. The end to end tests use it to check that the
// "Timestamp Field" option really filters on the selected range.
logs.push({
  timestamp: Date.UTC(2020, 0, 1),
  level: 'ancient',
  message: 'ancient log',
  value: 0,
});

target.logs.insertMany(logs);

print('Seeded ' + target.fruits.countDocuments() + ' fruits and ' + target.logs.countDocuments() + ' logs');
