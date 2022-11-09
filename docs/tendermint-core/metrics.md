---
order: 5
---

# Metrics

Tendermint can report and serve the Prometheus metrics, which in their turn can
be consumed by Prometheus collector(s).

This functionality is disabled by default.

To enable the Prometheus metrics, set `instrumentation.prometheus=true` in your
config file. Metrics will be served under `/metrics` on 26660 port by default.
Listen address can be changed in the config file (see
`instrumentation.prometheus\_listen\_addr`).

## List of available metrics

The following metrics are available:

| **Name**                                 | **Type**  | **Tags**          | **Description**                                                        |
|------------------------------------------|-----------|-------------------|------------------------------------------------------------------------|
| `consensus_height`                       | Gauge     |                   | Height of the chain                                                    |
| `consensus_validators`                   | Gauge     |                   | Number of validators                                                   |
| `consensus_validators_power`             | Gauge     |                   | Total voting power of all validators                                   |
| `consensus_validator_power`              | Gauge     |                   | Voting power of the node if in the validator set                       |
| `consensus_validator_last_signed_height` | Gauge     |                   | Last height the node signed a block, if the node is a validator        |
| `consensus_validator_missed_blocks`      | Gauge     |                   | Total amount of blocks missed for the node, if the node is a validator |
| `consensus_missing_validators`           | Gauge     |                   | Number of validators who did not sign                                  |
| `consensus_missing_validators_power`     | Gauge     |                   | Total voting power of the missing validators                           |
| `consensus_byzantine_validators`         | Gauge     |                   | Number of validators who tried to double sign                          |
| `consensus_byzantine_validators_power`   | Gauge     |                   | Total voting power of the byzantine validators                         |
| `consensus_block_interval_seconds`       | Histogram |                   | Time between this and last block (Block.Header.Time) in seconds        |
| `consensus_rounds`                       | Gauge     |                   | Number of rounds                                                       |
| `consensus_num_txs`                      | Gauge     |                   | Number of transactions                                                 |
| `consensus_total_txs`                    | Gauge     |                   | Total number of transactions committed                                 |
| `consensus_block_parts`                  | Counter   | `peer_id`         | Number of blockparts transmitted by peer                               |
| `consensus_latest_block_height`          | Gauge     |                   | /status sync\_info number                                              |
| `consensus_fast_syncing`                 | Gauge     |                   | Either 0 (not fast syncing) or 1 (syncing)                             |
| `consensus_state_syncing`                | Gauge     |                   | Either 0 (not state syncing) or 1 (syncing)                            |
| `consensus_block_size_bytes`             | Gauge     |                   | Block size in bytes                                                    |
| `p2p_message_send_bytes_total`           | Counter   | `message_type`    | Number of bytes sent to all peers per message type                     |
| `p2p_message_receive_bytes_total`        | Counter   | `message_type`    | Number of bytes received from all peers per message type               |
| `p2p_peers`                              | Gauge     |                   | Number of peers node's connected to                                    |
| `p2p_peer_receive_bytes_total`           | Counter   | `peer_id`, `chID` | Number of bytes per channel received from a given peer                 |
| `p2p_peer_send_bytes_total`              | Counter   | `peer_id`, `chID` | Number of bytes per channel sent to a given peer                       |
| `p2p_peer_pending_send_bytes`            | Gauge     | `peer_id`         | Number of pending bytes to be sent to a given peer                     |
| `p2p_num_txs`                            | Gauge     | `peer_id`         | Number of transactions submitted by each peer\_id                      |
| `p2p_pending_send_bytes`                 | Gauge     | `peer_id`         | Amount of data pending to be sent to peer                              |
| `mempool_size`                           | Gauge     |                   | Number of uncommitted transactions                                     |
| `mempool_tx_size_bytes`                  | Histogram |                   | Transaction sizes in bytes                                             |
| `mempool_failed_txs`                     | Counter   |                   | Number of failed transactions                                          |
| `mempool_recheck_times`                  | Counter   |                   | Number of transactions rechecked in the mempool                        |
| `state_block_processing_time`            | Histogram |                   | Time between BeginBlock and EndBlock in ms                             |

## Useful queries

Percentage of missing + byzantine validators:

```md
((consensus\_byzantine\_validators\_power + consensus\_missing\_validators\_power) / consensus\_validators\_power) * 100
```
