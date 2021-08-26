DROP TABLE blocks CASCADE;
DROP TABLE tx_results CASCADE;
DROP TABLE events CASCADE;
DROP TABLE attributes CASCADE;
DELETE VIEW event_attributes CASCADE;
DELETE VIEW block_events CASCADE;
DELETE VIEW tx_events CASCADE;

\i schema.sql

-- Test data
INSERT INTO blocks (height, chain_id, created_at)
  VALUES (100, 'chain1', TIMESTAMP '2020-11-22 13:14:15') RETURNING rowid; -- 1
INSERT INTO blocks (height, chain_id, created_at)
  VALUES (100, 'chain2', TIMESTAMP '2021-04-02 14:14:14') RETURNING rowid; -- 2
INSERT INTO blocks (height, chain_id, created_at)
  VALUES (121, 'chain1', TIMESTAMP '2020-11-23 13:00:19') RETURNING rowid; -- 3

INSERT INTO tx_results (block_id, index, created_at, tx_hash, tx_result)
  VALUES (1, 4, TIMESTAMP '2020-11-22 13:14:17', 'DEADBEEF', E'\\x01c3');  -- 1
INSERT INTO tx_results (block_id, index, created_at, tx_hash, tx_result)
  VALUES (2, 1, TIMESTAMP '2021-04-03 14:15:17', 'FEEDBABE', E'\\x039a');  -- 2

-- Block events
INSERT INTO events (block_id, type) VALUES (1, 'carrot');    -- 1
INSERT INTO events (block_id, type) VALUES (1, 'tomato');    -- 2
INSERT INTO events (block_id, type) VALUES (2, 'starfruit'); -- 3
INSERT INTO events (block_id, type) VALUES (3, 'guava');     -- 4

INSERT INTO attributes (event_id, key, composite_key, value) VALUES
  (1, 'color', 'carrot.color', 'orange'),
  (2, 'color', 'tomato.color', 'red'),
  (3, 'color', 'starfruit.color', 'yellow'),
  (4, 'color', 'guava.color', 'pink');

-- Transaction events
INSERT INTO events (block_id, tx_id, type) VALUES
  (1, 1, 'mood'),    -- 5
  (1, 1, 'stripes'), -- 6
  (1, 1, 'tail'),    -- 7
  (1, 1, 'empty'),   -- 8, has no attributes
  (1, NULL, 'void'); -- 9, has no attributes
INSERT INTO attributes (event_id, key, composite_key, value) VALUES
  (5, 'kind', 'mood.kind', 'distracted'),
  (6, 'count', 'stripes.count', '35'),
  (7, 'length', 'tail.length', 'long');
