package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	schema "github.com/adlio/schema"
	proto "github.com/gogo/protobuf/proto"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"

	// Register the Postgres database driver.
	_ "github.com/lib/pq"
)

// Verify that the type satisfies the EventSink interface.
var _ indexer.EventSink = (*EventSink)(nil)

var db *sql.DB
var resource *dockertest.Resource
var chainID = "test-chainID"

const (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
)

func TestType(t *testing.T) {
	pool := mustSetupDB(t)
	psqlSink := &EventSink{store: db, chainID: chainID}
	assert.Equal(t, indexer.PSQL, psqlSink.Type())
	mustTeardown(t, pool)
}

func TestBlockFuncs(t *testing.T) {
	pool := mustSetupDB(t)

	indexer := &EventSink{store: db, chainID: chainID}
	require.NoError(t, indexer.IndexBlockEvents(getTestBlockHeader()))

	r, err := verifyBlock(1)
	assert.True(t, r)
	require.NoError(t, err)

	r, err = verifyBlock(2)
	assert.False(t, r)
	require.NoError(t, err)

	r, err = indexer.HasBlock(1)
	assert.False(t, r)
	assert.Equal(t, errors.New("hasBlock is not supported via the postgres event sink"), err)

	r, err = indexer.HasBlock(2)
	assert.False(t, r)
	assert.Equal(t, errors.New("hasBlock is not supported via the postgres event sink"), err)

	r2, err := indexer.SearchBlockEvents(context.TODO(), nil)
	assert.Nil(t, r2)
	assert.Equal(t, errors.New("block search is not supported via the postgres event sink"), err)

	require.NoError(t, verifyTimeStamp(tableEventBlock))

	// try to insert the duplicate block events.
	err = indexer.IndexBlockEvents(getTestBlockHeader())
	require.NoError(t, err)

	mustTeardown(t, pool)
}

func TestTxFuncs(t *testing.T) {
	pool := mustSetupDB(t)

	indexer := &EventSink{store: db, chainID: chainID}

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: "number", Value: "1", Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: "owner", Value: "Ivan", Index: true}}},
		{Type: "", Attributes: []abci.EventAttribute{{Key: "not_allowed", Value: "Vlad", Index: true}}},
	})
	require.NoError(t, indexer.IndexTxEvents([]*abci.TxResult{txResult}))

	tx, err := verifyTx(types.Tx(txResult.Tx).Hash())
	require.NoError(t, err)
	assert.Equal(t, txResult, tx)

	require.NoError(t, verifyTimeStamp(tableEventTx))
	require.NoError(t, verifyTimeStamp(tableResultTx))

	tx, err = indexer.GetTxByHash(types.Tx(txResult.Tx).Hash())
	assert.Nil(t, tx)
	assert.Equal(t, errors.New("getTxByHash is not supported via the postgres event sink"), err)

	r2, err := indexer.SearchTxEvents(context.TODO(), nil)
	assert.Nil(t, r2)
	assert.Equal(t, errors.New("tx search is not supported via the postgres event sink"), err)

	// try to insert the duplicate tx events.
	err = indexer.IndexTxEvents([]*abci.TxResult{txResult})
	require.NoError(t, err)

	mustTeardown(t, pool)
}

func TestStop(t *testing.T) {
	pool := mustSetupDB(t)

	indexer := &EventSink{store: db}
	require.NoError(t, indexer.Stop())

	defer db.Close()
	require.NoError(t, pool.Purge(resource))
}

func getTestBlockHeader() types.EventDataNewBlockHeader {
	return types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultBeginBlock: abci.ResponseBeginBlock{
			Events: []abci.Event{
				{
					Type: "begin_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   "proposer",
							Value: "FCAA001",
							Index: true,
						},
					},
				},
			},
		},
		ResultEndBlock: abci.ResponseEndBlock{
			Events: []abci.Event{
				{
					Type: "end_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   "foo",
							Value: "100",
							Index: true,
						},
					},
				},
			},
		},
	}
}

func readSchema() ([]*schema.Migration, error) {

	filename := "schema.sql"
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql file from '%s': %w", filename, err)
	}

	mg := &schema.Migration{}
	mg.ID = time.Now().Local().String() + " db schema"
	mg.Script = string(contents)
	return append([]*schema.Migration{}, mg), nil
}

