# Metrics

Tendermint can report and serve the Prometheus metrics, which in their turn can
be consumed by Prometheus collector(s).

This functionality is disabled by default.

To enable the Prometheus metrics, set `instrumentation.prometheus=true` if your
config file. Metrics will be served under `/metrics` on 26660 port by default.
Listen address can be changed in the config file (see
`instrumentation.prometheus_listen_addr`).

## List of available metrics

The following metrics are available:

```
| Name                                    | Type      | Since     | Description                                                                   |
| --------------------------------------- | -------   | --------- | ----------------------------------------------------------------------------- |
| consensus_height                        | Gauge     | 0.21.0    | Height of the chain                                                           |
| consensus_validators                    | Gauge     | 0.21.0    | Number of validators                                                          |
| consensus_validators_power              | Gauge     | 0.21.0    | Total voting power of all validators                                          |
| consensus_missing_validators            | Gauge     | 0.21.0    | Number of validators who did not sign                                         |
| consensus_missing_validators_power      | Gauge     | 0.21.0    | Total voting power of the missing validators                                  |
| consensus_byzantine_validators          | Gauge     | 0.21.0    | Number of validators who tried to double sign                                 |
| consensus_byzantine_validators_power    | Gauge     | 0.21.0    | Total voting power of the byzantine validators                                |
| consensus_block_interval_seconds        | Histogram | 0.21.0    | Time between this and last block (Block.Header.Time) in seconds               |
| consensus_rounds                        | Gauge     | 0.21.0    | Number of rounds                                                              |
| consensus_num_txs                       | Gauge     | 0.21.0    | Number of transactions                                                        |
| mempool_size                            | Gauge     | 0.21.0    | Number of uncommitted transactions                                            |
| consensus_total_txs                     | Gauge     | 0.21.0    | Total number of transactions committed                                        |
| consensus_block_size_bytes              | Gauge     | 0.21.0    | Block size in bytes                                                           |
| p2p_peers                               | Gauge     | 0.21.0    | Number of peers node's connected to                                           |
```

## Useful queries

Percentage of missing + byzantine validators:

```
((consensus_byzantine_validators_power + consensus_missing_validators_power) / consensus_validators_power) * 100
```
