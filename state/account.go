package state

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"io"
)

// NOTE: consensus/Validator embeds this, so..
type Account struct {
	Id     uint64 // Numeric id of account, incrementing.
	PubKey []byte
}

func ReadAccount(r io.Reader, n *int64, err *error) *Account {
	return &Account{
		Id:     ReadUInt64(r, n, err),
		PubKey: ReadByteSlice(r, n, err),
	}
}

func (self *Account) Verify(msg []byte, sig []byte) bool {
	return false
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey []byte
}

func (self *PrivAccount) Sign(msg []byte) Signature {
	return Signature{}
}

/*
// Signs the URI, which includes all data and metadata.
// XXX implement or change
func (bp *BlockPart) Sign(acc *PrivAccount) {
	// TODO: populate Signature
}

// XXX maybe change.
func (bp *BlockPart) ValidateWithSigner(signer *Account) error {
	// TODO: Sanity check height, index, total, bytes, etc.
	if !signer.Verify([]byte(bp.URI()), bp.Signature.Bytes) {
		return ErrInvalidBlockPartSignature
	}
	return nil
}

*/
