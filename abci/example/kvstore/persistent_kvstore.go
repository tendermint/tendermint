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
	app *Application
}

func NewPersistentKVStoreApplication(logger log.Logger, dbDir string) *PersistentKVStoreApplication {
	db, err := dbm.NewGoLevelDB("kvstore", dbDir)
	if err != nil {
		panic(err)
	}

	return &PersistentKVStoreApplication{
		app: &Application{
			valAddrToPubKeyMap: make(map[string]cryptoproto.PublicKey),
			state:              loadState(db),
			logger:             logger,
		},
	}
}

func (app *PersistentKVStoreApplication) Close() error {
	return app.app.Close()
}

func (app *PersistentKVStoreApplication) Info(req types.RequestInfo) types.ResponseInfo {
	return app.app.Info(req)
}

func (app *PersistentKVStoreApplication) handleTx(tx []byte) *types.ResponseDeliverTx {
	return app.app.handleTx(tx)
}

func (app *PersistentKVStoreApplication) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	return app.app.CheckTx(req)
}

// Commit will panic if InitChain was not called
func (app *PersistentKVStoreApplication) Commit() types.ResponseCommit {
	return app.app.Commit()
}

// When path=/val and data={validator address}, returns the validator update (types.ValidatorUpdate) varint encoded.
// For any other path, returns an associated value or nil if missing.
func (app *PersistentKVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	return app.app.Query(reqQuery)
}

// Save the validators in the merkle tree
func (app *PersistentKVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	return app.app.InitChain(req)
}

// Track the block hash and header information
// Execute transactions
// Update the validator set
func (app *PersistentKVStoreApplication) FinalizeBlock(req types.RequestFinalizeBlock) types.ResponseFinalizeBlock {
	return app.app.FinalizeBlock(req)
}

func (app *PersistentKVStoreApplication) ListSnapshots(req types.RequestListSnapshots) types.ResponseListSnapshots {
	return types.ResponseListSnapshots{}
}

func (app *PersistentKVStoreApplication) LoadSnapshotChunk(req types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	return types.ResponseLoadSnapshotChunk{}
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

func (app *PersistentKVStoreApplication) PrepareProposal(req types.RequestPrepareProposal) types.ResponsePrepareProposal {
	return app.app.PrepareProposal(req)
}

func (app *PersistentKVStoreApplication) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
	return app.app.ProcessProposal(req)
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
