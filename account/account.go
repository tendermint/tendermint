package account

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tendermint/go-wire"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

// Signable is an interface for all signable things.
// It typically removes signatures before serializing.
type Signable interface {
	WriteSignBytes(chainID string, w io.Writer, n *int64, err *error)
}

// SignBytes is a convenience method for getting the bytes to sign of a Signable.
func SignBytes(chainID string, o Signable) []byte {
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	o.WriteSignBytes(chainID, buf, n, err)
	if *err != nil {
		PanicCrisis(err)
	}
	return buf.Bytes()
}

// HashSignBytes is a convenience method for getting the hash of the bytes of a signable
func HashSignBytes(chainID string, o Signable) []byte {
	return merkle.SimpleHashFromBinary(SignBytes(chainID, o))
}

//-----------------------------------------------------------------------------

// Account resides in the application state, and is mutated by transactions
// on the blockchain.
// Serialized by wire.[read|write]Reflect
type Account struct {
	Address     []byte `json:"address"`
	PubKey      PubKey `json:"pub_key"`
	Sequence    int    `json:"sequence"`
	Balance     int64  `json:"balance"`
	Code        []byte `json:"code"`         // VM code
	StorageRoot []byte `json:"storage_root"` // VM storage merkle root.

	Permissions ptypes.AccountPermissions `json:"permissions"`
}

func (acc *Account) Copy() *Account {
	accCopy := *acc
	return &accCopy
}

func (acc *Account) String() string {
	if acc == nil {
		return "nil-Account"
	}
	return fmt.Sprintf("Account{%X:%v B:%v C:%v S:%X P:%s}", acc.Address, acc.PubKey, acc.Balance, len(acc.Code), acc.StorageRoot, acc.Permissions)
}

func AccountEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	wire.WriteBinary(o.(*Account), w, n, err)
}

func AccountDecoder(r io.Reader, n *int64, err *error) interface{} {
	return wire.ReadBinary(&Account{}, r, n, err)
}

var AccountCodec = wire.Codec{
	Encode: AccountEncoder,
	Decode: AccountDecoder,
}
