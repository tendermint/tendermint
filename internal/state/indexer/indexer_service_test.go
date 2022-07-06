package indexer_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/adlio/schema"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/kv"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/psql"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"

	// Register the Postgre database driver.
	_ "github.com/lib/pq"
)

var psqldb *sql.DB
var resource *dockertest.Resource
var pSink indexer.EventSink

var (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
)

func TestIndexerServiceIndexesBlocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := tmlog.NewNopLogger()
	// event bus
	eventBus := eventbus.NewDefault(logger)
	err := eventBus.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(eventBus.Wait)

	assert.False(t, indexer.KVSinkEnabled([]indexer.EventSink{}))
	assert.False(t, indexer.IndexingEnabled([]indexer.EventSink{}))

	// event sink setup
	pool := setupDB(t)

	store := dbm.NewMemDB()
	eventSinks := []indexer.EventSink{kv.NewEventSink(store), pSink}
	assert.True(t, indexer.KVSinkEnabled(eventSinks))
	assert.True(t, indexer.IndexingEnabled(eventSinks))

	service := indexer.NewService(indexer.ServiceArgs{
		Logger:   logger,
		Sinks:    eventSinks,
		EventBus: eventBus,
	})
	require.NoError(t, service.Start(ctx))
	t.Cleanup(service.Wait)

	// publish block with txs
	err = eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		NumTxs: int64(2),
	})
	require.NoError(t, err)
	txResult1 := &abci.TxResult{
		Height: 1,
		Index:  uint32(0),
		Tx:     types.Tx("foo"),
		Result: abci.ExecTxResult{Code: 0},
	}
	err = eventBus.PublishEventTx(types.EventDataTx{TxResult: *txResult1})
	require.NoError(t, err)
	txResult2 := &abci.TxResult{
		Height: 1,
		Index:  uint32(1),
		Tx:     types.Tx("bar"),
		Result: abci.ExecTxResult{Code: 0},
	}
	err = eventBus.PublishEventTx(types.EventDataTx{TxResult: *txResult2})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	res, err := eventSinks[0].GetTxByHash(types.Tx("foo").Hash())
	require.NoError(t, err)
	require.Equal(t, txResult1, res)

	ok, err := eventSinks[0].HasBlock(1)
	require.NoError(t, err)
	require.True(t, ok)

	res, err = eventSinks[0].GetTxByHash(types.Tx("bar").Hash())
	require.NoError(t, err)
	require.Equal(t, txResult2, res)

	assert.Nil(t, teardown(t, pool))
}

func TestTxIndexDuplicatedTx(t *testing.T) {
	var mockTx = types.Tx("MOCK_TX_HASH")

	testCases := []struct {
		name    string
		tx1     abci.TxResult
		tx2     abci.TxResult
		expSkip bool // do we expect the second tx to be skipped by tx indexer
	}{
		{"skip, previously successful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			true,
		},
		{"not skip, previously unsuccessful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			false,
		},
		{"not skip, both successful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK,
				},
			},
			false,
		},
		{"not skip, both unsuccessful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			false,
		},
		{"skip, same block, previously successful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK,
				},
			},
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			true,
		},
		{"not skip, same block, previously unsuccessful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK + 1,
				},
			},
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ExecTxResult{
					Code: abci.CodeTypeOK,
				},
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink := kv.NewEventSink(dbm.NewMemDB())

			if tc.tx1.Height != tc.tx2.Height {
				// index the first tx
				err := sink.IndexTxEvents([]*abci.TxResult{&tc.tx1})
				require.NoError(t, err)

				// check if the second one should be skipped.
				ops, err := indexer.DeduplicateBatch([]*abci.TxResult{&tc.tx2}, sink)
				require.NoError(t, err)

				if tc.expSkip {
					require.Empty(t, ops)
				} else {
					require.Equal(t, []*abci.TxResult{&tc.tx2}, ops)
				}
			} else {
				// same block
				ops := []*abci.TxResult{&tc.tx1, &tc.tx2}
				ops, err := indexer.DeduplicateBatch(ops, sink)
				require.NoError(t, err)
				if tc.expSkip {
					// the second one is skipped
					require.Equal(t, []*abci.TxResult{&tc.tx1}, ops)
				} else {
					require.Equal(t, []*abci.TxResult{&tc.tx1, &tc.tx2}, ops)
				}
			}
		})
	}
}

func readSchema() ([]*schema.Migration, error) {
	filename := "./sink/psql/schema.sql"
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql file from '%s': %w", filename, err)
	}

	mg := &schema.Migration{}
	mg.ID = time.Now().Local().String() + " db schema"
	mg.Script = string(contents)
	return append([]*schema.Migration{}, mg), nil
}

func resetDB(t *testing.T) {
	q := "DROP TABLE IF EXISTS block_events,tx_events,tx_results"
	_, err := psqldb.Exec(q)
	assert.NoError(t, err)

	q = "DROP TYPE IF EXISTS block_event_type"
	_, err = psqldb.Exec(q)
	assert.NoError(t, err)
}

func setupDB(t *testing.T) *dockertest.Pool {
	t.Helper()
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	assert.NoError(t, err)
	if _, err := pool.Client.Info(); err != nil {
		t.Skipf("WARNING: Docker is not available: %v [skipping this test]", err)
	}

	resource, err = pool.RunWithOptions(&dockertest.RunOptions{
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

	assert.NoError(t, err)

	// Set the container to expire in a minute to avoid orphaned containers
	// hanging around
	_ = resource.Expire(60)

	conn := fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)

	assert.NoError(t, pool.Retry(func() error {
		sink, err := psql.NewEventSink(conn, "test-chainID")
		if err != nil {
			return err
		}

		pSink = sink
		psqldb = sink.DB()
		return psqldb.Ping()
	}))

	resetDB(t)

	sm, err := readSchema()
	assert.NoError(t, err)

	migrator := schema.NewMigrator()
	err = migrator.Apply(psqldb, sm)
	assert.NoError(t, err)

	return pool
}

func teardown(t *testing.T, pool *dockertest.Pool) error {
	t.Helper()
	// When you're done, kill and remove the container
	assert.Nil(t, pool.Purge(resource))
	return psqldb.Close()
}
