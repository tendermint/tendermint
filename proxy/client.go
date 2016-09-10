package proxy

import (
	"fmt"
	"sync"

	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/dummy"
	nilapp "github.com/tendermint/tmsp/example/nil"
)

// Function type to get a connected tmsp client
// Allows consumers to provide their own in-proc apps,
// or to implement alternate address schemes and transports
type NewTMSPClient func(addr, transport string) (tmspcli.Client, error)

// Get a connected tmsp client.
// Offers some default in-proc apps, else socket/grpc.
func NewTMSPClientDefault(addr, transport string) (tmspcli.Client, error) {
	var client tmspcli.Client

	// use local app (for testing)
	// TODO: local proxy app conn
	switch addr {
	case "nilapp":
		app := nilapp.NewNilApplication()
		mtx := new(sync.Mutex) // TODO
		client = tmspcli.NewLocalClient(mtx, app)
	case "dummy":
		app := dummy.NewDummyApplication()
		mtx := new(sync.Mutex) // TODO
		client = tmspcli.NewLocalClient(mtx, app)
	default:
		// Run forever in a loop
		mustConnect := false
		remoteApp, err := tmspcli.NewClient(addr, transport, mustConnect)
		if err != nil {
			return nil, fmt.Errorf("Failed to connect to proxy for mempool: %v", err)
		}
		client = remoteApp
	}
	return client, nil
}
