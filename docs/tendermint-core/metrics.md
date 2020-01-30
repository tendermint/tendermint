---
order: 11
---

# Metrics

Tendermint can report and serve the Prometheus metrics, which in their turn can
be consumed by Prometheus collector(s).

This functionality is disabled by default.

To enable the Prometheus metrics, set `instrumentation.prometheus=true` if your
config file. Metrics will be served under `/metrics` on 26660 port by default.
Listen address can be changed in the config file (see
`instrumentation.prometheus\_listen\_addr`).

## List of available metrics

The following metrics are available:

| **Name**                               | **Type**  | **Since** | **Tags**      | **Description**                                                        |
| -------------------------------------- | --------- | --------- | ------------- | ---------------------------------------------------------------------- |
| consensus_height                       | Gauge     | 0.21.0    |               | Height of the chain                                                    |
| consensus_validators                   | Gauge     | 0.21.0    |               | Number of validators                                                   |
| consensus_validators_power             | Gauge     | 0.21.0    |               | Total voting power of all validators                                   |
| consensus_validator_power              | Gauge     | 0.33.0    |               | Voting power of the node if in the validator set                       |
| consensus_validator_last_signed_height | Gauge     | 0.33.0    |               | Last height the node signed a block, if the node is a validator        |
| consensus_validator_missed_blocks      | Gauge     | 0.33.0    |               | Total amount of blocks missed for the node, if the node is a validator |
| consensus_missing_validators           | Gauge     | 0.21.0    |               | Number of validators who did not sign                                  |
| consensus_missing_validators_power     | Gauge     | 0.21.0    |               | Total voting power of the missing validators                           |
| consensus_byzantine_validators         | Gauge     | 0.21.0    |               | Number of validators who tried to double sign                          |
| consensus_byzantine_validators_power   | Gauge     | 0.21.0    |               | Total voting power of the byzantine validators                         |
| consensus_block_interval_seconds       | Histogram | 0.21.0    |               | Time between this and last block (Block.Header.Time) in seconds        |
| consensus_rounds                       | Gauge     | 0.21.0    |               | Number of rounds                                                       |
| consensus_num_txs                      | Gauge     | 0.21.0    |               | Number of transactions                                                 |
| consensus_total_txs                    | Gauge     | 0.21.0    |               | Total number of transactions committed                                 |
| consensus_block_parts                  | counter   | 0.25.0    | peer_id       | number of blockparts transmitted by peer                               |
| consensus_latest_block_height          | gauge     | 0.25.0    |               | /status sync_info number                                               |
| consensus_fast_syncing                 | gauge     | 0.25.0    |               | either 0 (not fast syncing) or 1 (syncing)                             |
| consensus_block_size_bytes             | Gauge     | 0.21.0    |               | Block size in bytes                                                    |
| p2p_peers                              | Gauge     | 0.21.0    |               | Number of peers node's connected to                                    |
| p2p_peer_receive_bytes_total           | counter   | 0.25.0    | peer_id, chID | number of bytes per channel received from a given peer                 |
| p2p_peer_send_bytes_total              | counter   | 0.25.0    | peer_id, chID | number of bytes per channel sent to a given peer                       |
| p2p_peer_pending_send_bytes            | gauge     | 0.25.0    | peer_id       | number of pending bytes to be sent to a given peer                     |
| p2p_num_txs                            | gauge     | 0.25.0    | peer_id       | number of transactions submitted by each peer_id                       |
| p2p_pending_send_bytes                 | gauge     | 0.25.0    | peer_id       | amount of data pending to be sent to peer                              |
| mempool_size                           | Gauge     | 0.21.0    |               | Number of uncommitted transactions                                     |
| mempool_tx_size_bytes                  | histogram | 0.25.0    |               | transaction sizes in bytes                                             |
| mempool_failed_txs                     | counter   | 0.25.0    |               | number of failed transactions                                          |
| mempool_recheck_times                  | counter   | 0.25.0    |               | number of transactions rechecked in the mempool                        |
| state_block_processing_time            | histogram | 0.25.0    |               | time between BeginBlock and EndBlock in ms                             |

## Useful queries

Percentage of missing + byzantine validators:

```
((consensus\_byzantine\_validators\_power + consensus\_missing\_validators\_power) / consensus\_validators\_power) * 100
```
