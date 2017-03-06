package main

import (
	"errors"
	"net/http"

	rpc "github.com/tendermint/go-rpc/server"
)

func startRPC(listenAddr string, m *Monitor) {
	routes := routes(m)

	// serve http and ws
	mux := http.NewServeMux()
	wm := rpc.NewWebsocketManager(routes, nil) // TODO: evsw
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	rpc.RegisterRPCFuncs(mux, routes)
	if _, err := rpc.StartHTTPServer(listenAddr, mux); err != nil {
		panic(err)
	}
}

func routes(m *Monitor) map[string]*rpc.RPCFunc {
	return map[string]*rpc.RPCFunc{
		"status":         rpc.NewRPCFunc(RPCStatus(m), ""),
		"status/network": rpc.NewRPCFunc(RPCNetworkStatus(m), ""),
		"status/node":    rpc.NewRPCFunc(RPCNodeStatus(m), "name"),
		"monitor":        rpc.NewRPCFunc(RPCMonitor(m), "endpoint"),
		"unmonitor":      rpc.NewRPCFunc(RPCUnmonitor(m), "endpoint"),

		// "start_meter": rpc.NewRPCFunc(network.StartMeter, "chainID,valID,event"),
		// "stop_meter":  rpc.NewRPCFunc(network.StopMeter, "chainID,valID,event"),
		// "meter":       rpc.NewRPCFunc(GetMeterResult(network), "chainID,valID,event"),
	}
}

// RPCStatus returns common statistics for the network and statistics per node.
func RPCStatus(m *Monitor) interface{} {
	return func() (networkAndNodes, error) {
		values := make([]*Node, len(m.Nodes))
		i := 0
		for _, v := range m.Nodes {
			values[i] = v
			i++
		}

		return networkAndNodes{m.Network, values}, nil
	}
}

// RPCNetworkStatus returns common statistics for the network.
func RPCNetworkStatus(m *Monitor) interface{} {
	return func() (*Network, error) {
		return m.Network, nil
	}
}

// RPCNodeStatus returns statistics for the given node.
func RPCNodeStatus(m *Monitor) interface{} {
	return func(name string) (*Node, error) {
		if n, ok := m.Nodes[name]; ok {
			return n, nil
		}
		return nil, errors.New("Cannot find node with that name")
	}
}

// RPCMonitor allows to dynamically add a endpoint to under the monitor.
func RPCMonitor(m *Monitor) interface{} {
	return func(endpoint string) (*Node, error) {
		n := NewNode(endpoint)
		if err := m.Monitor(n); err != nil {
			return nil, err
		}
		return n, nil
	}
}

// RPCUnmonitor removes the given endpoint from under the monitor.
func RPCUnmonitor(m *Monitor) interface{} {
	return func(endpoint string) (bool, error) {
		if n, ok := m.Nodes[endpoint]; ok {
			m.Unmonitor(n)
			return true, nil
		}
		return false, errors.New("Cannot find node with that name")
	}
}

// func (tn *TendermintNetwork) StartMeter(chainID, valID, eventID string) error {
// 	tn.mtx.Lock()
// 	defer tn.mtx.Unlock()
// 	val, err := tn.getChainVal(chainID, valID)
// 	if err != nil {
// 		return err
// 	}
// 	return val.EventMeter().Subscribe(eventID, nil)
// }

// func (tn *TendermintNetwork) StopMeter(chainID, valID, eventID string) error {
// 	tn.mtx.Lock()
// 	defer tn.mtx.Unlock()
// 	val, err := tn.getChainVal(chainID, valID)
// 	if err != nil {
// 		return err
// 	}
// 	return val.EventMeter().Unsubscribe(eventID)
// }

// func (tn *TendermintNetwork) GetMeter(chainID, valID, eventID string) (*eventmeter.EventMetric, error) {
// 	tn.mtx.Lock()
// 	defer tn.mtx.Unlock()
// 	val, err := tn.getChainVal(chainID, valID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return val.EventMeter().GetMetric(eventID)
// }

//--> types

type networkAndNodes struct {
	Network *Network `json:"network"`
	Nodes   []*Node  `json:"nodes"`
}
