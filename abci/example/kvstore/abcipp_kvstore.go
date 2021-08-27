package kvstore

import (
	"bytes"
	"strings"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
)

var _ types.Application = (*ABCIPPKVStoreApplication)(nil)

type ABCIPPKVStoreApplication struct {
	app *Application

  extension []byte

	logger log.Logger
}

func NewABCIPPKVStoreApplication(dbDir string) *ABCIPPKVStoreApplication {
	name := "kvstore"
	db, err := dbm.NewGoLevelDB(name, dbDir)
	if err != nil {
		panic(err)
	}

	state := loadState(db)

	return &ABCIPPKVStoreApplication{
		app:                &Application{state: state},
		logger:             log.NewNopLogger(),
	}
}

func (app *ABCIPPKVStoreApplication) Close() error {
	return app.app.state.db.Close()
}

func (app *ABCIPPKVStoreApplication) SetLogger(l log.Logger) {
	app.logger = l
}

func (app *ABCIPPKVStoreApplication) Info(req types.RequestInfo) types.ResponseInfo {
	res := app.app.Info(req)
	res.LastBlockHeight = app.app.state.Height
	res.LastBlockAppHash = app.app.state.AppHash
	return res
}

// tx is either "val:pubkey!power" or "key=value" or just arbitrary bytes
func (app *ABCIPPKVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
  // if it starts with "prepare:", use it as placeholder transaction for prepare proposal
  // format is "prepare:[null bytes]"
	if isPrepareTx(req.Tx) {
		return app.execPrepareTx(req.Tx)
	}

  // if it starts with "extension:", use it as vote extension signed data
  if isExtensionTx(req.Tx) {
    return app.execExtensionTx(req.Tx)
  }

	// otherwise, update the key-value store
	return app.app.DeliverTx(req)
}

func (app *ABCIPPKVStoreApplication) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	return app.app.CheckTx(req)
}

// Commit will panic if InitChain was not called
func (app *ABCIPPKVStoreApplication) Commit() types.ResponseCommit {
	return app.app.Commit()
}

// When path=/val and data={validator address}, returns the validator update (types.ValidatorUpdate) varint encoded.
// For any other path, returns an associated value or nil if missing.
func (app *ABCIPPKVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	switch reqQuery.Path {
    /*
	case "/val":
		key := []byte("val:" + string(reqQuery.Data))
		value, err := app.app.state.db.Get(key)
		if err != nil {
			panic(err)
		}

		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		return
    */
	default:
		return app.app.Query(reqQuery)
	}
}

// Save the validators in the merkle tree
func (app *ABCIPPKVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	return types.ResponseInitChain{}
}

// Track the block hash and header information
func (app *ABCIPPKVStoreApplication) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	return types.ResponseBeginBlock{}
}

// Update the validator set
func (app *ABCIPPKVStoreApplication) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
    return types.ResponseEndBlock{}
}

func (app *ABCIPPKVStoreApplication) ListSnapshots(
	req types.RequestListSnapshots) types.ResponseListSnapshots {
	return types.ResponseListSnapshots{}
}

func (app *ABCIPPKVStoreApplication) LoadSnapshotChunk(
	req types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	return types.ResponseLoadSnapshotChunk{}
}

func (app *ABCIPPKVStoreApplication) OfferSnapshot(
	req types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	return types.ResponseOfferSnapshot{Result: types.ResponseOfferSnapshot_ABORT}
}

func (app *ABCIPPKVStoreApplication) ApplySnapshotChunk(
	req types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	return types.ResponseApplySnapshotChunk{Result: types.ResponseApplySnapshotChunk_ABORT}
}

func (app *ABCIPPKVStoreApplication) ExtendVote(
	req types.RequestExtendVote) types.ResponseExtendVote {
  return types.RespondExtendVote(app.constructExtension(req.Vote.ValidatorAddress), nil)
}

func (app *ABCIPPKVStoreApplication) VerifyVoteExtension(
	req types.RequestVerifyVoteExtension) types.ResponseVerifyVoteExtension {
  return types.RespondVerifyVoteExtension(
    app.verifyExtension(req.Vote.ValidatorAddress, req.Vote.VoteExtension.AppDataToSign))
}

func (app *ABCIPPKVStoreApplication) PrepareProposal(
	req types.RequestPrepareProposal) types.ResponsePrepareProposal {
    return types.ResponsePrepareProposal{BlockData: app.substPrepareTx(req.BlockData)}
}

// -----------------------------

const PreparePrefix = "prepare"

func isPrepareTx(tx []byte) bool {
  return strings.HasPrefix(string(tx), PreparePrefix)
}

// execPrepareTx is noop. tx data is considered as placeholder
// and is substitute at the PrepareProposal.
func (app *ABCIPPKVStoreApplication) execPrepareTx(tx []byte) types.ResponseDeliverTx {
  // noop
  return types.ResponseDeliverTx{}
}

// substPrepareTx subst all the preparetx in the blockdata
// to null string(could be any arbitrary string).
func (app *ABCIPPKVStoreApplication) substPrepareTx(blockData [][]byte) [][]byte {
  for i, tx := range blockData {
    if isPrepareTx(tx) {
      blockData[i] = make([]byte, len(tx))
    }
  }

  return blockData
}

const ExtensionPrefix = "extension"

func isExtensionTx(tx []byte) bool {
  return strings.HasPrefix(string(tx), ExtensionPrefix)
}

// execExtensionTx stores the input string in the application struct
// which must be included in the VoteExtension.AppDataToSign
func (app *ABCIPPKVStoreApplication) execExtensionTx(tx []byte) types.ResponseDeliverTx {
  app.extension = []byte(strings.Split(string(tx), ":")[1])

  return types.ResponseDeliverTx{}
}

func (app *ABCIPPKVStoreApplication) constructExtension(valAddr []byte) []byte {
  ext := make([]byte, len(valAddr)+len(app.extension))
  copy(ext, valAddr)
  copy(ext[len(valAddr):], app.extension)
  return ext
}

func (app *ABCIPPKVStoreApplication) verifyExtension(valAddr []byte, ext []byte) bool {
  return bytes.Equal(app.constructExtension(valAddr), ext)
}
