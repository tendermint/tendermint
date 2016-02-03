package handlers

import (
	"time"

	"github.com/tendermint/go-event-meter"
	"github.com/tendermint/go-events"

	"github.com/tendermint/netmon/types"

	tmtypes "github.com/tendermint/tendermint/types"
)

/*
	Each chain-validator gets an eventmeter which maintains the websocket
	Certain pre-defined events may update the netmon state: latency pongs, new blocks
	All callbacks are called in a go-routine by the event-meter
	TODO: config changes for new validators and changing ip/port
*/

func (tn *TendermintNetwork) registerCallbacks(chainState *types.ChainState, v *types.ValidatorState) error {
	v.EventMeter().RegisterLatencyCallback(tn.latencyCallback(chainState, v))
	v.EventMeter().RegisterDisconnectCallback(tn.disconnectCallback(chainState, v))
	return v.EventMeter().Subscribe(tmtypes.EventStringNewBlock(), tn.newBlockCallback(chainState, v))
}

// implements eventmeter.EventCallbackFunc
// updates validator and possibly chain with new block
func (tn *TendermintNetwork) newBlockCallback(chainState *types.ChainState, val *types.ValidatorState) eventmeter.EventCallbackFunc {
	return func(metric *eventmeter.EventMetric, data events.EventData) {
		block := data.(tmtypes.EventDataNewBlock).Block

		// these functions are thread safe
		// we should run them concurrently

		// update height for validator
		val.NewBlock(block)

		// possibly update height and mean block time for chain
		chainState.NewBlock(block)
	}
}

// implements eventmeter.EventLatencyFunc
func (tn *TendermintNetwork) latencyCallback(chain *types.ChainState, val *types.ValidatorState) eventmeter.LatencyCallbackFunc {
	return func(latency float64) {
		latency = latency / 1000000.0 // ns to ms
		oldLatency := val.UpdateLatency(latency)
		chain.UpdateLatency(oldLatency, latency)
	}
}

// implements eventmeter.DisconnectCallbackFunc
func (tn *TendermintNetwork) disconnectCallback(chain *types.ChainState, val *types.ValidatorState) eventmeter.DisconnectCallbackFunc {
	return func() {
		// Validator is down!
		chain.SetOnline(val, false)

		// reconnect
		// TODO: stop trying eventually ...
		for {
			time.Sleep(time.Second)

			if err := val.Start(); err != nil {
				log.Debug("Can't connect to validator", "valID", val.Config.Validator.ID)
			} else {
				// register callbacks for the validator
				tn.registerCallbacks(chain, val)

				chain.SetOnline(val, true)

				// TODO: authenticate pubkey

				return
			}
		}
	}
}
