# Trust Metric Design

## Overview

The proposed trust metric will allow Tendermint to maintain local trust rankings for peers it has directly interacted with, which can then be used to implement soft security controls. The calculations were obtained from the [TrustGuard](https://dl.acm.org/citation.cfm?id=1060808) project.

## Background

The Tendermint Core project developers would like to improve Tendermint security and reliability by keeping track of the level of trustworthiness peers have demonstrated within the peer-to-peer network. This way, undesirable outcomes from peers will not immediately result in them being dropped from the network (potentially causing drastic changes to take place). Instead, peers behavior can be monitored with appropriate metrics and be removed from the network once Tendermint Core is certain the peer is a threat. For example, when the PEXReactor makes a request for peers network addresses from a already known peer, and the returned network addresses are unreachable, this untrustworthy behavior should be tracked. Returning a few bad network addresses probably shouldn’t cause a peer to be dropped, while excessive amounts of this behavior does qualify the peer being dropped.

Trust metrics can be circumvented by malicious nodes through the use of strategic oscillation techniques, which adapts the malicious node’s behavior pattern in order to maximize its goals. For instance, if the malicious node learns that the time interval of the Tendermint trust metric is *X* hours, then it could wait *X* hours in-between malicious activities. We could try to combat this issue by increasing the interval length, yet this will make the system less adaptive to recent events.

Instead, having shorter intervals, but keeping a history of interval values, will give our metric the flexibility needed in order to keep the network stable, while also making it resilient against a strategic malicious node in the Tendermint peer-to-peer network. Also, the metric can access trust data over a rather long period of time while not greatly increasing its history size by aggregating older history values over a larger number of intervals, and at the same time, maintain great precision for the recent intervals. This approach is referred to as fading memories, and closely resembles the way human beings remember their experiences. The trade-off to using history data is that the interval values should be preserved in-between executions of the node.

## Scope

The proposed trust metric will be implemented as a Go programming language object that will allow a developer to inform the object of all good and bad events relevant to the trust object instantiation, and at any time, the metric can be queried for the current trust ranking. Methods will be provided for storing trust metric history data that is required across instantiations.

## Detailed Design

This section will cover the process being considered for calculating the trust ranking and the interface for the trust metric.

### Proposed Process

The proposed trust metric will count good and bad events relevant to the object, and calculate the percent of counters that are good over an interval with a predefined duration. This is the procedure that will continue for the life of the trust metric. When the trust metric is queried for the current **trust value**, a resilient equation will be utilized to perform the calculation.

The equation being proposed resembles a Proportional-Integral-Derivative (PID) controller used in control systems. The proportional component allows us to be sensitive to the value of the most recent interval, while the integral component allows us to incorporate trust values stored in the history data, and the derivative component allows us to give weight to sudden changes in the behavior of a peer. We compute the trust value of a peer in interval i based on its current trust ranking, its trust rating history prior to interval *i* (over the past *maxH* number of intervals) and its trust ranking fluctuation. We will break up the equation into the three components.

```math
(1) Proportional Value = a * R[i]
```

where *R*[*i*] denotes the raw trust value at time interval *i* (where *i* == 0 being current time) and *a* is the weight applied to the contribution of the current reports. The next component of our equation uses a weighted sum over the last *maxH* intervals to calculate the history value for time *i*:
 

`H[i] = ` ![formula1](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/img/formula1.png "Weighted Sum Formula")


The weights can be chosen either optimistically or pessimistically. With the history value available, we can now finish calculating the integral value:

```math
(2) Integral Value = b * H[i]
```

Where *H*[*i*] denotes the history value at time interval *i* and *b* is the weight applied to the contribution of past performance for the object being measured. The derivative component will be calculated as follows:

```math
D[i] = R[i] – H[i]

(3) Derivative Value = (c * D[i]) * D[i]
```

Where the value of *c* is selected based on the *D*[*i*] value relative to zero. With the three components brought together, our trust value equation is calculated as follows:

```math
TrustValue[i] = a * R[i] + b * H[i] + (c * D[i]) * D[i]
```

As a performance optimization that will keep the amount of raw interval data being saved to a reasonable size of *m*, while allowing us to represent 2^*m* - 1 history intervals, we can employ the fading memories technique that will trade space and time complexity for the precision of the history data values by summarizing larger quantities of less recent values. While our equation above attempts to access up to *maxH* (which can be 2^*m* - 1), we will map those requests down to *m* values using equation 4 below:

```math
(4) j = index, where index > 0
```

Where *j* is one of *(0, 1, 2, … , m – 1)* indices used to access history interval data. Now we can access the raw intervals using the following calculations:

```math
R[0] = raw data for current time interval
```

`R[j] = ` ![formula2](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/img/formula2.png "Fading Memories Formula")


### Interface Detailed Design

This section will cover the Go programming language API designed for the previously proposed process. Below is the interface for a TrustMetric:

```go
package trust

type TrustMetric struct {
    
}

type TrustMetricConfig struct {
    ProportionalWeight float64
    IntegralWeight float64
    HistoryMaxSize int
    IntervalLen time.Duration
}

func (tm *TrustMetric) Stop()

func (tm *TrustMetric) IncBad()

func (tm *TrustMetric) AddBad(num int)

func (tm *TrustMetric) IncGood()

func (tm *TrustMetric) AddGood(num int)

// get the dependable trust value
func (tm *TrustMetric) TrustValue() float64

func NewMetric() *TrustMetric

func NewMetricWithConfig(tmc *TrustMetricConfig) *TrustMetric

func GetPeerTrustMetric(key string) *TrustMetric

func PeerDisconnected(key string)

```

## References

S. Mudhakar, L. Xiong, and L. Liu, “TrustGuard: Countering Vulnerabilities in Reputation Management for Decentralized Overlay Networks,” in *Proceedings of the 14th international conference on World Wide Web, pp. 422-431*, May 2005.