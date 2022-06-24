package types

import (
	"bytes"
	"errors"
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	errExtensionSignEmpty  = errors.New("vote extension signature is missing")
	errExtensionSignTooBig = fmt.Errorf("vote extension signature is too big (max: %d)", SignatureSize)
	errUnableCopySigns     = errors.New("unable copy signatures the sizes of extensions are not equal")
)

// VoteExtensions is a container where the key is vote-extension type and value is a list of VoteExtension
type VoteExtensions map[tmproto.VoteExtensionType][]VoteExtension

// NewVoteExtensionsFromABCIExtended returns vote-extensions container for given ExtendVoteExtension
func NewVoteExtensionsFromABCIExtended(exts []*abci.ExtendVoteExtension) VoteExtensions {
	voteExtensions := make(VoteExtensions)
	for _, ext := range exts {
		voteExtensions.Add(ext.Type, ext.Extension)
	}
	return voteExtensions
}

// Add creates and adds VoteExtension into a container by vote-extension type
func (e VoteExtensions) Add(t tmproto.VoteExtensionType, ext []byte) {
	e[t] = append(e[t], VoteExtension{Extension: ext})
}

// Validate returns error if an added vote-extension is invalid
func (e VoteExtensions) Validate() error {
	for _, et := range VoteExtensionTypes {
		for _, ext := range e[et] {
			err := ext.Validate()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// IsEmpty returns true if a vote-extension container is empty, otherwise false
func (e VoteExtensions) IsEmpty() bool {
	for _, exts := range e {
		if len(exts) > 0 {
			return false
		}
	}
	return true
}

// ToProto transforms the current state of vote-extension container into VoteExtensions's protobuf
func (e VoteExtensions) ToProto() []*tmproto.VoteExtension {
	extensions := make([]*tmproto.VoteExtension, 0, e.totalCount())
	for _, t := range VoteExtensionTypes {
		for _, ext := range e[t] {
			extensions = append(extensions, &tmproto.VoteExtension{
				Type:      t,
				Extension: ext.Extension,
				Signature: ext.Signature,
			})
		}
	}
	return extensions
}

// ToExtendProto transforms the current state of vote-extension container into ExtendVoteExtension's protobuf
func (e VoteExtensions) ToExtendProto() []*abci.ExtendVoteExtension {
	proto := make([]*abci.ExtendVoteExtension, 0, e.totalCount())
	for _, et := range VoteExtensionTypes {
		for _, ext := range e[et] {
			proto = append(proto, &abci.ExtendVoteExtension{
				Type:      et,
				Extension: ext.Extension,
			})
		}
	}
	return proto
}

// Fingerprint returns a fingerprint of all vote-extensions in a state of this container
func (e VoteExtensions) Fingerprint() []byte {
	cnt := 0
	for _, v := range e {
		cnt += len(v)
	}
	l := make([][]byte, 0, cnt)
	for _, et := range VoteExtensionTypes {
		for _, ext := range e[et] {
			l = append(l, ext.Extension)
		}
	}
	return tmbytes.Fingerprint(bytes.Join(l, nil))
}

// IsSameWithProto compares the current state of the vote-extension with the same in VoteExtensions's protobuf
// checks only the value of extensions
func (e VoteExtensions) IsSameWithProto(proto tmproto.VoteExtensions) bool {
	for t, extensions := range e {
		if len(proto[t]) != len(extensions) {
			return false
		}
		for i, ext := range extensions {
			if !bytes.Equal(ext.Extension, proto[t][i].Extension) {
				return false
			}
		}
	}
	return true
}

func (e VoteExtensions) totalCount() int {
	cnt := 0
	for _, exts := range e {
		cnt += len(exts)
	}
	return cnt
}

// VoteExtension represents a vote extension data, with possible types: default or threshold recover
type VoteExtension struct {
	Extension []byte           `json:"extension"`
	Signature tmbytes.HexBytes `json:"signature"`
}

// Validate ...
func (v *VoteExtension) Validate() error {
	if len(v.Extension) > 0 && len(v.Signature) == 0 {
		return errExtensionSignEmpty
	}
	if len(v.Signature) > SignatureSize {
		return errExtensionSignTooBig
	}
	return nil
}

// Clone returns a copy of current vote-extension
func (v *VoteExtension) Clone() VoteExtension {
	return VoteExtension{
		Extension: v.Extension,
		Signature: v.Signature,
	}
}

// VoteExtensionsFromProto creates VoteExtensions container from VoteExtensions's protobuf
func VoteExtensionsFromProto(pve []*tmproto.VoteExtension) VoteExtensions {
	if pve == nil {
		return nil
	}
	voteExtensions := make(VoteExtensions)
	for _, ext := range pve {
		voteExtensions[ext.Type] = append(voteExtensions[ext.Type], VoteExtension{
			Extension: ext.Extension,
			Signature: ext.Signature,
		})
	}
	return voteExtensions
}

// CopySignsFromProto copies the signatures from VoteExtensions's protobuf into the current VoteExtension state
func (e VoteExtensions) CopySignsFromProto(src tmproto.VoteExtensions) error {
	return e.copySigns(src, func(a *tmproto.VoteExtension, b *VoteExtension) {
		b.Signature = a.Signature
	})
}

// CopySignsToProto copies the signatures from the current VoteExtensions into VoteExtension's protobuf
func (e VoteExtensions) CopySignsToProto(dist tmproto.VoteExtensions) error {
	return e.copySigns(dist, func(a *tmproto.VoteExtension, b *VoteExtension) {
		a.Signature = b.Signature
	})
}

func (e VoteExtensions) copySigns(
	protoMap tmproto.VoteExtensions,
	modifier func(a *tmproto.VoteExtension, b *VoteExtension),
) error {
	for t, exts := range e {
		if len(exts) != len(protoMap[t]) {
			return errUnableCopySigns
		}
		for i := range exts {
			modifier(protoMap[t][i], &exts[i])
		}
	}
	return nil
}
