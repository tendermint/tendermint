package node

import (
	"testing"
	"time"

	"github.com/tendermint/go-p2p"
	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
)

func init() {
	tendermint_test.ResetConfig("node_node_test")
}

func TestNodeStartStop(t *testing.T) {

	// Get PrivValidator
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)

	// Create & start node
	n := NewNode(privValidator)
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"), config.GetBool("skip_upnp"))
	n.AddListener(l)
	n.Start()
	log.Notice("Started node", "nodeInfo", n.sw.NodeInfo())
	time.Sleep(time.Second * 2)
	ch := make(chan struct{}, 1)
	go func() {
		n.Stop()
		ch <- struct{}{}
	}()
	ticker := time.NewTicker(time.Second * 5)
	select {
	case <-ch:
	case <-ticker.C:
		t.Fatal("timed out waiting for shutdown")
	}
}
