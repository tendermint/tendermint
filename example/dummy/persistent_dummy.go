package dummy

import (
	"bytes"

	. "github.com/tendermint/go-common"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

//-----------------------------------------

type PersistentDummyApplication struct {
	app *DummyApplication
	db  dbm.DB

	// latest received
	blockHash   []byte
	blockHeader *types.Header
}

func NewPersistentDummyApplication(dbDir string) *PersistentDummyApplication {
	db := dbm.NewDB("dummy", "leveldb", dbDir)
	lastBlock := LoadLastBlock(db)

	stateTree := merkle.NewIAVLTree(
		0,
		db,
	)
	stateTree.Load(lastBlock.AppHash)

	log.Notice("Loaded state", "block", lastBlock.BlockHeight, "root", stateTree.Hash())

	return &PersistentDummyApplication{
		app: &DummyApplication{state: stateTree},
		db:  db,
	}
}

func (app *PersistentDummyApplication) Info() (string, *types.TMSPInfo, *types.LastBlockInfo, *types.ConfigInfo) {
	s, _, _, _ := app.app.Info()
	lastBlock := LoadLastBlock(app.db)
	return s, nil, &lastBlock, nil
}

func (app *PersistentDummyApplication) SetOption(key string, value string) (log string) {
	return app.app.SetOption(key, value)
}

// tx is either "key=value" or just arbitrary bytes
func (app *PersistentDummyApplication) AppendTx(tx []byte) types.Result {
	return app.app.AppendTx(tx)
}

func (app *PersistentDummyApplication) CheckTx(tx []byte) types.Result {
	return app.app.CheckTx(tx)
}

func (app *PersistentDummyApplication) Commit() types.Result {
	// Save
	appHash := app.app.state.Save()
	log.Info("Saved state", "root", appHash)

	lastBlock := types.LastBlockInfo{
		BlockHeight: app.blockHeader.Height,
		BlockHash:   app.blockHash,
		AppHash:     appHash, // this hash will be in the next block header
	}
	SaveLastBlock(app.db, lastBlock)
	return types.NewResultOK(appHash, "")
}

func (app *PersistentDummyApplication) Query(query []byte) types.Result {
	return app.app.Query(query)
}

func (app *PersistentDummyApplication) InitChain(validators []*types.Validator) {
	return
}

func (app *PersistentDummyApplication) BeginBlock(hash []byte, header *types.Header) {
	app.blockHash = hash
	app.blockHeader = header
}

func (app *PersistentDummyApplication) EndBlock(height uint64) (diffs []*types.Validator) {
	return nil
}

//-----------------------------------------
// persist the last block info

var lastBlockKey = []byte("lastblock")

// Get the last block from the db
func LoadLastBlock(db dbm.DB) (lastBlock types.LastBlockInfo) {
	buf := db.Get(lastBlockKey)
	if len(buf) != 0 {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&lastBlock, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			Exit(Fmt("Data has been corrupted or its spec has changed: %v\n", *err))
		}
		// TODO: ensure that buf is completely read.
	}

	return lastBlock
}

func SaveLastBlock(db dbm.DB, lastBlock types.LastBlockInfo) {
	log.Notice("Saving block", "height", lastBlock.BlockHeight, "hash", lastBlock.BlockHash, "root", lastBlock.AppHash)
	buf, n, err := new(bytes.Buffer), new(int), new(error)
	wire.WriteBinary(lastBlock, buf, n, err)
	if *err != nil {
		// TODO
		PanicCrisis(*err)
	}
	db.Set(lastBlockKey, buf.Bytes())
}
