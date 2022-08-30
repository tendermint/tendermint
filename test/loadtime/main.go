package main

import (
	"crypto/rand"
	"fmt"

	"github.com/informalsystems/tm-load-test/pkg/loadtest"
	"github.com/tendermint/tendermint/test/loadtime/payload"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// payloadSizeBytes holds the maximum encoded size of the payload message, not
// counting the 'Padding' field.
// This value is used to calculated how much padding should be included in the
// encoded data.
//
// The values break down as follows:
// 7 total field tags, each 1 byte.
// 4 varint encoded uint64s, requiring 10 total bytes.
// 1 varint encoded uint32, requiring 5 total bytes.
const payloadSizeBytes = 7 + 4*10 + 1*5

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
	conns uint64
	rate  uint64
	size  uint64
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
	if cfg.Size < payloadSizeBytes {
		return fmt.Errorf("payload size exceeds configured size")
	}
	return nil
}

func (f *ClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	return &TxGenerator{
		conns: uint64(cfg.Connections),
		rate:  uint64(cfg.Rate),
		size:  uint64(cfg.Size),
	}, nil
}

func (c *TxGenerator) GenerateTx() ([]byte, error) {
	p := &payload.Payload{
		Time:        timestamppb.Now(),
		Connections: c.conns,
		Rate:        c.rate,
		Size:        c.size,
		Padding:     make([]byte, c.size-payloadSizeBytes),
	}
	_, err := rand.Read(p.Padding)
	if err != nil {
		return nil, err
	}
	b, err := proto.Marshal(p)
	if err != nil {
		return nil, err
	}
	return b, nil
}
