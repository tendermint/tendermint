package proxy

import (
	"testing"
	"time"

	"github.com/tendermint/go-p2p"
	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
)

func TestPersistence(t *testing.T) {

	// create persistent dummy app
	// set state on dummy app
	// proxy handshake

	config := tendermint_test.ResetConfig("proxy_test_")
	multiApp := NewMultiAppConn(config, state, blockStore)

}
