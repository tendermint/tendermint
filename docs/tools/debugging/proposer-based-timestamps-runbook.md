---
order: 3
---

# Proposer-Based Timestamps Runbook

Version v0.36 of Tendermint added new constraints for the timestamps included in
each block created by Tendermint. The new constraints mean that validators may
fail to produce valid blocks or may issue `nil` `prevotes` for proposed blocks
depending on the configuration of the validator's local clock.

## What is this document for?

This document provides a set of actionable steps for application developers and
node operators to diagnose and fix issues related to clock synchronization and
configuration of the Proposer-Based Timestamps [SynchronyParams](https://github.com/tendermint/tendermint/blob/master/spec/core/data_structures.md#synchronyparams).

Use this runbook if you observe that validators are frequently voting `nil` for a block that the rest
of the network votes for or if validators are frequently producing block proposals
that are not voted for by the rest of the network.

## Requirements

To use this runbook, you must be running a node that has the [Prometheus metrics endpoint enabled](https://github.com/tendermint/tendermint/blob/master/docs/nodes/metrics.md)
and the Tendermint RPC endpoint enabled and accessible.

It is strongly recommended to also run a Prometheus metrics collector to gather and
analyze metrics from the Tendermint node.

## Debugging a Single Node

If you observe that a single validator is frequently failing to produce blocks or
voting nil for proposals that other validators vote for and suspect it may be
related to clock synchronization, use the following steps to debug and correct the issue.

### Check Timely Metric

Tendermint exposes a histogram metric for the difference between the timestamp in the proposal
the and the time read from the node's local clock when the proposal is received.

The histogram exposes multiple metrics on the Prometheus `/metrics` endpoint called
* `tendermint_consensus_proposal_timestamp_difference_bucket`.
* `tendermint_consensus_proposal_timestamp_difference_sum`.
* `tendermint_consensus_proposal_timestamp_difference_count`.

Each metric is also labeled with the key `is_timely`, which can have a value of
`true` or `false`.

#### From the Prometheus Collector UI

If you are running a Prometheus collector, navigate to the query web interface and select the 'Graph' tab.

Issue a query for the following:

```
tendermint_consensus_proposal_timestamp_difference_count{is_timely="false"} /
tendermint_consensus_proposal_timestamp_difference_count{is_timely="true"}
```

This query will graph the ratio of proposals the node considered timely to those it
considered untimely. If the ratio is increasing, it means that your node is consistently
seeing more proposals that are far from its local clock. If this is the case, you should
check to make sure your local clock is properly synchronized to NTP.

#### From the `/metrics` url

If you are not running a Prometheus collector, navigate to the `/metrics` endpoint
exposed on the Prometheus metrics port with `curl` or a browser.

Search for the `tendermint_consensus_proposal_timestamp_difference_count` metrics.
This metric is labeled with `is_timely`. Investigate the value of
`tendermint_consensus_proposal_timestamp_difference_count` where `is_timely="false"`
and where `is_timely="true"`. Refresh the endpoint and observe if the value of `is_timely="false"`
is growing.

If you observe that `is_timely="false"` is growing, it means that your node is consistently
seeing proposals that are far from its local clock. If this is the case, you should check
to make sure your local clock is properly synchronized to NTP.

### Checking Clock Sync

NTP configuration and tooling is very specific to the operating system and distribution
that your validator node is running. This guide assumes you have `timedatectl` installed with
[chrony](https://chrony.tuxfamily.org/), a popular tool for interacting with time
synchronization on Linux distributions. If you are using an operating system or
distribution with a different time synchronization mechanism, please consult the
documentation for your operating system to check the status and re-synchronize the daemon.

#### Check if NTP is Enabled

```shell
$ timedatectl
```

From the output, ensure that `NTP service` is `active`. If `NTP service` is `inactive`, run:

```shell
$ timedatectl set-ntp true
```

Re-run the `timedatectl` command and verify that the change has taken effect.

#### Check if Your NTP Daemon is Synchronized

Check the status of your local `chrony` NTP daemon using by running the following:

```shell
$ chronyc tracking
```

If the `chrony` daemon is running, you will see output that indicates its current status.
If the `chrony` daemon is not running, restart it and re-run `chronyc tracking`.

The `System time` field of the response should show a value that is much smaller than 100
milliseconds.

If the value is very large, restart the `chronyd` daemon.

## Debugging a Network

If you observe that a network is frequently failing to produce blocks and suspect
it may be related to clock synchronization, use the following steps to debug and correct the issue.

### Check Prevote Message Delay

Tendermint exposes metrics that help determine how synchronized the clocks on a network are.

These metrics are visible on the Prometheus `/metrics` endpoint and are called:
* `tendermint_consensus_quorum_prevote_delay`
* `tendermint_consensus_full_prevote_delay`

These metrics calculate the difference between the timestamp in the proposal message and
the timestamp of a prevote that was issued during consensus.

The `tendermint_consensus_quorum_prevote_delay` metric is the interval in seconds
between the proposal timestamp and the timestamp of the earliest prevote that
achieved a quorum during the prevote step.

The `tendermint_consensus_full_prevote_delay` metric is the interval in seconds
between the proposal timestamp and the timestamp of the latest prevote in a round
where 100% of the validators voted.

#### From the Prometheus Collector UI

If you are running a Prometheus collector, navigate to the query web interface and select the 'Graph' tab.

Issue a query for the following:

```
sum(tendermint_consensus_quorum_prevote_delay) by (proposer_address)
```

This query will graph the difference in seconds for each proposer on the network.

If the value is much larger for some proposers, then the issue is likely related to the clock
synchronization of their nodes. Contact those proposers and ensure that their nodes
are properly connected to NTP using the steps for [Debugging a Single Node](#debugging-a-single-node).

If the value is relatively similar for all proposers you should next compare this
value to the `SynchronyParams` values for the network. Continue to the [Checking
Sychrony](#checking-synchrony) steps.

#### From the `/metrics` url

If you are not running a Prometheus collector, navigate to the `/metrics` endpoint
exposed on the Prometheus metrics port.

Search for the `tendermint_consensus_quorum_prevote_delay` metric. There will be one
entry of this metric for each `proposer_address`. If the value of this metric is
much larger for some proposers, then the issue is likely related to synchronization of their
nodes with NTP. Contact those proposers and ensure that their nodes are properly connected
to NTP using the steps for [Debugging a Single Node](#debugging-a-single-node).

If the values are relatively similar for all proposers you should next compare,
you'll need to compare this value to the `SynchronyParams` for the network. Continue
to the [Checking Sychrony](#checking-synchrony) steps.

### Checking Synchrony

To determine the currently configured `SynchronyParams` for your network, issue a
request to your node's RPC endpoint. For a node running locally with the RPC server
exposed on port `26657`, run the following command:

```shell
$ curl localhost:26657/consensus_params
```

The json output will contain a field named `synchrony`, with the following structure:

```json
{
  "precision": "500000000",
  "message_delay": "3000000000"
}
```

The `precision` and `message_delay` values returned are listed in nanoseconds:
In the examples above, the precision is 500ms and the message delay is 3s.
Remember, `tendermint_consensus_quorum_prevote_delay` is listed in seconds.
If the `tendermint_consensus_quorum_prevote_delay` value approaches the sum of `precision` and `message_delay`,
then the value selected for these parameters is too small. Your application will
need to be modified to update the `SynchronyParams` to have larger values.

### Updating SynchronyParams

The `SynchronyParams` are `ConsensusParameters` which means they are set and updated
by the application running alongside Tendermint. Updates to these parameters must
be passed to the application during the `FinalizeBlock` ABCI method call.

If the application was built using the CosmosSDK, then these parameters can be updated
programatically using a governance proposal. For more information, see the [CosmosSDK
documentation](https://hub.cosmos.network/main/governance/submitting.html#sending-the-transaction-that-submits-your-governance-proposal).

If the application does not implement a way to update the consensus parameters
programatically, then the application itself must be updated to do so. More information on updating
the consensus parameters via ABCI can be found in the [FinalizeBlock documentation](https://github.com/tendermint/tendermint/blob/master/spec/abci++/abci++_methods_002_draft.md#finalizeblock).
