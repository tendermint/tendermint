package light_test

import (
	"context"
	"testing"
	"time"

	dashcore "github.com/tendermint/tendermint/dashcore/rpc"
	"github.com/tendermint/tendermint/light"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light/provider"
	mockp "github.com/tendermint/tendermint/light/provider/mock"
	dbs "github.com/tendermint/tendermint/light/store/db"
	dbm "github.com/tendermint/tm-db"
)

// NOTE: block is produced every minute. Make sure the verification time
// provided in the function call is correct for the size of the blockchain. The
// benchmarking may take some time hence it can be more useful to set the time
// or the amount of iterations use the flag -benchtime t -> i.e. -benchtime 5m
// or -benchtime 100x.
//
// Remember that none of these benchmarks account for network latency.
var (
	blocks            = int64(1000)
	benchmarkFullNode = mockp.New(genMockNode(chainID, blocks, 20, bTime))
)

func setupDashCoreRPCMockForBenchmark(b *testing.B) {
	dashCoreMockClient = dashcore.NewMockClient(chainID, 100, benchmarkFullNode.MockPV, false)

	b.Cleanup(func() {
		dashCoreMockClient = nil
	})
}

func BenchmarkSequence(b *testing.B) {
	setupDashCoreRPCMockForBenchmark(b)

	c, err := light.NewClient(
		context.Background(),
		chainID,
		benchmarkFullNode,
		[]provider.Provider{benchmarkFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, err = c.VerifyLightBlockAtHeight(context.Background(), 1000, bTime.Add(1000*time.Minute))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBisection(b *testing.B) {
	setupDashCoreRPCMockForBenchmark(b)

	c, err := light.NewClient(
		context.Background(),
		chainID,
		benchmarkFullNode,
		[]provider.Provider{benchmarkFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, err = c.VerifyLightBlockAtHeight(context.Background(), 1000, bTime.Add(1000*time.Minute))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBackwards(b *testing.B) {
	setupDashCoreRPCMockForBenchmark(b)
	_, err := benchmarkFullNode.LightBlock(context.Background(), 0)
	if err != nil {
		b.Fatal(err)
	}

	c, err := light.NewClient(
		context.Background(),
		chainID,
		benchmarkFullNode,
		[]provider.Provider{benchmarkFullNode},
		dbs.New(dbm.NewMemDB(), chainID),
		dashCoreMockClient,
		light.Logger(log.TestingLogger()),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, err = c.VerifyLightBlockAtHeight(context.Background(), 1, bTime)
		if err != nil {
			b.Fatal(err)
		}
	}
}
