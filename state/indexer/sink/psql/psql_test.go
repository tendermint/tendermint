package psql

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/adlio/schema"
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

var (
	doDebug = flag.Bool("debug", false, "If true, pause at teardown")

	db *sql.DB
)

const (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
	chainID  = "test-chainID"
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Set up docker and start a container running PostgreSQL.
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	if err != nil {
		log.Fatalf("Creating docker pool: %v", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
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
	if err != nil {
		log.Fatalf("Starting docker pool: %v", err)
	}

	if *doDebug {
		log.Print("Manual debugging is enabled, containers will not expire")
	} else {
		const expireSeconds = 60
		_ = resource.Expire(expireSeconds)
		log.Printf("Container expiration set to %d seconds", expireSeconds)
	}

	// Connect to the database, clear any leftover data, and install the
	// indexing schema.
	conn := fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)

	if err := pool.Retry(func() error {
		sink, err := NewEventSink(conn, chainID)
		if err != nil {
			return err
		}
		db = sink.DB() // set global for test use
		return db.Ping()
	}); err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}

	if err := resetDatabase(db); err != nil {
		log.Fatalf("Flushing database: %v", err)
	}

	sm, err := readSchema()
	if err != nil {
		log.Fatalf("Reading schema: %v", err)
	} else if err := schema.NewMigrator().Apply(db, sm); err != nil {
		log.Fatalf("Applying schema: %v", err)
	}

	// Run the selected test cases.
	code := m.Run()

	// Clean up and shut down the database container.
	if *doDebug {
		log.Print("Testing complete, send SIGINT to resume teardown")
		waitForInterrupt()
		log.Print("(resuming)")
	}
	log.Print("Shutting down database")
	if err := pool.Purge(resource); err != nil {
		log.Printf("WARNING: Purging pool failed: %v", err)
	}
	if err := db.Close(); err != nil {
		log.Printf("WARNING: Closing database failed: %v", err)
	}

	os.Exit(code)
}

func TestType(t *testing.T) {
	psqlSink := &EventSink{store: db, chainID: chainID}
	assert.Equal(t, indexer.PSQL, psqlSink.Type())
}

func TestBlockFuncs(t *testing.T) {
	indexer := &EventSink{store: db, chainID: chainID}
	require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))

	verifyBlock(t, 1)
	verifyBlock(t, 2)

	verifyNotImplemented(t, "hasBlock", func() (bool, error) { return indexer.HasBlock(1) })
	verifyNotImplemented(t, "hasBlock", func() (bool, error) { return indexer.HasBlock(2) })

	verifyNotImplemented(t, "block search", func() (bool, error) {
		v, err := indexer.SearchBlockEvents(context.Background(), nil)
		return v != nil, err
	})

	require.NoError(t, verifyTimeStamp(tableBlocks))

	// Attempting to reindex the same events should gracefully succeed.
	require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))
}

func TestTxFuncs(t *testing.T) {
	indexer := &EventSink{store: db, chainID: chainID}

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: "number", Value: "1", Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: "owner", Value: "Ivan", Index: true}}},
		{Type: "", Attributes: []abci.EventAttribute{{Key: "not_allowed", Value: "Vlad", Index: true}}},
	})
	require.NoError(t, indexer.IndexTxEvents([]*abci.TxResult{txResult}))

	txr, err := loadTxResult(types.Tx(txResult.Tx).Hash())
	require.NoError(t, err)
	assert.Equal(t, txResult, txr)

	require.NoError(t, verifyTimeStamp(tableTxResults))
	require.NoError(t, verifyTimeStamp(viewTxEvents))

	verifyNotImplemented(t, "getTxByHash", func() (bool, error) {
		txr, err := indexer.GetTxByHash(types.Tx(txResult.Tx).Hash())
		return txr != nil, err
	})
	verifyNotImplemented(t, "tx search", func() (bool, error) {
		txr, err := indexer.SearchTxEvents(context.Background(), nil)
		return txr != nil, err
	})

	// try to insert the duplicate tx events.
	err = indexer.IndexTxEvents([]*abci.TxResult{txResult})
	require.NoError(t, err)
}

func TestStop(t *testing.T) {
	indexer := &EventSink{store: db}
	require.NoError(t, indexer.Stop())
}

