package lite_test

import (
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/lite2/provider"
	mockp "github.com/tendermint/tendermint/lite2/provider/mock"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
)

// NOTE: block is produced every minute. Make sure the verification time
// provided in the function call is correct for the size of the blockchain. The
// benchmarking may take some time hence it can be more useful to set the time
// or the amount of iterations use the flag -benchtime t -> i.e. -benchtime 5m
// or -benchtime 100x.
//
// Remember that none of these benchmarks account for network latency.
var (
	benchmarkFullNode = mockp.New(GenMockNode(chainID, 1000, 100, 1, bTime))
	genesisHeader, _  = benchmarkFullNode.SignedHeader(1)
)

func BenchmarkSequence(b *testing.B) {
	c, err := lite.NewClient(
		chainID,
		lite.TrustOptions{
			Period: 24 * time.Hour,
			Height: 1,
			Hash:   genesisHeader.Hash(),
		},
		benchmarkFullNode,
		[]provider.Provider{benchmarkFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		lite.Logger(log.TestingLogger()),
		lite.SequentialVerification(),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, err = c.VerifyHeaderAtHeight(1000, bTime.Add(1000*time.Minute))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBisection(b *testing.B) {
	c, err := lite.NewClient(
		chainID,
		lite.TrustOptions{
			Period: 24 * time.Hour,
			Height: 1,
			Hash:   genesisHeader.Hash(),
		},
		benchmarkFullNode,
		[]provider.Provider{benchmarkFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		lite.Logger(log.TestingLogger()),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, err = c.VerifyHeaderAtHeight(1000, bTime.Add(1000*time.Minute))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBackwards(b *testing.B) {
	trustedHeader, _ := benchmarkFullNode.SignedHeader(0)
	c, err := lite.NewClient(
		chainID,
		lite.TrustOptions{
			Period: 24 * time.Hour,
			Height: trustedHeader.Height,
			Hash:   trustedHeader.Hash(),
		},
		benchmarkFullNode,
		[]provider.Provider{benchmarkFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		lite.Logger(log.TestingLogger()),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, err = c.VerifyHeaderAtHeight(1, bTime)
		if err != nil {
			b.Fatal(err)
		}
	}
}
