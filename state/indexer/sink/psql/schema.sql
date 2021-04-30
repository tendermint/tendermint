CREATE TYPE block_event_type AS ENUM ('begin_block', 'end_block', '');
CREATE TABLE block_events (
    block_event_id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    type block_event_type
);
CREATE TABLE tx_results (
tx_result_id SERIAL PRIMARY KEY,
tx_result BYTEA NOT NULL
);
CREATE TABLE tx_events (
    tx_event_id SERIAL PRIMARY KEY,
    key VARCHAR NOT NULL,
    value VARCHAR NOT NULL,
    height INTEGER NOT NULL,
    hash VARCHAR NOT NULL,
    txid SERIAL,
    FOREIGN KEY (txid) 
        REFERENCES tx_results(tx_result_id)
        ON DELETE CASCADE
);
CREATE INDEX idx_block_events_key_value ON block_events(key, value);
CREATE INDEX idx_tx_events_key_value ON tx_events(key, value);
CREATE INDEX idx_tx_events_hash ON tx_events(hash);