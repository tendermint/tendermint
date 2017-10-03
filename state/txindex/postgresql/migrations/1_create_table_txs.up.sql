CREATE TABLE txs (
  id serial PRIMARY KEY,
  hash VARCHAR NOT NULL,
  height BIGINT NOT NULL,
  index INTEGER NOT NULL,
  tx BYTEA NOT NULL,
  result_data BYTEA NOT NULL,
  result_code SMALLINT NOT NULL,
  result_log VARCHAR NOT NULL DEFAULT ''
);
