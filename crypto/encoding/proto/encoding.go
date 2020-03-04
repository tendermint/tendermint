package proto

import (
	proto "github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
)

func MarshalPubKey(pki crypto.PubKey) ([]byte, error) {
	key := &PubKey{}
	if err := key.SetPubKey(pki); err != nil {
		return nil, err
	}
	return proto.Marshal(key)
}

func UnmarshalPubKey(bz []byte) (crypto.PubKey, error) {
	key := &PubKey{}

	err := proto.Unmarshal(bz, key)
	if err != nil {
		return nil, err
	}

	return key.GetPubKey(), nil
}

func MarshalPrivKey(pki crypto.PrivKey) ([]byte, error) {
	key := &PrivKey{}
	if err := key.SetPrivKey(pki); err != nil {
		return nil, err
	}
	return proto.Marshal(key)
}

func UnmarshalPrivKey(bz []byte) (crypto.PrivKey, error) {
	key := &PrivKey{}

	err := proto.Unmarshal(bz, key)
	if err != nil {
		return nil, err
	}

	return key.GetPrivKey(), nil
}
