package handlers

import (
	"github.com/tendermint/go-event-meter"
	"github.com/tendermint/go-events"

	tmtypes "github.com/tendermint/tendermint/types"
)

// implements eventmeter.EventCallbackFunc
func (tn *TendermintNetwork) newBlockCallback(chainID, valID string) eventmeter.EventCallbackFunc {
	return func(metric *eventmeter.EventMetric, data events.EventData) {
		block := data.(tmtypes.EventDataNewBlock).Block

		tn.mtx.Lock()
		defer tn.mtx.Unlock()

		// grab chain and validator
		chain := tn.Chains[chainID]
		val, _ := chain.Config.GetValidatorByID(valID)

		// update height for validator
		val.BlockHeight = block.Header.Height

		// possibly update height and mean block time for chain
		if block.Header.Height > chain.Status.Height {
			chain.Status.NewBlock(block)
		}

	}
}

// implements eventmeter.EventLatencyFunc
func (tn *TendermintNetwork) latencyCallback(chainID, valID string) eventmeter.LatencyCallbackFunc {
	return func(latency float64) {
		tn.mtx.Lock()
		defer tn.mtx.Unlock()

		// grab chain and validator
		chain := tn.Chains[chainID]
		val, _ := chain.Config.GetValidatorByID(valID)

		// update latency for this validator and avg latency for chain
		mean := chain.Status.MeanLatency * float64(chain.Status.NumValidators)
		mean = (mean - val.Latency + latency) / float64(chain.Status.NumValidators)
		val.Latency = latency
		chain.Status.MeanLatency = mean

		// TODO: possibly update active nodes and uptime for chain
		chain.Status.ActiveValidators = chain.Status.NumValidators // XXX

	}
}
