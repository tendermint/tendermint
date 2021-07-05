CREATE TYPE block_event_type AS ENUM ('begin_block', 'end_block', '');
CREATE TABLE block_events (
    id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    type block_event_type,
    created_at TIMESTAMPTZ NOT NULL,
    chain_id VARCHAR NOT NULL,
    UNIQUE (key, height)
);
CREATE TABLE tx_results (
    id SERIAL PRIMARY KEY,
    tx_result BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tx_result)
);
CREATE TABLE tx_events (
    id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    hash VARCHAR NOT NULL,
    tx_result_id SERIAL,
    created_at TIMESTAMPTZ NOT NULL,
    chain_id VARCHAR NOT NULL,
    UNIQUE (hash, key),
    FOREIGN KEY (tx_result_id)
        REFERENCES tx_results(id)
        ON DELETE CASCADE
);
CREATE INDEX idx_block_events_key_value ON block_events(key, value);
CREATE INDEX idx_tx_events_key_value ON tx_events(key, value);
CREATE INDEX idx_tx_events_hash ON tx_events(hash);
