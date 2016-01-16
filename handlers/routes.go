package handlers

import (
	rpc "github.com/tendermint/go-rpc/server"
	"github.com/tendermint/netmon/types"
)

func Routes(network *TendermintNetwork) map[string]*rpc.RPCFunc {
	return map[string]*rpc.RPCFunc{
		// subscribe/unsubscribe are reserved for websocket events.
		//	"subscribe":   rpc.NewWSRPCFunc(Subscribe, []string{"event"}),
		//	"unsubscribe": rpc.NewWSRPCFunc(Unsubscribe, []string{"event"}),

		"status":         rpc.NewRPCFunc(StatusResult(network), ""),
		"blockchain":     rpc.NewRPCFunc(GetChainResult(network), "chain"),
		"register_chain": rpc.NewRPCFunc(RegisterChainResult(network), "chainConfig"),
		"validator_set":  rpc.NewRPCFunc(GetValidatorSetResult(network), "valsetID"),
		"validator":      rpc.NewRPCFunc(GetValidatorResult(network), "valSetID,valID"),

		"start_meter": rpc.NewRPCFunc(network.StartMeter, "chainID,valID,event"),
		"stop_meter":  rpc.NewRPCFunc(network.StopMeter, "chainID,valID,event"),
		"meter":       rpc.NewRPCFunc(GetMeterResult(network), "chainID,valID,event"),
	}
}

func StatusResult(network *TendermintNetwork) interface{} {
	return func() (NetMonResult, error) {
		return network.Status()
	}
}

func GetChainResult(network *TendermintNetwork) interface{} {
	return func(chain string) (NetMonResult, error) {
		return network.GetChain(chain)
	}
}

func RegisterChainResult(network *TendermintNetwork) interface{} {
	return func(chainConfig *types.BlockchainConfig) (NetMonResult, error) {
		return network.RegisterChain(chainConfig)
	}
}

func GetValidatorSetResult(network *TendermintNetwork) interface{} {
	return func(valSetID string) (NetMonResult, error) {
		return network.GetValidatorSet(valSetID)
	}
}

func GetValidatorResult(network *TendermintNetwork) interface{} {
	return func(valSetID, valID string) (NetMonResult, error) {
		return network.GetValidator(valSetID, valID)
	}
}

func GetMeterResult(network *TendermintNetwork) interface{} {
	return func(chainID, valID, eventID string) (NetMonResult, error) {
		return network.GetMeter(chainID, valID, eventID)
	}
}
