---
order: 1
parent:
  title: Tendermint Quality Assurance Results for v0.34.x
  description: This is a report on the results obtained when running v0.34.x on testnets
  order: 2
---

# 200 Node Testnet

## Finding the Saturation Point

The first goal when examining the results of the tests is identifying the saturation point.
The saturation point is a setup with a transaction load big enough to prevent the testnet
from being stable: the load runner tries to produce slightly more transactions than can
be processed by the testnet.

The following table summarizes the results for v0.34.x, for the different experiments
(extracted from file [`v034_report_tabbed.txt`](./v034_report_tabbed.txt)).

|        |  c=4 |  c=8 | c=16 |
| :---   | ---: | ---: | ---: |
| r=20   |  144 |  309 |  632 |
| r=200  | 1547 | 3195 | 5958 |
| r=400  | 3102 | 6110 | 8526 |
| r=800  | 6231 | 8224 | 8653 |
| r=1200 | 7978 | 8368 | 9087 |

The table shows the number of 1024-byte-long transactions that were produced by the load runner,
and processed by Tendermint, during the 90 seconds of the experiment's duration.
Each cell in the table refers to an experiment with a particular number of websocket connections
to a chosen validator (`c`), and the number of transactions per second that the load runner
tries to produce (`r`). Note that the overall load is $c \cdot r$.

We can see that the saturation point is a diagonal (given that the overall load is de product
of the rate and the number of connections) that spans cells

* `r=1200,c=4`
* `r=800,c=8`
* `r=400,c=16`

These experiments and all others below the diagonal they form have in common that the total
number of transactions processed is noticeably less than the product $c \cdot r \cdot 90$,
which is the expected number of transactions when the system is able to deal well with the
load.

At this point, we chose an experiment with an important load but below the saturation point,
in order to further study the performance of this release.
**The chosen experiment is `r=400,c=8`**.

## Examining latencies

The method described [here](../method.md) allows us to plot the latencies of transactions
for all experiments.

![all-latencies](./all.svg)

As we can see, even the experiments beyond the saturation diagonal managed to maintain
transaction latency within stable limits. Our interpretation for this is that
contention within Tendermint was propagated, via the websockets, to the load runner,
hence the latter could not produce the target load, but a fraction of it.

Further examination of the Prometheus data, showed that indeed the mempool, while big
was not growing without control. Finally, the test script made sure that, at the end
of an experiment, the mempool was empty.

This plot can we used as a baseline to compare with other releases.

## Prometheus Metrics on the Chosen Experiment

As mentioned [above](#finding-the-saturation-point), the chosen experiment is `r=400,c=8`.
This section further examines key metrics for this experiment extracted from Prometheus data.

### Mempool Size

The mempool size was stable and homogeneous at all full nodes.
The plot below shows the evolution over time of the cumulative number of transactions inside all full nodes' mempools.

![mempool-cumulative](./v034_r400c8_mempool_size.png)

The plot below shows evolution of the average over all full nodes, which seems to oscillate around 140 outstanding transactions.

![mempool-avg](./v034_r400c8_mempool_size_avg.png)

### Peers

The number of peers was stable at all nodes.
It was higher for the seed nodes (around 140) than for the rest (between 21 and 71).

![peers](./v034_r400c8_peers.png)

### Consensus Rounds per Height

Most heights took just one round, but some nodes needed to advance to round 1 at some point.

![rounds](./v034_r400c8_rounds.png)

### Blocks Produced per Minute

The blocks produced per minute are the gradient of this plot.

![heights](./v034_r400c8_heights.png)

Over a period of 2 minutes, the height goes from 967 to 1033.
This result in an average of 33 blocks produced per minute.

### Memory resident set size

Resident Set Size of all monitored processes is plotted below.

![rss](./v034_r400c8_rss.png)

The average over all processes oscillates around 230 MiB.

![rss-avg](./v034_r400c8_rss_avg.png)

### CPU utilization

The best metric from Prometheus to gauge CPU utilization is `load1`, as it usually appears in the output of `top`.

![load1](./v034_r400c8_load1.png)

It is contained between 0.3 and 4.2 at all nodes.

## Test Result

**Result: N/A**
Date: 2022-09-23
Version: a41c5eec1109121376de3d32379613856d4a47dd

# Rotating Node Testnet

TODO
