package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/merkle"
	"io"
)

/* Block */

type Block struct {
	Header
	Validation
	Data
	// Checkpoint
}

func ReadBlock(r io.Reader) *Block {
	return &Block{
		Header:     ReadHeader(r),
		Validation: ReadValidation(r),
		Data:       ReadData(r),
	}
}

func (self *Block) Validate() bool {
	return false
}

func (self *Block) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(&self.Header, w, n, err)
	n, err = WriteOnto(&self.Validation, w, n, err)
	n, err = WriteOnto(&self.Data, w, n, err)
	return
}

/* Block > Header */

type Header struct {
	Name           String
	Height         UInt64
	Fees           UInt64
	Time           UInt64
	PrevHash       ByteSlice
	ValidationHash ByteSlice
	DataHash       ByteSlice
}

func ReadHeader(r io.Reader) Header {
	return Header{
		Name:           ReadString(r),
		Height:         ReadUInt64(r),
		Fees:           ReadUInt64(r),
		Time:           ReadUInt64(r),
		PrevHash:       ReadByteSlice(r),
		ValidationHash: ReadByteSlice(r),
		DataHash:       ReadByteSlice(r),
	}
}

func (self *Header) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Name, w, n, err)
	n, err = WriteOnto(self.Height, w, n, err)
	n, err = WriteOnto(self.Fees, w, n, err)
	n, err = WriteOnto(self.Time, w, n, err)
	n, err = WriteOnto(self.PrevHash, w, n, err)
	n, err = WriteOnto(self.ValidationHash, w, n, err)
	n, err = WriteOnto(self.DataHash, w, n, err)
	return
}

/* Block > Validation */

type Validation struct {
	Signatures  []Signature
	Adjustments []Adjustment
}

func ReadValidation(r io.Reader) Validation {
	numSigs := int(ReadUInt64(r))
	numAdjs := int(ReadUInt64(r))
	sigs := make([]Signature, 0, numSigs)
	for i := 0; i < numSigs; i++ {
		sigs = append(sigs, ReadSignature(r))
	}
	adjs := make([]Adjustment, 0, numAdjs)
	for i := 0; i < numAdjs; i++ {
		adjs = append(adjs, ReadAdjustment(r))
	}
	return Validation{
		Signatures:  sigs,
		Adjustments: adjs,
	}
}

func (self *Validation) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(UInt64(len(self.Signatures)), w, n, err)
	n, err = WriteOnto(UInt64(len(self.Adjustments)), w, n, err)
	for _, sig := range self.Signatures {
		n, err = WriteOnto(sig, w, n, err)
	}
	for _, adj := range self.Adjustments {
		n, err = WriteOnto(adj, w, n, err)
	}
	return
}

/* Block > Data */

type Data struct {
	Txs []Tx
}

func ReadData(r io.Reader) Data {
	numTxs := int(ReadUInt64(r))
	txs := make([]Tx, 0, numTxs)
	for i := 0; i < numTxs; i++ {
		txs = append(txs, ReadTx(r))
	}
	return Data{txs}
}

func (self *Data) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(UInt64(len(self.Txs)), w, n, err)
	for _, tx := range self.Txs {
		n, err = WriteOnto(tx, w, n, err)
	}
	return
}

func (self *Data) MerkleHash() ByteSlice {
	bs := make([]Binary, 0, len(self.Txs))
	for i, tx := range self.Txs {
		bs[i] = Binary(tx)
	}
	return merkle.HashFromBinarySlice(bs)
}
