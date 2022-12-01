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

<<<<<<< HEAD
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
=======
| **Name**                                   | **Type**  | **Tags**         | **Description**                                                                                                                            |
|--------------------------------------------|-----------|------------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| abci\_connection\_method\_timing\_seconds  | Histogram | method, type     | Timings for each of the ABCI methods                                                                                                       |
| blocksync\_syncing                         | Gauge     |                  | Either 0 (not block syncing) or 1 (syncing)                                                                                                |
| consensus\_height                          | Gauge     |                  | Height of the chain                                                                                                                        |
| consensus\_validators                      | Gauge     |                  | Number of validators                                                                                                                       |
| consensus\_validators\_power               | Gauge     |                  | Total voting power of all validators                                                                                                       |
| consensus\_validator\_power                | Gauge     |                  | Voting power of the node if in the validator set                                                                                           |
| consensus\_validator\_last\_signed\_height | Gauge     |                  | Last height the node signed a block, if the node is a validator                                                                            |
| consensus\_validator\_missed\_blocks       | Gauge     |                  | Total amount of blocks missed for the node, if the node is a validator                                                                     |
| consensus\_missing\_validators             | Gauge     |                  | Number of validators who did not sign                                                                                                      |
| consensus\_missing\_validators\_power      | Gauge     |                  | Total voting power of the missing validators                                                                                               |
| consensus\_byzantine\_validators           | Gauge     |                  | Number of validators who tried to double sign                                                                                              |
| consensus\_byzantine\_validators\_power    | Gauge     |                  | Total voting power of the byzantine validators                                                                                             |
| consensus\_block\_interval\_seconds        | Histogram |                  | Time between this and last block (Block.Header.Time) in seconds                                                                            |
| consensus\_rounds                          | Gauge     |                  | Number of rounds                                                                                                                           |
| consensus\_num\_txs                        | Gauge     |                  | Number of transactions                                                                                                                     |
| consensus\_total\_txs                      | Gauge     |                  | Total number of transactions committed                                                                                                     |
| consensus\_block\_parts                    | Counter   | peer\_id         | Number of blockparts transmitted by peer                                                                                                   |
| consensus\_latest\_block\_height           | Gauge     |                  | /status sync\_info number                                                                                                                  |
| consensus\_block\_size\_bytes              | Gauge     |                  | Block size in bytes                                                                                                                        |
| consensus\_step\_duration                  | Histogram | step             | Histogram of durations for each step in the consensus protocol                                                                             |
| consensus\_round\_duration                 | Histogram |                  | Histogram of durations for all the rounds that have occurred since the process started                                                     |
| consensus\_block\_gossip\_parts\_received  | Counter   | matches\_current | Number of block parts received by the node                                                                                                 |
| consensus\_quorum\_prevote\_delay          | Gauge     |                  | Interval in seconds between the proposal timestamp and the timestamp of the earliest prevote that achieved a quorum                        |
| consensus\_full\_prevote\_delay            | Gauge     |                  | Interval in seconds between the proposal timestamp and the timestamp of the latest prevote in a round where all validators voted           |
| consensus\_proposal\_receive\_count        | Counter   | status           | Total number of proposals received by the node since process start                                                                         |
| consensus\_proposal\_create\_count         | Counter   |                  | Total number of proposals created by the node since process start                                                                          |
| consensus\_round\_voting\_power\_percent   | Gauge     | vote\_type       | A value between 0 and 1.0 representing the percentage of the total voting power per vote type received within a round                      |
| consensus\_late\_votes                     | Counter   | vote\_type       | Number of votes received by the node since process start that correspond to earlier heights and rounds than this node is currently in.     |
| p2p\_message\_send\_bytes\_total           | Counter   | message\_type    | Number of bytes sent to all peers per message type                                                                                         |
| p2p\_message\_receive\_bytes\_total        | Counter   | message\_type    | Number of bytes received from all peers per message type                                                                                   |
| p2p\_peers                                 | Gauge     |                  | Number of peers node's connected to                                                                                                        |
| p2p\_peer\_receive\_bytes\_total           | Counter   | peer\_id, chID   | Number of bytes per channel received from a given peer                                                                                     |
| p2p\_peer\_send\_bytes\_total              | Counter   | peer\_id, chID   | Number of bytes per channel sent to a given peer                                                                                           |
| p2p\_peer\_pending\_send\_bytes            | Gauge     | peer\_id         | Number of pending bytes to be sent to a given peer                                                                                         |
| p2p\_num\_txs                              | Gauge     | peer\_id         | Number of transactions submitted by each peer\_id                                                                                          |
| p2p\_pending\_send\_bytes                  | Gauge     | peer\_id         | Amount of data pending to be sent to peer                                                                                                  |
| mempool\_size                              | Gauge     |                  | Number of uncommitted transactions                                                                                                         |
| mempool\_tx\_size\_bytes                   | Histogram |                  | Transaction sizes in bytes                                                                                                                 |
| mempool\_failed\_txs                       | Counter   |                  | Number of failed transactions                                                                                                              |
| mempool\_recheck\_times                    | Counter   |                  | Number of transactions rechecked in the mempool                                                                                            |
| state\_block\_processing\_time             | Histogram |                  | Time between BeginBlock and EndBlock in ms                                                                                                 |
| state\_consensus\_param\_updates           | Counter   |                  | Number of consensus parameter updates returned by the application since process start                                                      |
| state\_validator\_set\_updates             | Counter   |                  | Number of validator set updates returned by the application since process start                                                            |
| statesync\_syncing                         | Gauge     |                  | Either 0 (not state syncing) or 1 (syncing)                                                                                                |
>>>>>>> 17c94bb0d (docs: Fix metrics name rendering (#9695))

## Useful queries

Percentage of missing + byzantine validators:

```md
((consensus\_byzantine\_validators\_power + consensus\_missing\_validators\_power) / consensus\_validators\_power) * 100
```
