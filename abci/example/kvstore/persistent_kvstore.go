package kvstore

import (
	"bytes"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	ptypes "github.com/tendermint/tendermint/proto/tendermint/types"
)

const (
	ValidatorSetChangePrefix string = "val:"
)

//-----------------------------------------

var _ types.Application = (*PersistentKVStoreApplication)(nil)

type PersistentKVStoreApplication struct {
	*Application
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

func (app *PersistentKVStoreApplication) OfferSnapshot(req types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	return types.ResponseOfferSnapshot{Result: types.ResponseOfferSnapshot_ABORT}
}

func (app *PersistentKVStoreApplication) ApplySnapshotChunk(req types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	return types.ResponseApplySnapshotChunk{Result: types.ResponseApplySnapshotChunk_ABORT}
}

func (app *PersistentKVStoreApplication) ExtendVote(req types.RequestExtendVote) types.ResponseExtendVote {
	return types.ResponseExtendVote{VoteExtension: ConstructVoteExtension(req.Vote.ValidatorAddress)}
}

func (app *PersistentKVStoreApplication) VerifyVoteExtension(req types.RequestVerifyVoteExtension) types.ResponseVerifyVoteExtension {
	return types.RespondVerifyVoteExtension(app.verifyExtension(req.Vote.ValidatorAddress, req.Vote.VoteExtension))
}

// -----------------------------

func ConstructVoteExtension(valAddr []byte) *ptypes.VoteExtension {
	return &ptypes.VoteExtension{
		AppDataToSign:             valAddr,
		AppDataSelfAuthenticating: valAddr,
	}
}

func (app *PersistentKVStoreApplication) verifyExtension(valAddr []byte, ext *ptypes.VoteExtension) bool {
	if ext == nil {
		return false
	}
	canonical := ConstructVoteExtension(valAddr)
	if !bytes.Equal(canonical.AppDataToSign, ext.AppDataToSign) {
		return false
	}
	if !bytes.Equal(canonical.AppDataSelfAuthenticating, ext.AppDataSelfAuthenticating) {
		return false
	}
	return true
}
