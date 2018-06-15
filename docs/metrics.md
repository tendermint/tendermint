# Metrics

Tendermint can serve metrics under `/metrics` RPC endpoint, which can be
consumed by Prometheus server.

This functionality is disabled by default. Set `monitoring=true` if your config
to enable it.

## List of available metrics

| Name                                    | Type    | Description                                                                   |
| --------------------------------------- | ------- | ----------------------------------------------------------------------------- |
| consensus_height                        | Gauge   | Height of the chain                                                           |
| consensus_validators                    | Gauge   | Number of validators                                                          |
| consensus_validators_power              | Gauge   | Total voting power of all validators                                          |
| consensus_missing_validators            | Gauge   | Number of validators who did not sign                                         |
| consensus_missing_validators_power      | Gauge   | Total voting power of the missing validators                                  |
| consensus_byzantine_validators          | Gauge   | Number of validators who tried to double sign                                 |
| consensus_byzantine_validators_power    | Gauge   | Total voting power of the byzantine validators                                |
| consensus_block_interval                | Timing  | Time between this and last block (Block.Header.Time)                          |
| consensus_rounds                        | Gauge   | Number of rounds                                                              |
| consensus_num_txs                       | Gauge   | Number of transactions                                                        |
| mempool_size                            | Gauge   | Number of uncommitted transactions                                            |
| consensus_total_txs                     | Gauge   | Total number of transactions committed                                        |
| consensus_block_size                    | Gauge   | Block size in bytes                                                           |
| p2p_peers                               | Gauge   | Number of peers node's connected to                                           |

## Useful queries

Percentage of missing + byzantine validators:

```
((consensus_byzantine_validators_power + consensus_missing_validators_power) / consensus_validators_power) * 100
```
