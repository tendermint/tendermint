package psql

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/adlio/schema"
	"github.com/gogo/protobuf/proto"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/util"
	"github.com/tendermint/tendermint/types"

	// Register the Postgres database driver.
	_ "github.com/lib/pq"
)

// Verify that the type satisfies the EventSink interface.
var _ indexer.EventSink = (*EventSink)(nil)

var (
	doPauseAtExit = flag.Bool("pause-at-exit", false,
		"If true, pause the test until interrupted at shutdown, to allow debugging")

	// A hook that test cases can call to obtain the shared database instance
	// used for testing the sink. This is initialized in TestMain (see below).
	testDB func() *sql.DB
)

const (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
	chainID  = "test-chainID"

	viewBlockEvents = "block_events"
	viewTxEvents    = "tx_events"

	sinkType = "postgres"
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Set up docker and start a container running PostgreSQL.
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	if err != nil {
		log.Fatalf("Creating docker pool: %v", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
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

	if *doPauseAtExit {
		log.Print("Pause at exit is enabled, containers will not expire")
	} else {
		const expireSeconds = 60
		_ = resource.Expire(expireSeconds)
		log.Printf("Container expiration set to %d seconds", expireSeconds)
	}

	// Connect to the database, clear any leftover data, and install the
	// indexing schema.
	conn := fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)
	var db *sql.DB

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

	// Set up the hook for tests to get the shared database handle.
	testDB = func() *sql.DB { return db }

	// Run the selected test cases.
	code := m.Run()

	// Clean up and shut down the database container.
	if *doPauseAtExit {
		log.Print("Testing complete, pausing for inspection. Send SIGINT to resume teardown")
		util.WaitForInterrupt()
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
	psqlSink := &EventSink{store: testDB(), chainID: chainID}
	assert.Equal(t, indexer.PSQL, psqlSink.Type())
}

func TestIndexing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("IndexBlockEvents", func(t *testing.T) {
		indexer := &EventSink{store: testDB(), chainID: chainID}
		require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))

		verifyBlock(t, 1)
		verifyBlock(t, 2)

		util.VerifyNotImplemented(t, "hasBlock", sinkType, func() (bool, error) { return indexer.HasBlock(1) })
		util.VerifyNotImplemented(t, "hasBlock", sinkType, func() (bool, error) { return indexer.HasBlock(2) })

		util.VerifyNotImplemented(t, "block search", sinkType, func() (bool, error) {
			v, err := indexer.SearchBlockEvents(ctx, nil)
			return v != nil, err
		})

		require.NoError(t, verifyTimeStamp(tableBlocks))

		// Attempting to reindex the same events should gracefully succeed.
		require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))
	})

	t.Run("IndexTxEvents", func(t *testing.T) {
		indexer := &EventSink{store: testDB(), chainID: chainID}

		txResult := util.TxResultWithEvents([]abci.Event{
			makeIndexedEvent("account.number", "1"),
			makeIndexedEvent("account.owner", "Ivan"),
			makeIndexedEvent("account.owner", "Yulieta"),

			{Type: "", Attributes: []abci.EventAttribute{{Key: "not_allowed", Value: "Vlad", Index: true}}},
		})
		require.NoError(t, indexer.IndexTxEvents([]*abci.TxResult{txResult}))

		txr, err := loadTxResult(types.Tx(txResult.Tx).Hash())
		require.NoError(t, err)
		assert.Equal(t, txResult, txr)

		require.NoError(t, verifyTimeStamp(tableTxResults))
		require.NoError(t, verifyTimeStamp(viewTxEvents))

		util.VerifyNotImplemented(t, "getTxByHash", sinkType, func() (bool, error) {
			txr, err := indexer.GetTxByHash(types.Tx(txResult.Tx).Hash())
			return txr != nil, err
		})
		util.VerifyNotImplemented(t, "tx search", sinkType, func() (bool, error) {
			txr, err := indexer.SearchTxEvents(ctx, nil)
			return txr != nil, err
		})

		// try to insert the duplicate tx events.
		err = indexer.IndexTxEvents([]*abci.TxResult{txResult})
		require.NoError(t, err)
	})
}

func TestStop(t *testing.T) {
	indexer := &EventSink{store: testDB()}
	require.NoError(t, indexer.Stop())
}

// newTestBlockHeader constructs a fresh copy of a block header containing
// known test values to exercise the indexer.
func newTestBlockHeader() types.EventDataNewBlockHeader {
	return types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultBeginBlock: abci.ResponseBeginBlock{
			Events: []abci.Event{
				makeIndexedEvent("begin_event.proposer", "FCAA001"),
				makeIndexedEvent("thingy.whatzit", "O.O"),
			},
		},
		ResultEndBlock: abci.ResponseEndBlock{
			Events: []abci.Event{
				makeIndexedEvent("end_event.foo", "100"),
				makeIndexedEvent("thingy.whatzit", "-.O"),
			},
		},
	}
}

// readSchema loads the indexing database schema file
func readSchema() ([]*schema.Migration, error) {
	const filename = "schema.sql"
	contents, err := os.ReadFile(filename)
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

func loadTxResult(hash []byte) (*abci.TxResult, error) {
	hashString := fmt.Sprintf("%X", hash)
	var resultData []byte
	if err := testDB().QueryRow(`
SELECT tx_result FROM `+tableTxResults+` WHERE tx_hash = $1;
`, hashString).Scan(&resultData); err != nil {
		return nil, fmt.Errorf("lookup transaction for hash %q failed: %v", hashString, err)
	}

	txr := new(abci.TxResult)
	if err := proto.Unmarshal(resultData, txr); err != nil {
		return nil, fmt.Errorf("unmarshaling txr: %v", err)
	}

	return txr, nil
}

func verifyTimeStamp(tableName string) error {
	return testDB().QueryRow(fmt.Sprintf(`
SELECT DISTINCT %[1]s.created_at
  FROM %[1]s
  WHERE %[1]s.created_at >= $1;
`, tableName), time.Now().Add(-2*time.Second)).Err()
}

func verifyBlock(t *testing.T, height int64) {
	// Check that the blocks table contains an entry for this height.
	if err := testDB().QueryRow(`
SELECT height FROM `+tableBlocks+` WHERE height = $1;
`, height).Err(); err == sql.ErrNoRows {
		t.Errorf("No block found for height=%d", height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	// Verify the presence of begin_block and end_block events.
	if err := testDB().QueryRow(`
SELECT type, height, chain_id FROM `+viewBlockEvents+`
  WHERE height = $1 AND type = $2 AND chain_id = $3;
`, height, types.EventTypeBeginBlock, chainID).Err(); err == sql.ErrNoRows {
		t.Errorf("No %q event found for height=%d", types.EventTypeBeginBlock, height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	if err := testDB().QueryRow(`
SELECT type, height, chain_id FROM `+viewBlockEvents+`
  WHERE height = $1 AND type = $2 AND chain_id = $3;
`, height, types.EventTypeEndBlock, chainID).Err(); err == sql.ErrNoRows {
		t.Errorf("No %q event found for height=%d", types.EventTypeEndBlock, height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}
}
