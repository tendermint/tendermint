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

| **Name**                                | **Type**  | **Since** | **Tags**       | **Description**                                                 |
|-----------------------------------------|-----------|-----------|----------------|-----------------------------------------------------------------|
| consensus\_height                       | Gauge     | 0.21.0    |                | Height of the chain                                             |
| consensus\_validators                   | Gauge     | 0.21.0    |                | Number of validators                                            |
| consensus\_validators\_power            | Gauge     | 0.21.0    |                | Total voting power of all validators                            |
| consensus\_missing\_validators          | Gauge     | 0.21.0    |                | Number of validators who did not sign                           |
| consensus\_missing\_validators\_power   | Gauge     | 0.21.0    |                | Total voting power of the missing validators                    |
| consensus\_byzantine\_validators        | Gauge     | 0.21.0    |                | Number of validators who tried to double sign                   |
| consensus\_byzantine\_validators\_power | Gauge     | 0.21.0    |                | Total voting power of the byzantine validators                  |
| consensus\_block\_interval\_seconds     | Histogram | 0.21.0    |                | Time between this and last block (Block.Header.Time) in seconds |
| consensus\_rounds                       | Gauge     | 0.21.0    |                | Number of rounds                                                |
| consensus\_num\_txs                     | Gauge     | 0.21.0    |                | Number of transactions                                          |
| consensus\_block\_parts                 | counter   | on dev    | peer\_id       | number of blockparts transmitted by peer                        |
| consensus\_latest\_block\_height        | gauge     | on dev    |                | /status sync\_info number                                       |
| consensus\_fast\_syncing                | gauge     | on dev    |                | either 0 (not fast syncing) or 1 (syncing)                      |
| consensus\_total\_txs                   | Gauge     | 0.21.0    |                | Total number of transactions committed                          |
| consensus\_block\_size\_bytes           | Gauge     | 0.21.0    |                | Block size in bytes                                             |
| p2p\_peers                              | Gauge     | 0.21.0    |                | Number of peers node's connected to                             |
| p2p\_peer\_receive\_bytes\_total        | counter   | on dev    | peer\_id, chID | number of bytes per channel received from a given peer          |
| p2p\_peer\_send\_bytes\_total           | counter   | on dev    | peer\_id, chID | number of bytes per channel sent to a given peer                |
| p2p\_peer\_pending\_send\_bytes         | gauge     | on dev    | peer\_id       | number of pending bytes to be sent to a given peer              |
| p2p\_num\_txs                           | gauge     | on dev    | peer\_id       | number of transactions submitted by each peer\_id               |
| p2p\_pending\_send\_bytes               | gauge     | on dev    | peer\_id       | amount of data pending to be sent to peer                       |
| mempool\_size                           | Gauge     | 0.21.0    |                | Number of uncommitted transactions                              |
| mempool\_tx\_size\_bytes                | histogram | on dev    |                | transaction sizes in bytes                                      |
| mempool\_failed\_txs                    | counter   | on dev    |                | number of failed transactions                                   |
| mempool\_recheck\_times                 | counter   | on dev    |                | number of transactions rechecked in the mempool                 |
| state\_block\_processing\_time          | histogram | on dev    |                | time between BeginBlock and EndBlock in ms                      |

## Useful queries

Percentage of missing + byzantine validators:

```
((consensus\_byzantine\_validators\_power + consensus\_missing\_validators\_power) / consensus\_validators\_power) * 100
```