func resetDB(t *testing.T) {
	_, err := db.Exec(`DROP TABLE IF EXISTS blocks,tx_results,events,attributes CASCADE;`)
	require.NoError(t, err)

	_, err = db.Exec(`DELETE VIEW IF EXISTS event_attributes,block_events,tx_events CASCADE;`)
	require.NoError(t, err)
}

func txResultWithEvents(events []abci.Event) *abci.TxResult {
	tx := types.Tx("HELLO WORLD")
	return &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     tx,
		Result: abci.ResponseDeliverTx{
			Data:   []byte{0},
			Code:   abci.CodeTypeOK,
			Log:    "",
			Events: events,
		},
	}
}

func verifyTx(hash []byte) (*abci.TxResult, error) {
	join := fmt.Sprintf("%s ON %s.id = tx_result_id", tableEventTx, tableResultTx)
	sqlStmt := sq.
		Select("tx_result", fmt.Sprintf("%s.id", tableResultTx), "tx_result_id", "hash", "chain_id").
		Distinct().From(tableResultTx).
		InnerJoin(join).
		Where(fmt.Sprintf("hash = $1 AND chain_id = '%s'", chainID), fmt.Sprintf("%X", hash))

	rows, err := sqlStmt.RunWith(db).Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var txResult []byte
		var txResultID, txid int
		var h, cid string
		err = rows.Scan(&txResult, &txResultID, &txid, &h, &cid)
		if err != nil {
			return nil, nil
		}

		msg := new(abci.TxResult)
		err = proto.Unmarshal(txResult, msg)
		if err != nil {
			return nil, err
		}

		return msg, err
	}

	// No result
	return nil, nil
}

func verifyTimeStamp(tableName string) error {
	return db.QueryRow(fmt.Sprintf(`
SELECT DISTINCT %[1]s.created_at
  FROM %[1]s
  WHERE %[1]s.created_at >= ?;
`, tableName), time.Now().Add(-2*time.Second)).Err()
}

func verifyBlock(h int64) (bool, error) {
	sqlStmt := sq.
		Select("height").
		Distinct().
		From(tableEventBlock).
		Where(fmt.Sprintf("height = %d", h))
	rows, err := sqlStmt.RunWith(db).Query()
	if err != nil {
		return false, err
	}

	defer rows.Close()

	if !rows.Next() {
		return false, nil
	}

	sqlStmt = sq.
		Select("type, height", "chain_id").
		Distinct().
		From(tableEventBlock).
		Where(fmt.Sprintf("height = %d AND type = '%s' AND chain_id = '%s'", h, types.EventTypeBeginBlock, chainID))

	rows, err = sqlStmt.RunWith(db).Query()
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return false, nil
	}

	sqlStmt = sq.
		Select("type, height").
		Distinct().
		From(tableEventBlock).
		Where(fmt.Sprintf("height = %d AND type = '%s'", h, types.EventTypeEndBlock))
	rows, err = sqlStmt.RunWith(db).Query()

	if err != nil {
		return false, err
	}
	defer rows.Close()

	return rows.Next(), nil
}

func mustSetupDB(t *testing.T) *dockertest.Pool {
	t.Helper()
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	require.NoError(t, err)

	resource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: driverName,
		Tag:        "13",
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + dbName,
			"listen_addresses = '*'",
		},
		ExposedPorts: []string{port},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})

	require.NoError(t, err)

	// Set the container to expire in a minute to avoid orphaned containers
	// hanging around
	_ = resource.Expire(60)

	conn := fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)

	require.NoError(t, pool.Retry(func() error {
		sink, err := NewEventSink(conn, chainID)
		if err != nil {
			return err
		}
		db = sink.DB() // set global for test use
		return db.Ping()
	}))

	resetDB(t)

	sm, err := readSchema()
	require.Nil(t, err)
	require.Nil(t, schema.NewMigrator().Apply(db, sm))
	return pool
}

// mustTeardown purges the pool and closes the test database.
func mustTeardown(t *testing.T, pool *dockertest.Pool) {
	t.Helper()
	require.Nil(t, pool.Purge(resource))
	require.Nil(db.Close())
}