// newTestBlockHeader constructs a fresh copy of a block header containing
// known test values to exercise the indexer.
func newTestBlockHeader() types.EventDataNewBlockHeader {
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

// readSchema loads the indexing database schema file
func readSchema() ([]*schema.Migration, error) {
	const filename = "schema.sql"
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql file from '%s': %w", filename, err)
	}

	return []*schema.Migration{{
		ID:     time.Now().Local().String() + " db schema",
		Script: string(contents),
	}}, nil
}

// resetDB drops all the data from the test database.
func resetDatabase(db *sql.DB) error {
	_, err := db.Exec(`DROP TABLE IF EXISTS blocks,tx_results,events,attributes CASCADE;`)
	if err != nil {
		return fmt.Errorf("dropping tables: %v", err)
	}
	_, err = db.Exec(`DROP VIEW IF EXISTS event_attributes,block_events,tx_events CASCADE;`)
	if err != nil {
		return fmt.Errorf("dropping views: %v", err)
	}
	return nil
}

// txResultWithEvents constructs a fresh transaction result with fixed values
// for testing, that includes the specified events.
func txResultWithEvents(events []abci.Event) *abci.TxResult {
	return &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     types.Tx("HELLO WORLD"),
		Result: abci.ResponseDeliverTx{
			Data:   []byte{0},
			Code:   abci.CodeTypeOK,
			Log:    "",
			Events: events,
		},
	}
}

func loadTxResult(hash []byte) (*abci.TxResult, error) {
	hashString := fmt.Sprintf("%X", hash)
	var resultData []byte
	if err := db.QueryRow(`
SELECT tx_result FROM `+tableTxResults+`
  WHERE tx_hash = $1 AND chain_id = $2
`, hashString, chainID).Scan(&resultData); err != nil {
		return nil, fmt.Errorf("lookup transaction for hash %q failed: %v", hashString, err)
	}

	txr := new(abci.TxResult)
	if err := proto.Unmarshal(resultData, txr); err != nil {
		return nil, fmt.Errorf("unmarshaling txr: %v", err)
	}

	return txr, nil
}

func verifyTimeStamp(tableName string) error {
	return db.QueryRow(fmt.Sprintf(`
SELECT DISTINCT %[1]s.created_at
  FROM %[1]s
  WHERE %[1]s.created_at >= $1;
`, tableName), time.Now().Add(-2*time.Second)).Err()
}

func verifyBlock(t *testing.T, height int64) {
	// Check that the blocks table contains an entry for this height.
	if err := db.QueryRow(`
SELECT height FROM `+tableBlocks+` WHERE height = $1;
`, height).Err(); err == sql.ErrNoRows {
		t.Errorf("No block found for height=%d", height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	// Verify the presence of begin_block and end_block events.
	if err := db.QueryRow(`
SELECT type, height, chain_id FROM `+viewBlockEvents+`
  WHERE height = $1 AND type = $2 AND chain_id = $3;
`, height, types.EventTypeBeginBlock, chainID).Err(); err == sql.ErrNoRows {
		t.Errorf("No %q event found for height=%d", types.EventTypeBeginBlock, height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	if err := db.QueryRow(`
SELECT type, height, chain_id FROM `+viewBlockEvents+`
  WHERE height = $1 AND type = $2 AND chain_id = $3;
`, height, types.EventTypeEndBlock, chainID).Err(); err == sql.ErrNoRows {
		t.Errorf("No %q event found for height=%d", types.EventTypeEndBlock, height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}
}

// verifyNotImplemented calls f and verifies that it returns both a
// false-valued flag and a non-nil error whose string matching the expected
// "not supported" message with label prefixed.
func verifyNotImplemented(t *testing.T, label string, f func() (bool, error)) {
	t.Helper()
	t.Logf("Verifying that %q reports it is not implemented", label)

	want := label + " is not supported via the postgres event sink"
	ok, err := f()
	assert.False(t, ok)
	require.NotNil(t, err)
	assert.Equal(t, want, err.Error())
}

// waitForInterrupt blocks until a SIGINT is received by the process.
func waitForInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	select {
	case <-ch:
		return
	}
}
