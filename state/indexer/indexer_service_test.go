package indexer_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/adlio/schema"
	_ "github.com/lib/pq"
	dockertest "github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	indexer "github.com/tendermint/tendermint/state/indexer"
	kv "github.com/tendermint/tendermint/state/indexer/sink/kv"
	psql "github.com/tendermint/tendermint/state/indexer/sink/psql"
	"github.com/tendermint/tendermint/types"
	db "github.com/tendermint/tm-db"
)

var psqldb *sql.DB
var pSink indexer.EventSink

var (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: psql.DriverName,
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
		log.Fatalf("Could not start resource: %s", err)
	}

	// Set the container to expire in a minute to avoid orphaned containers
	// hanging around
	_ = resource.Expire(60)

	dsn = fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)
	if err = pool.Retry(func() error {
		var err error

		pSink, psqldb, err = psql.NewEventSink(dsn)
		if err != nil {
			return err
		}

		return psqldb.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	resetDB()

	sm, err := readSchema()
	if err != nil {
		psqldb.Close()
		log.Fatalf("Could not read schema: %s", err)
	}

	err = schema.NewMigrator().Apply(psqldb, sm)
	if err != nil {
		psqldb.Close()
		log.Fatalf("Could not apply schema to db: %s", err)
	}

	code := m.Run()

	// When you're done, kill and remove the container
	if err = pool.Purge(resource); err != nil {
		psqldb.Close()
		log.Fatalf("Could not purge resource: %s", err)
	}

	psqldb.Close()
	os.Exit(code)
}

func TestIndexerServiceIndexesBlocks(t *testing.T) {
	// event bus
	eventBus := types.NewEventBus()
	eventBus.SetLogger(tmlog.TestingLogger())
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	// event sink setup
	store := db.NewMemDB()
	eventSinks := []indexer.EventSink{kv.NewEventSink(store), pSink}

	service := indexer.NewIndexerService(eventSinks, eventBus)
	service.SetLogger(tmlog.TestingLogger())
	err = service.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := service.Stop(); err != nil {
			t.Error(err)
		}
	})

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
		Result: abci.ResponseDeliverTx{Code: 0},
	}
	err = eventBus.PublishEventTx(types.EventDataTx{TxResult: *txResult1})
	require.NoError(t, err)
	txResult2 := &abci.TxResult{
		Height: 1,
		Index:  uint32(1),
		Tx:     types.Tx("bar"),
		Result: abci.ResponseDeliverTx{Code: 0},
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
}

func readSchema() ([]*schema.Migration, error) {

	filename := "./sink/psql/schema.sql"
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql file from '%s': %w", filename, err)
	}

	mg := &schema.Migration{}
	mg.ID = time.Now().Local().String() + " db schema"
	mg.Script = string(contents)
	return append([]*schema.Migration{}, mg), nil
}

func resetDB() {
	q := "DROP TABLE IF EXISTS block_events,tx_events,tx_results"
	_, err := psqldb.Exec(q)
	if err != nil {
		psqldb.Close()
		log.Fatalf("Could not reset TABLE: %s", err)
	}

	q = "DROP TYPE IF EXISTS block_event_type"
	_, err = psqldb.Exec(q)
	if err != nil {
		psqldb.Close()
		log.Fatalf("Could not reset TYPE: %s", err)
	}
}
