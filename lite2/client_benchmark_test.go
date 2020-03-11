package lite

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite2/provider"
	mockp "github.com/tendermint/tendermint/lite2/provider/mock"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	dbm "github.com/tendermint/tm-db"
)

//
// #################################  BENCHMARKING ######################################
//

// NOTE: block is produced every minute. Make sure the verification time provided in the function call is correct for
// the size of the blockchain. The benchmarking may take some time hence it can be more useful to set the time or
// the amount of iterations use the flag -benchtime t -> i.e. -benchtime 5m or -benchtime 100x
// Remember that none of these benchmarks account for network latency
var (
	largeFullNode    = mockp.New(GenMockNode(chainID, 1000, 100, 1, bTime))
	genesisHeader, _ = largeFullNode.SignedHeader(1)
	result           *Client
)

func BenchmarkSequence(b *testing.B) {
	c, _ := NewClient(
		chainID,
		TrustOptions{
			Period: 24 * time.Hour,
			Height: 1,
			Hash:   genesisHeader.Hash(),
		},
		largeFullNode,
		[]provider.Provider{largeFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
		SequentialVerification(),
	)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = c.VerifyHeaderAtHeight(1000, bTime.Add(1000*time.Minute))
	}
}

func BenchmarkBisection(b *testing.B) {
	c, _ := NewClient(
		chainID,
		TrustOptions{
			Period: 24 * time.Hour,
			Height: 1,
			Hash:   genesisHeader.Hash(),
		},
		largeFullNode,
		[]provider.Provider{largeFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = c.VerifyHeaderAtHeight(1000, bTime.Add(1000*time.Minute))
	}
}

func BenchmarkBackwards(b *testing.B) {
	trustedHeader, _ := largeFullNode.SignedHeader(0)
	c, _ := NewClient(
		chainID,
		TrustOptions{
			Period: 24 * time.Hour,
			Height: trustedHeader.Height,
			Hash:   trustedHeader.Hash(),
		},
		largeFullNode,
		[]provider.Provider{largeFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		Logger(log.TestingLogger()),
	)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = c.VerifyHeaderAtHeight(1, bTime)
	}
}
