package types

import "github.com/tendermint/tendermint/libs/bytes"

// Info about the status of the light client
type LightClientInfo struct {
	Primary           string         `json:"primary"`
	Witnesses         []string       `json:"witnesses"`
	NumPeers          int            `json:"number_of_peers"`
	LastTrustedHeight int64          `json:"last_trusted_height"`
	LastTrustedHash   bytes.HexBytes `json:"last_trusted_hash"`
}
