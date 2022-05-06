package kvstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/protoio"
	"github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
)

const ValidatorSetUpdatePrefix string = "vsu:"

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

type PersistentKVStoreApplication struct {
	*Application
	mtx sync.Mutex

	valUpdatesRepo      *repository
	ValidatorSetUpdates types.ValidatorSetUpdate
}

func NewPersistentKVStoreApplication(logger log.Logger, dbDir string) *PersistentKVStoreApplication {
	db, err := dbm.NewGoLevelDB("kvstore", dbDir)
	if err != nil {
		panic(err)
	}

	return &PersistentKVStoreApplication{
		Application: &Application{
			valAddrToPubKeyMap: make(map[string]cryptoproto.PublicKey),
			state:              loadState(db),
			logger:             logger,
		},
	}
}

func (app *PersistentKVStoreApplication) OfferSnapshot(_ context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	return &types.ResponseOfferSnapshot{Result: types.ResponseOfferSnapshot_ABORT}, nil
}

func (app *PersistentKVStoreApplication) ApplySnapshotChunk(_ context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	return &types.ResponseApplySnapshotChunk{Result: types.ResponseApplySnapshotChunk_ABORT}, nil
}

// MarshalValidatorSetUpdate encodes validator-set-update into protobuf, encode into base64 and add "vsu:" prefix
func MarshalValidatorSetUpdate(vsu *types.ValidatorSetUpdate) ([]byte, error) {
	pbData, err := proto.Marshal(vsu)
	if err != nil {
		return nil, err
	}
	return []byte(ValidatorSetUpdatePrefix + base64.StdEncoding.EncodeToString(pbData)), nil
}

// UnmarshalValidatorSetUpdate removes "vsu:" prefix and unmarshal a string into validator-set-update
func UnmarshalValidatorSetUpdate(data []byte) (*types.ValidatorSetUpdate, error) {
	l := len(ValidatorSetUpdatePrefix)
	data, err := base64.StdEncoding.DecodeString(string(data[l:]))
	if err != nil {
		return nil, err
	}
	vsu := new(types.ValidatorSetUpdate)
	err = proto.Unmarshal(data, vsu)
	return vsu, err
}

type repository struct {
	db dbm.DB
}

func (r *repository) set(vsu *types.ValidatorSetUpdate) error {
	data, err := proto.Marshal(vsu)
	if err != nil {
		return err
	}
	return r.db.Set([]byte(ValidatorSetUpdatePrefix), data)
}

func (r *repository) get() (*types.ValidatorSetUpdate, error) {
	data, err := r.db.Get([]byte(ValidatorSetUpdatePrefix))
	if err != nil {
		return nil, err
	}
	vsu := new(types.ValidatorSetUpdate)
	err = proto.Unmarshal(data, vsu)
	if err != nil {
		return nil, err
	}
	return vsu, nil
}

func isValidatorSetUpdateTx(tx []byte) bool {
	return strings.HasPrefix(string(tx), ValidatorSetUpdatePrefix)
}

func encodeMsg(data proto.Message) ([]byte, error) {
	buf := bytes.NewBufferString("")
	w := protoio.NewDelimitedWriter(buf)
	_, err := w.WriteMsg(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
