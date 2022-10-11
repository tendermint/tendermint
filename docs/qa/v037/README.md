---
order: 1
parent:
  title: Tendermint Quality Assurance Results for v0.37.x
  description: This is a report on the results obtained when running v0.37.x on testnets
  order: 2
---

# 200 Node Testnet

## Finding the Saturation Point

The first goal is to identify the saturation point and compare it with the baseline (v0.34.x).
For further details, see [this paragraph](../v034/README.md#finding-the-saturation-point)
in the baseline version.

The following table summarizes the results for v0.37.x, for the different experiments
(extracted from file [`v037_report_tabbed.txt`](./img/v037_report_tabbed.txt)).

The X axis of this table is `c`, the number of connections created by the load runner process to the target node.
The Y axis of this table is `r`, the rate or number of transactions issued per second.

|        |  c=4 |  c=8 | c=16 |
| :---   | ---: | ---: | ---: |
| r=20   |  148 |  288 |  591 |
| r=200  | 1519 | 3087 | 6198 |
| r=400  | 3094 | 6231 | 8698 |
| r=800  | 6155 | 8444 | 8407 |
| r=1200 | 8290 | 8448 | 8666 |

For comparison, this is the table with the baseline version.

|        |  c=4 |  c=8 | c=16 |
| :---   | ---: | ---: | ---: |
| r=20   |  144 |  309 |  632 |
| r=200  | 1547 | 3195 | 5958 |
| r=400  | 3102 | 6110 | 8526 |
| r=800  | 6231 | 8224 | 8653 |
| r=1200 | 7978 | 8368 | 9087 |

The saturation point is a diagonal across:

* `r=1200,c=4`
* `r=800,c=8`
* `r=400,c=16`

which is at the same place as the baseline. For more details on the saturation point, see
[this paragraph](../v034/README.md#finding-the-saturation-point) in the baseline version.

The experiment chosen to examine Prometheus metrics is the same as in the baseline:
**`r=400,c=8`**.

This is a plot of the CPU load of the load runner for `r=400,c=8`,
where we can see that the load (average over 1 minute) stays below 1 most of the time.

![load-load-runner](./img/v037_r400c8_load-runner.png)

## Examining latencies

The method described [here](../method.md) allows us to plot the latencies of transactions
for all experiments.

![all-latencies](./img/v037_r400c8_latencies.png)

The data seen in the plot is similar to that of the baseline.

![all-latencies-bl](../v034/img/v034_r400c8_latencies.png)

Therefore, for further details on these plots,
see [this paragraph](../v034/README.md#examining-latencies) in the baseline version.

## Prometheus Metrics on the Chosen Experiment

As mentioned [above](#finding-the-saturation-point), the chosen experiment is `r=400,c=8`.
This section further examines key metrics for this experiment extracted from Prometheus data.

### Mempool Size

The mempool size, a count of the number of transactions in the mempool, was shown to be stable and homogeneous
at all full nodes. It did not exhibit any unconstrained growth.
The plot below shows the evolution over time of the cumulative number of transactions inside all full nodes' mempools.

![mempool-cumulative](./img/v037_r400c8_mempool_size.png)

The plot below shows evolution of the average over all full nodes, which oscillate around 140 outstanding transactions.

![mempool-avg](./img/v037_r400c8_mempool_size_avg.png)

The peaks observed coincide with the moments when some nodes reached round 1 of consensus (see below).

**These plots yield similar results to the baseline**:

![mempool-cumulative-bl](../v034/img/v034_r400c8_mempool_size.png)

![mempool-avg-bl](../v034/img/v034_r400c8_mempool_size_avg.png)

### Peers

The number of peers was stable at all nodes.
It was higher for the seed nodes (around 140) than for the rest (between 25 and 68).

![peers](./img/v037_r400c8_peers.png)

**This plot yields similar results to the baseline**:

![peers-bl](../v034/img/v034_r400c8_peers.png)

### Consensus Rounds per Height

Most heights took just one round, but some nodes needed to advance to round 1 at some point.

![rounds](./img/v037_r400c8_rounds.png)

**This plot yields slightly better results than the baseline**:

![rounds-bl](../v034/img/v034_r400c8_rounds.png)

### Blocks Produced per Minute, Transactions Processed per Minute

The blocks produced per minute are the gradient of this plot.

![heights](./img/v037_r400c8_heights.png)

Over a period of 2 minutes, the height goes from 680 to 748.
This results in an average of 34 blocks produced per minute.

The transactions processed per minute are the gradient of this plot.

![total-txs](./img/v037_r400c8_total-txs.png)

Over a period of 2 minutes, the total goes from 22581 to 28812 transactions,
resulting in 3115 transactions per minute. However, we can see in the plot that
all transactions in the load are process long before the two minutes.
If we adjust the time window when transactions are processed (approx. 93 seconds),
we obtain 4063 transactions per minute.

**These plots yield similar results to the baseline**:

![heights-bl](../v034/img/v034_r400c8_heights.png)

![total-txs](../v034/img/v034_r400c8_total-txs.png)

### Memory Resident Set Size

Resident Set Size of all monitored processes is plotted below.

![rss](./img/v037_r400c8_rss.png)

The average over all processes oscillates around 380 MiB and does not demonstrate unconstrained growth.

![rss-avg](./img/v037_r400c8_rss_avg.png)

**These plots yield similar results to the baseline**:

![rss-bl](../v034/img/v034_r400c8_rss.png)

![rss-avg-bl](../v034/img/v034_r400c8_rss_avg.png)

### CPU utilization

The best metric from Prometheus to gauge CPU utilization in a Unix machine is `load1`,
as it usually appears in the
[output of `top`](https://www.digitalocean.com/community/tutorials/load-average-in-linux).

![load1](./img/v037_r400c8_load1.png)

It is contained between 0.5 and 4 at all nodes.

**This plot yields similar results to the baseline**:

![load1](../v034/img/v034_r400c8_load1.png)

## Test Result

**Result: PASS**

Date: 2022-09-23

Version: b9480d0ec79c53b06344148afc6589f895d0abbf

# Rotating Node Testnet

We use the same load as in the baseline: `c=4,r=800`.

# Latencies

The plot of all latencies can be seen here.

![rotating-all-latencies](./img/v037_rotating_latencies.png)

Which is similar to the baseline.

![rotating-all-latencies-bl](../v034/img/v034_rotating_latencies_uniq.png)

Note that we are comparing against the baseline plot with _unique_
transactions. This is because the problem with duplicate transactions
detected during the baseline experiment did not show up for `v0.37`,
which is _not_ proof that the problem is not present in `v0.37`.

## Prometheus Metrics

The set of metrics shown here match those shown on the baseline (`v0.34`) for the same experiment.
We also show the baseline results for comparison.

### Blocks and Transactions per minute

The blocks produced per minute are the gradient of this plot.

![rotating-heights](./img/v037_rotating_heights.png)

Over a period of 4446 seconds, the height goes from 5 to 3323.
This results in an average of 45 blocks produced per minute,
which is similar to the baseline, shown below.

![rotating-heights-bl](../v034/img/v034_rotating_heights.png)

The following two plots show only the heights reported by ephemeral nodes.
The second plot is the baseline plot for comparison.

![rotating-heights-ephe](./img/v037_rotating_heights_ephe.png)

![rotating-heights-ephe-bl](../v034/img/v034_rotating_heights_ephe.png)

By the length of the segments, we can see that ephemeral nodes in `v0.37`
catch up slightly faster.

The transactions processed per minute are the gradient of this plot.

![rotating-total-txs](./img/v037_rotating_total-txs.png)

Over a period of 3852 seconds, the total goes from 597 to 267298 transactions in one of the validators,
resulting in 4154 transactions per minute, which is slightly lower than the baseline,
although the baseline had to deal with duplicate transactions.

For comparison, this is the baseline plot.

![rotating-total-txs-bl](../v034/img/v034_rotating_total-txs.png)

### Peers

The plot below shows the evolution of the number of peers throughout the experiment.

![rotating-peers](./img/v037_rotating_peers.png)

This is the baseline plot, for comparison.

![rotating-peers-bl](../v034/img/v034_rotating_peers.png)

The plotted values and their evolution are comparable in both plots.

For further details on these plots, see the baseline report.

### Memory Resident Set Size

The average Resident Set Size (RSS) over all processes looks slightly more stable
on `v0.37` (first plot) than on the baseline (second plot).

![rotating-rss-avg](./img/v037_rotating_rss_avg.png)

![rotating-rss-avg-bl](../v034/img/v034_rotating_rss_avg.png)

The memory taken by the validators and the ephemeral nodes when they are up is comparable (not shown in the plots),
just as observed in the baseline.

### CPU utilization

The plot shows metric `load1` for all nodes.

![rotating-load1](./img/v037_rotating_load1.png)

This is the baseline plot.

![rotating-load1-bl](../v034/img/v034_rotating_load1.png)

In both cases, it is contained under 5 most of the time, which is considered normal load.
The green line in the `v0.37` plot and the purple line in the baseline plot (`v0.34`)
correspond to the validators receiving all transactions, via RPC, from the load runner process.
In both cases, they oscillate around 5 (normal load). The main difference is that other
nodes are generally less loaded in `v0.37`.

## Test Result

**Result: PASS**

Date: 2022-10-10

Version: 155110007b9d8b83997a799016c1d0844c8efbaf
