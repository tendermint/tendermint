package main

import (
	"errors"
	"net"
	"net/http"

	"github.com/tendermint/tendermint/libs/log"
	rpc "github.com/tendermint/tendermint/rpc/lib/server"
	monitor "github.com/tendermint/tendermint/tools/tm-monitor/monitor"
)

func startRPC(listenAddr string, m *monitor.Monitor, logger log.Logger) net.Listener {
	routes := routes(m)

	mux := http.NewServeMux()
	wm := rpc.NewWebsocketManager(routes, nil)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	rpc.RegisterRPCFuncs(mux, routes, cdc, logger)
	listener, err := rpc.Listen(listenAddr, rpc.Config{})
	if err != nil {
		panic(err)
	}
	go rpc.StartHTTPServer(listener, mux, logger)
	return listener
}

func routes(m *monitor.Monitor) map[string]*rpc.RPCFunc {
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
func RPCStatus(m *monitor.Monitor) interface{} {
	return func() (networkAndNodes, error) {
		return networkAndNodes{m.Network, m.Nodes}, nil
	}
}

// RPCNetworkStatus returns common statistics for the network.
func RPCNetworkStatus(m *monitor.Monitor) interface{} {
	return func() (*monitor.Network, error) {
		return m.Network, nil
	}
}

// RPCNodeStatus returns statistics for the given node.
func RPCNodeStatus(m *monitor.Monitor) interface{} {
	return func(name string) (*monitor.Node, error) {
		if i, n := m.NodeByName(name); i != -1 {
			return n, nil
		}
		return nil, errors.New("Cannot find node with that name")
	}
}

// RPCMonitor allows to dynamically add a endpoint to under the monitor. Safe
// to call multiple times.
func RPCMonitor(m *monitor.Monitor) interface{} {
	return func(endpoint string) (*monitor.Node, error) {
		i, n := m.NodeByName(endpoint)
		if i == -1 {
			n = monitor.NewNode(endpoint)
			if err := m.Monitor(n); err != nil {
				return nil, err
			}
		}
		return n, nil
	}
}

// RPCUnmonitor removes the given endpoint from under the monitor.
func RPCUnmonitor(m *monitor.Monitor) interface{} {
	return func(endpoint string) (bool, error) {
		if i, n := m.NodeByName(endpoint); i != -1 {
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
	Network *monitor.Network `json:"network"`
	Nodes   []*monitor.Node  `json:"nodes"`
}
