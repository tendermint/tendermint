package handlers

import (
	rpc "github.com/tendermint/go-rpc/server"
)

func Routes(network *TendermintNetwork) map[string]*rpc.RPCFunc {
	return map[string]*rpc.RPCFunc{
		// subscribe/unsubscribe are reserved for websocket events.
		//	"subscribe":   rpc.NewWSRPCFunc(Subscribe, []string{"event"}),
		//	"unsubscribe": rpc.NewWSRPCFunc(Unsubscribe, []string{"event"}),

		"status":        rpc.NewRPCFunc(StatusResult(network), ""),
		"blockchain":    rpc.NewRPCFunc(GetChainResult(network), "chain"),
		"validator_set": rpc.NewRPCFunc(GetValidatorSetResult(network), "valsetID"),
		"validator":     rpc.NewRPCFunc(GetValidatorResult(network), "valSetID,valID"),

		"start_meter": rpc.NewRPCFunc(network.StartMeter, "chainID,valID,event"),
		"stop_meter":  rpc.NewRPCFunc(network.StopMeter, "chainID,valID,event"),
		"meter":       rpc.NewRPCFunc(GetMeterResult(network), "chainID,valID,event"),
	}
}

func StatusResult(network *TendermintNetwork) interface{} {
	return func() (*NetMonResult, error) {
		r, err := network.Status()
		if err != nil {
			return nil, err
		} else {
			return &NetMonResult{r}, nil
		}
	}
}

func GetChainResult(network *TendermintNetwork) interface{} {
	return func(chain string) (*NetMonResult, error) {
		r, err := network.GetChain(chain)
		if err != nil {
			return nil, err
		} else {
			return &NetMonResult{r}, nil
		}
	}
}

func GetValidatorSetResult(network *TendermintNetwork) interface{} {
	return func(valSetID string) (*NetMonResult, error) {
		r, err := network.GetValidatorSet(valSetID)
		if err != nil {
			return nil, err
		} else {
			return &NetMonResult{r}, nil
		}
	}
}

func GetValidatorResult(network *TendermintNetwork) interface{} {
	return func(valSetID, valID string) (*NetMonResult, error) {
		r, err := network.GetValidator(valSetID, valID)
		if err != nil {
			return nil, err
		} else {
			return &NetMonResult{r}, nil
		}
	}
}

func GetMeterResult(network *TendermintNetwork) interface{} {
	return func(chainID, valID, eventID string) (*NetMonResult, error) {
		r, err := network.GetMeter(chainID, valID, eventID)
		if err != nil {
			return nil, err
		} else {
			return &NetMonResult{r}, nil
		}
	}
}
