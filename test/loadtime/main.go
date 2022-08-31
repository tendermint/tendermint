package main

import (
	"crypto/rand"
	"fmt"

	"github.com/informalsystems/tm-load-test/pkg/loadtest"
	"github.com/tendermint/tendermint/test/loadtime/payload"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Ensure all of the interfaces are correctly satisfied.
var (
	_ loadtest.ClientFactory = (*ClientFactory)(nil)
	_ loadtest.Client        = (*TxGenerator)(nil)
)

// ClientFactory implements the loadtest.ClientFactory interface.
type ClientFactory struct{}

// TxGenerator is responsible for generating transactions.
// TxGenerator holds the set of information that will be used to generate
// each transaction.
type TxGenerator struct {
	conns            uint64
	rate             uint64
	size             uint64
	payloadSizeBytes uint64
}

func main() {
	if err := loadtest.RegisterClientFactory("loadtime-client", &ClientFactory{}); err != nil {
		panic(err)
	}
	loadtest.Run(&loadtest.CLIConfig{
		AppName:              "loadtime",
		AppShortDesc:         "Generate timestamped transaction load.",
		AppLongDesc:          "loadtime generates transaction load for the purpose of measuring the end-to-end latency of a transaction from submission to execution in a Tendermint network.",
		DefaultClientFactory: "loadtime-client",
	})
}

func (f *ClientFactory) ValidateConfig(cfg loadtest.Config) error {
	psb, err := payload.CalculateUnpaddedSizeBytes()
	if err != nil {
		return err
	}

	if psb > cfg.Size {
		return fmt.Errorf("payload size exceeds configured size")
	}
	return nil
}

func (f *ClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	psb, err := payload.CalculateUnpaddedSizeBytes()
	if err != nil {
		return nil, err
	}
	return &TxGenerator{
		conns:            uint64(cfg.Connections),
		rate:             uint64(cfg.Rate),
		size:             uint64(cfg.Size),
		payloadSizeBytes: uint64(psb),
	}, nil
}

func (c *TxGenerator) GenerateTx() ([]byte, error) {
	p := &payload.Payload{
		Time:        timestamppb.Now(),
		Connections: c.conns,
		Rate:        c.rate,
		Size:        c.size,
		Padding:     make([]byte, c.size-c.payloadSizeBytes),
	}
	_, err := rand.Read(p.Padding)
	if err != nil {
		return nil, err
	}
	b, err := proto.Marshal(p)
	if err != nil {
		return nil, err
	}

	// prepend a single key so that the kv store only ever stores a single
	// transaction instead of storing all tx and ballooning in size.
	return append([]byte("a="), b...), nil
}
