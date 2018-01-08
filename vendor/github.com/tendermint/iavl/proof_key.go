package iavl

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
)

// KeyProof represents a proof of existence or absence of a single key.
type KeyProof interface {
	// Verify verfies the proof is valid. To verify absence,
	// the value should be nil.
	Verify(key, value, root []byte) error

	// Root returns the root hash of the proof.
	Root() []byte

	// Serialize itself
	Bytes() []byte
}

// KeyExistsProof represents a proof of existence of a single key.
type KeyExistsProof struct {
	RootHash data.Bytes `json:"root_hash"`
	Version  uint64     `json:"version"`

	*PathToKey `json:"path"`
}

func (proof *KeyExistsProof) Root() []byte {
	return proof.RootHash
}

// Verify verifies the proof is valid and returns an error if it isn't.
func (proof *KeyExistsProof) Verify(key []byte, value []byte, root []byte) error {
	if !bytes.Equal(proof.RootHash, root) {
		return errors.WithStack(ErrInvalidRoot)
	}
	if key == nil || value == nil {
		return errors.WithStack(ErrInvalidInputs)
	}
	return proof.PathToKey.verify(proofLeafNode{key, value, proof.Version}, root)
}

// Bytes returns a go-wire binary serialization
func (proof *KeyExistsProof) Bytes() []byte {
	return wire.BinaryBytes(proof)
}

// ReadKeyExistsProof will deserialize a KeyExistsProof from bytes.
func ReadKeyExistsProof(data []byte) (*KeyExistsProof, error) {
	proof := new(KeyExistsProof)
	err := wire.ReadBinaryBytes(data, &proof)
	return proof, err
}

// KeyAbsentProof represents a proof of the absence of a single key.
type KeyAbsentProof struct {
	RootHash data.Bytes `json:"root_hash"`
	Version  uint64     `json:"version"`

	Left  *pathWithNode `json:"left"`
	Right *pathWithNode `json:"right"`
}

func (proof *KeyAbsentProof) Root() []byte {
	return proof.RootHash
}

func (p *KeyAbsentProof) String() string {
	return fmt.Sprintf("KeyAbsentProof\nroot=%s\nleft=%s%#v\nright=%s%#v\n", p.RootHash, p.Left.Path, p.Left.Node, p.Right.Path, p.Right.Node)
}

// Verify verifies the proof is valid and returns an error if it isn't.
func (proof *KeyAbsentProof) Verify(key, value []byte, root []byte) error {
	if !bytes.Equal(proof.RootHash, root) {
		return errors.WithStack(ErrInvalidRoot)
	}
	if key == nil || value != nil {
		return ErrInvalidInputs
	}

	if proof.Left == nil && proof.Right == nil {
		return errors.WithStack(ErrInvalidProof)
	}
	if err := verifyPaths(proof.Left, proof.Right, key, key, root); err != nil {
		return err
	}

	return verifyKeyAbsence(proof.Left, proof.Right)
}

// Bytes returns a go-wire binary serialization
func (proof *KeyAbsentProof) Bytes() []byte {
	return wire.BinaryBytes(proof)
}

// ReadKeyAbsentProof will deserialize a KeyAbsentProof from bytes.
func ReadKeyAbsentProof(data []byte) (*KeyAbsentProof, error) {
	proof := new(KeyAbsentProof)
	err := wire.ReadBinaryBytes(data, &proof)
	return proof, err
}
