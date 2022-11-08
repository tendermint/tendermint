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

| **Name**                                 | **Type**  | **Tags**        | **Description**                                                                                                                            |
|------------------------------------------|-----------|-----------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| `abci_connection_method_timing_seconds`  | Histogram | method, type    | Timings for each of the ABCI methods                                                                                                       |
| `consensus_height`                       | Gauge     |                 | Height of the chain                                                                                                                        |
| `consensus_validators`                   | Gauge     |                 | Number of validators                                                                                                                       |
| `consensus_validators_power`             | Gauge     |                 | Total voting power of all validators                                                                                                       |
| `consensus_validator_power`              | Gauge     |                 | Voting power of the node if in the validator set                                                                                           |
| `consensus_validator_last_signed_height` | Gauge     |                 | Last height the node signed a block, if the node is a validator                                                                            |
| `consensus_validator_missed_blocks`      | Gauge     |                 | Total amount of blocks missed for the node, if the node is a validator                                                                     |
| `consensus_missing_validators`           | Gauge     |                 | Number of validators who did not sign                                                                                                      |
| `consensus_missing_validators_power`     | Gauge     |                 | Total voting power of the missing validators                                                                                               |
| `consensus_byzantine_validators`         | Gauge     |                 | Number of validators who tried to double sign                                                                                              |
| `consensus_byzantine_validators_power`   | Gauge     |                 | Total voting power of the byzantine validators                                                                                             |
| `consensus_block_interval_seconds`       | Histogram |                 | Time between this and last block (Block.Header.Time) in seconds                                                                            |
| `consensus_rounds`                       | Gauge     |                 | Number of rounds                                                                                                                           |
| `consensus_num_txs`                      | Gauge     |                 | Number of transactions                                                                                                                     |
| `consensus_total_txs`                    | Gauge     |                 | Total number of transactions committed                                                                                                     |
| `consensus_block_parts`                  | counter   | peer_id         | number of blockparts transmitted by peer                                                                                                   |
| `consensus_latest_block_height`          | gauge     |                 | /status sync_info number                                                                                                                   |
| `consensus_block_syncing`                | gauge     |                 | either 0 (not block syncing) or 1 (syncing)                                                                                                 |
| `consensus_state_syncing`                | gauge     |                 | either 0 (not state syncing) or 1 (syncing)                                                                                                |
| `consensus_block_size_bytes`             | Gauge     |                 | Block size in bytes                                                                                                                        |
| `consensus_step_duration`                | Histogram | step            | Histogram of durations for each step in the consensus protocol                                                                             |
| `consensus_round_duration`               | Histogram |                 | Histogram of durations for all the rounds that have occurred since the process started                                                     |
| `consensus_block_gossip_parts_received`  | Counter   | matches_current | Number of block parts received by the node                                                                                                 |
| `consensus_quorum_prevote_delay`         | Gauge     |                 | Interval in seconds between the proposal timestamp and the timestamp of the earliest prevote that achieved a quorum                        |
| `consensus_full_prevote_delay`           | Gauge     |                 | Interval in seconds between the proposal timestamp and the timestamp of the latest prevote in a round where all validators voted           |
| `consensus_proposal_receive_count`       | Counter   | status          | Total number of proposals received by the node since process start                                                                         |
| `consensus_proposal_create_count`        | Counter   |                 | Total number of proposals created by the node since process start                                                                          |
| `consensus_round_voting_power_percent`   | Gauge     | vote_type       | A value between 0 and 1.0 representing the percentage of the total voting power per vote type received within a round                      |
| `consensus_late_votes`                   | Counter   | vote_type       | Number of votes received by the node since process start that correspond to earlier heights and rounds than this node is currently in.     |
| `p2p_peers`                              | Gauge     |                 | Number of peers node's connected to                                                                                                        |
| `p2p_peer_receive_bytes_total`           | counter   | peer_id, chID   | number of bytes per channel received from a given peer                                                                                     |
| `p2p_peer_send_bytes_total`              | counter   | peer_id, chID   | number of bytes per channel sent to a given peer                                                                                           |
| `p2p_peer_pending_send_bytes`            | gauge     | peer_id         | number of pending bytes to be sent to a given peer                                                                                         |
| `p2p_num_txs`                            | gauge     | peer_id         | number of transactions submitted by each peer_id                                                                                           |
| `p2p_pending_send_bytes`                 | gauge     | peer_id         | amount of data pending to be sent to peer                                                                                                  |
| `mempool_size`                           | Gauge     |                 | Number of uncommitted transactions                                                                                                         |
| `mempool_tx_size_bytes`                  | histogram |                 | transaction sizes in bytes                                                                                                                 |
| `mempool_failed_txs`                     | counter   |                 | number of failed transactions                                                                                                              |
| `mempool_recheck_times`                  | counter   |                 | number of transactions rechecked in the mempool                                                                                            |
| `state_block_processing_time`            | histogram |                 | time between BeginBlock and EndBlock in ms                                                                                                 |
| `state_consensus_param_updates`          | Counter   |                 | number of consensus parameter updates returned by the application since process start                                                      |
| `state_validator_set_updates`            | Counter   |                 | number of validator set updates returned by the application since process start                                                            |

## Useful queries

Percentage of missing + byzantine validators:

```md
((consensus\_byzantine\_validators\_power + consensus\_missing\_validators\_power) / consensus\_validators\_power) * 100
```
