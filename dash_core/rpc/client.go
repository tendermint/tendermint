package dash_rpc

import (
	rpc "github.com/dashevo/dashd-go/rpcclient"
)

// DashCoreSignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type DashCoreSignerClient struct {
	endpoint    *rpc.Client
	host        string
	rpcUsername string
	rpcPassword string
}
