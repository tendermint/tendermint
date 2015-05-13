package account

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
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
		// SOMETHING HAS GONE HORRIBLY WRONG
		panic(err)
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
// Serialized by binary.[read|write]Reflect
type Account struct {
	Address     []byte `json:"address"`
	PubKey      PubKey `json:"pub_key"`
	Sequence    int    `json:"sequence"`
	Balance     int64  `json:"balance"`
	Code        []byte `json:"code"`         // VM code
	StorageRoot []byte `json:"storage_root"` // VM storage merkle root.

	Permissions Permissions `json:"permissions"`
}

func (acc *Account) Copy() *Account {
	accCopy := *acc
	return &accCopy
}

func (acc *Account) String() string {
	// return fmt.Sprintf("Account{%X:%v C:%v S:%X}", acc.Address, acc.PubKey, len(acc.Code), acc.StorageRoot)
	return fmt.Sprintf("Account{%X:%v C:%v S:%X P:(%s)}", acc.Address, acc.PubKey, len(acc.Code), acc.StorageRoot, acc.Permissions)
}

func AccountEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	binary.WriteBinary(o.(*Account), w, n, err)
}

func AccountDecoder(r io.Reader, n *int64, err *error) interface{} {
	return binary.ReadBinary(&Account{}, r, n, err)
}

var AccountCodec = binary.Codec{
	Encode: AccountEncoder,
	Decode: AccountDecoder,
}

//-----------------------------------------------------------------------------

var GlobalPermissionsAddress = LeftPadBytes([]byte{}, 20)
var DougAddress = GlobalPermissionsAddress

type Permission uint

const (
	SendPermission Permission = iota
	CallPermission
	CreatePermission
	BondPermission

	NumBasePermissions = 4
)

type Permissions struct {
	Send   bool
	Call   bool
	Create bool
	Bond   bool
	Other  []bool
}

func (p Permissions) Get(ty uint) (bool, error) {
	tyP := Permission(ty)
	switch tyP {
	case SendPermission:
		return p.Send, nil
	case CallPermission:
		return p.Call, nil
	case CreatePermission:
		return p.Create, nil
	case BondPermission:
		return p.Bond, nil
	default:
		ty = ty - 4
		if ty <= uint(len(p.Other)-1) {
			return p.Other[ty], nil
		}
		return false, fmt.Errorf("Unknown permission number %v", ty)
	}
}

func (p Permissions) Set(ty uint, val bool) error {
	tyP := Permission(ty)
	switch tyP {
	case SendPermission:
		p.Send = val
	case CallPermission:
		p.Call = val
	case CreatePermission:
		p.Create = val
	case BondPermission:
		p.Bond = val
	default:
		ty = ty - 4
		if ty <= uint(len(p.Other)-1) {
			p.Other[ty] = val
			return nil
		}
		return fmt.Errorf("Unknown permission number %v", ty)
	}
	return nil
}

// Add should be called on all accounts in tandem
func (p Permissions) Add(val bool) (uint, error) {
	l := len(p.Other)
	p.Other = append(p.Other, val)
	return uint(l), nil
}

// Remove should be called on all accounts in tandem
func (p Permissions) Remove(ty uint) error {
	if ty < uint(NumBasePermissions) || ty >= uint(len(p.Other)) {
		return fmt.Errorf("Invalid permission number %v", ty)
	}

	// pop the permission out of the array
	perms := p.Other[:ty]
	if ty+1 < uint(len(p.Other)) {
		perms = append(perms, p.Other[ty+1:]...)
	}
	p.Other = perms
	return nil
}

// defaults for a Big Bad Public Blockchain
var DefaultPermissions = Permissions{
	Send:   true,
	Call:   true,
	Create: true,
	Bond:   true,
}

func (p Permissions) String() string {
	return fmt.Sprintf("CanSend:%v, CanCall:%v, CanCreate:%v, CanBond:%v", p.Send, p.Call, p.Create, p.Bond)
}
