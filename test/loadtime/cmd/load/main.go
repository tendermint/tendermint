package main

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"

	"github.com/tendermint/tendermint/test/loadtime/payload"
)

// Ensure all of the interfaces are correctly satisfied.
var (
	_ loadtest.ClientFactory = (*ClientFactory)(nil)
	_ loadtest.Client        = (*TxGenerator)(nil)
)

// ClientFactory implements the loadtest.ClientFactory interface.
type ClientFactory struct {
	ID []byte
}

// TxGenerator is responsible for generating transactions.
// TxGenerator holds the set of information that will be used to generate
// each transaction.
type TxGenerator struct {
	id    []byte
	conns uint64
	rate  uint64
	size  uint64
}

func main() {
	u := [16]byte(uuid.New()) // generate run ID on startup
	if err := loadtest.RegisterClientFactory("loadtime-client", &ClientFactory{ID: u[:]}); err != nil {
		panic(err)
	}
	loadtest.Run(&loadtest.CLIConfig{
		AppName:              "loadtime",
		AppShortDesc:         "Generate timestamped transaction load.",
		AppLongDesc:          "loadtime generates transaction load for the purpose of measuring the end-to-end latency of a transaction from submission to execution in a Tendermint network.", //nolint:lll
		DefaultClientFactory: "loadtime-client",
	})
}

func (f *ClientFactory) ValidateConfig(cfg loadtest.Config) error {
	psb, err := payload.MaxUnpaddedSize()
	if err != nil {
		return err
	}
	if psb > cfg.Size {
		return fmt.Errorf("payload size exceeds configured size")
	}
	return nil
}

func (f *ClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	return &TxGenerator{
		id:    f.ID,
		conns: uint64(cfg.Connections),
		rate:  uint64(cfg.Rate),
		size:  uint64(cfg.Size),
	}, nil
}

func (c *TxGenerator) GenerateTx() ([]byte, error) {
	return payload.NewBytes(&payload.Payload{
		Connections: c.conns,
		Rate:        c.rate,
		Size:        c.size,
		Id:          c.id,
	})
}
