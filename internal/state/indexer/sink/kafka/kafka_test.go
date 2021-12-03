package kafka

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/go-zookeeper/zk"
	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/state/indexer/sink/util"
	"github.com/tendermint/tendermint/types"
)

var (
	// Verify that the type satisfies the EventSink interface.
	_ indexer.EventSink = (*EventSink)(nil)

	// A hook that test cases can call to obtain the shared database instance
	// used for testing the sink. This is initialized in TestMain (see below).
	testProducer func() sarama.SyncProducer

	doPauseAtExit = flag.Bool("pause-at-exit", false,
		"If true, pause the test until interrupted at shutdown, to allow debugging")
)

const (
	zkPort       = "2181"
	kafkaOutPort = "9093"
	kafkaInPort  = "9092"
	ip           = "localhost"
	protocol     = "tcp"
	chainID      = "test-chainID"
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Set up docker and start a container running zookeeper and kafka.
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	if err != nil {
		log.Fatalf("Creating docker pool: %v", err)
	}

	if err = pool.Client.Ping(); err != nil {
		log.Fatalf(`could not connect to docker: %s`, err)
	}

	network, err := pool.Client.CreateNetwork(docker.CreateNetworkOptions{Name: "zookeeper_kafka_network"})
	if err != nil {
		log.Fatalf("could not create a network to zookeeper and kafka: %s", err)
	}

	zkResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:         "zookeeper-EventSinkTest",
		Repository:   "wurstmeister/zookeeper",
		Tag:          "3.4.6",
		NetworkID:    network.ID,
		Hostname:     "zookeeper",
		ExposedPorts: []string{zkPort},
	})

	if err != nil {
		log.Fatalf("Could not start zookeeper: %s", err)
	}

	if *doPauseAtExit {
		log.Print("Pause at exit is enabled, containers will not expire")
	} else {
		const expireSeconds = 90
		_ = zkResource.Expire(expireSeconds)
		log.Printf("Container expiration set to %d seconds", expireSeconds)
	}

	zkConn, _, err := zk.Connect(
		[]string{fmt.Sprintf("%s:%s", ip, zkResource.GetPort(fmt.Sprintf("%s/%s", zkPort, protocol)))}, 10*time.Second)
	if err != nil {
		log.Fatalf("Could not connect to the zookeeper: %s", err)
	}
	defer zkConn.Close()

	if err != nil {
		log.Fatalf("Starting docker pool: %v", err)
	}

	retryFn := func() error {
		switch zkConn.State() {
		case zk.StateHasSession, zk.StateConnected:
			return nil
		default:
			return errors.New("Not yet connected")
		}
	}

	if err = pool.Retry(retryFn); err != nil {
		log.Fatalf("Could not connect to the zookeeper: %s", err)
	}

	portBinding := fmt.Sprintf("%s/tcp", kafkaOutPort)
	kafkaResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       "kafka-EventSinkTest",
		Repository: "wurstmeister/kafka",
		Tag:        "2.13-2.7.1",
		NetworkID:  network.ID,
		Hostname:   "kafka",
		Env: []string{
			fmt.Sprintf("KAFKA_ADVERTISED_LISTENERS=INSIDE://kafka:%s,OUTSIDE://%s:%s", kafkaInPort, ip, kafkaOutPort),
			fmt.Sprintf("KAFKA_LISTENERS=INSIDE://0.0.0.0:%s,OUTSIDE://0.0.0.0:%s", kafkaInPort, kafkaOutPort),
			fmt.Sprintf("KAFKA_ZOOKEEPER_CONNECT=zookeeper:%s", zkPort),
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=INSIDE:PLAINTEXT,OUTSIDE:PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME=INSIDE",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1",
		},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"9093/tcp": {{HostIP: ip, HostPort: portBinding}},
		},
		ExposedPorts: []string{fmt.Sprintf("%s/%s", kafkaOutPort, protocol)},
	})
	if err != nil {
		log.Fatalf("Could not get the kafkaResource: %s", err)
	}

	conn := fmt.Sprintf("%s:%s", ip, kafkaResource.GetPort(fmt.Sprintf("%s/%s", kafkaOutPort, protocol)))
	var producer sarama.SyncProducer

	if err := pool.Retry(func() error {
		sink, err := NewEventSink(conn, chainID)
		if err != nil {
			return err
		}

		producer = sink.Producer() // set global for test use
		return nil
	}); err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}

	// Set up the hook for tests to get the shared producer handle.
	testProducer = func() sarama.SyncProducer { return producer }

	// Run the selected test cases.
	code := m.Run()

	// Clean up and shut down the database container.
	if *doPauseAtExit {
		log.Print("Testing complete, pausing for inspection. Send SIGINT to resume teardown")
		util.WaitForInterrupt()
		log.Print("(resuming)")
	}

	if err := producer.Close(); err != nil {
		log.Printf("WARNING: Closing kafka producer failed: %v", err)
	}

	log.Print("Shutting down zookeeper&kafka service")
	if err := zkResource.Close(); err != nil {
		log.Printf("WARNING: could not close zookeeperResource: %v", err)
	}
	if err := pool.Purge(zkResource); err != nil {
		log.Printf("WARNING: could not purge zookeeperResource: %v", err)
	}

	if err := kafkaResource.Close(); err != nil {
		log.Printf("WARNING: could not close zookeeperResource: %v", err)
	}
	if err := pool.Purge(kafkaResource); err != nil {
		log.Printf("WARNING: could not purge zookeeperResource: %v", err)
	}
	if err := pool.Client.RemoveNetwork(network.ID); err != nil {
		log.Printf("WARNING: could not remove %s network: %s", network.Name, err)
	}

	os.Exit(code)
}

func TestType(t *testing.T) {
	kafkaSink := &EventSink{chainID: chainID, producer: testProducer()}
	assert.Equal(t, indexer.KAFKA, kafkaSink.Type())
}

func TestIndexing(t *testing.T) {
	t.Run("IndexBlockEvents", func(t *testing.T) {
		indexer := &EventSink{chainID: chainID, producer: testProducer()}
		err := indexer.IndexBlockEvents(newTestBlockHeader())
		require.NoError(t, err)

		verifyBlock(t, 1)

		util.VerifyNotImplemented(t, "hasBlock", string(indexer.Type()), func() (bool, error) {
			return indexer.HasBlock(1)
		})

		util.VerifyNotImplemented(t, "block search", string(indexer.Type()), func() (bool, error) {
			v, err := indexer.SearchBlockEvents(context.Background(), nil)
			return v != nil, err
		})

		// Attempting to reindex the same events should gracefully succeed.
		require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))
	})

	t.Run("IndexTxEvents", func(t *testing.T) {
		indexer := &EventSink{chainID: chainID, producer: testProducer()}

		txResult := util.TxResultWithEvents([]abci.Event{
			makeIndexedEvent("account.number", "1"),
			makeIndexedEvent("account.owner", "Ivan"),
			makeIndexedEvent("account.owner", "Yulieta"),

			{Type: "", Attributes: []abci.EventAttribute{{Key: "not_allowed", Value: "Vlad", Index: true}}},
		})
		require.NoError(t, indexer.IndexTxEvents([]*abci.TxResult{txResult}))

		verifyTx(t, txResult)

		util.VerifyNotImplemented(t, "getTxByHash", string(indexer.Type()), func() (bool, error) {
			txr, err := indexer.GetTxByHash(types.Tx(txResult.Tx).Hash())
			return txr != nil, err
		})
		util.VerifyNotImplemented(t, "tx search", string(indexer.Type()), func() (bool, error) {
			txr, err := indexer.SearchTxEvents(context.Background(), nil)
			return txr != nil, err
		})

		// try to insert the duplicate tx events.
		err := indexer.IndexTxEvents([]*abci.TxResult{txResult})
		require.NoError(t, err)
	})
}

func verifyBlock(t *testing.T, height int64) {
	consumer, err := sarama.NewConsumer([]string{"localhost:9093"}, nil)
	if err != nil {
		t.Errorf("No able to create the consumer: %v", err)
	}
	defer consumer.Close()

	topicHeight := fmt.Sprintf("%s.block", chainID)
	partitionConsumer, err := consumer.ConsumePartition(topicHeight, 0, 0)
	if err != nil {
		t.Errorf("Not able to create the partitionConsumer-%s: %v", topicHeight, err)
	}

	msg := <-partitionConsumer.Messages()
	partitionConsumer.Close()
	// Check that the blocks table contains an entry for this height.
	assert.Equal(t, []byte(fmt.Sprint(height)), msg.Value)

	// Verify the first presence of begin_block
	topicBeginEvent := fmt.Sprintf("%s.begin_event", chainID)
	partitionConsumer, err = consumer.ConsumePartition(topicBeginEvent, 0, 0)
	if err != nil {
		t.Errorf("Not able to create the partitionConsumer-%s: %v", topicBeginEvent, err)
	}

	msg = <-partitionConsumer.Messages()
	partitionConsumer.Close()
	key, _ := orderedcode.Append(nil, "proposer", height)
	assert.Equal(t, key, msg.Key)
	assert.Equal(t, []byte("FCAA001"), msg.Value)

	// Verify the second presence of begin_block
	topicThingy := fmt.Sprintf("%s.thingy", chainID)
	partitionConsumer, err = consumer.ConsumePartition(topicThingy, 0, 0)
	if err != nil {
		t.Errorf("Not able to create the partitionConsumer-%s: %v", topicThingy, err)
	}

	// There are two messages for topic thingy, so we need to read twice
	msg = <-partitionConsumer.Messages()
	key, _ = orderedcode.Append(nil, "whatzit", height)
	assert.Equal(t, key, msg.Key)
	assert.Equal(t, []byte("O.O"), msg.Value)

	msg = <-partitionConsumer.Messages()
	partitionConsumer.Close()
	assert.Equal(t, key, msg.Key)
	assert.Equal(t, []byte("-.O"), msg.Value)

	// Verify the first presence of end_block
	topicEndEvent := fmt.Sprintf("%s.end_event", chainID)
	partitionConsumer, err = consumer.ConsumePartition(topicEndEvent, 0, 0)
	if err != nil {
		t.Errorf("Not able to create the partitionConsumer-%s: %v", partitionConsumer, err)
	}

	msg = <-partitionConsumer.Messages()
	partitionConsumer.Close()
	key, _ = orderedcode.Append(nil, "foo", height)
	assert.Equal(t, key, msg.Key)
	assert.Equal(t, []byte("100"), msg.Value)
}

func verifyTx(t *testing.T, txResult *abci.TxResult) {

	consumer, err := sarama.NewConsumer([]string{"localhost:9093"}, nil)
	if err != nil {
		t.Errorf("Not able to create the consumer: %v", err)
	}
	defer consumer.Close()

	// Check that the message has indexed the tx height.
	topicTx := fmt.Sprintf("%s.tx", chainID)
	partitionConsumer, err := consumer.ConsumePartition(topicTx, 0, 0)
	if err != nil {
		t.Errorf("Not able to create the partitionConsumer-%s: %v", topicTx, err)
	}

	select {
	case msg := <-partitionConsumer.Messages():
		key, _ := orderedcode.Append(nil, "height")
		assert.Equal(t, key, msg.Key)
		assert.Equal(t, []byte(fmt.Sprint(txResult.Height)), msg.Value)
	case err := <-partitionConsumer.Errors():
		t.Errorf("Not able to consume the %s message: %v", topicTx, err)
	}

	hash := fmt.Sprintf("%X", types.Tx(txResult.Tx).Hash())

	// Check that the message has indexed the tx hash.
	select {
	case msg := <-partitionConsumer.Messages():
		key, _ := orderedcode.Append(nil, "hash", txResult.Height)
		assert.Equal(t, key, msg.Key)
		assert.Equal(t, []byte(hash), msg.Value)
	case err := <-partitionConsumer.Errors():
		t.Errorf("Not able to consume the %s message: %v", topicTx, err)
	}

	txRaw, err := proto.Marshal(txResult)
	if err != nil {
		t.Errorf("marshaling tx_result: %v", err)
	}

	// Check that the message has indexed the tx raw data.
	select {
	case msg := <-partitionConsumer.Messages():
		key, _ := orderedcode.Append(nil, "hash", hash)
		assert.Equal(t, key, msg.Key)
		assert.Equal(t, txRaw, msg.Value)
	case err := <-partitionConsumer.Errors():
		t.Errorf("Not able to consume the %s message: %v", topicTx, err)
	}
	partitionConsumer.Close()

	// Check that the message has indexed the tx account.
	topicAccount := fmt.Sprintf("%s.account", chainID)
	partitionConsumer, err = consumer.ConsumePartition(topicAccount, 0, 0)
	if err != nil {
		t.Errorf("Not able to create the partitionConsumer-%s: %v", topicAccount, err)
	}

	for _, e := range txResult.Result.Events {
		// The empty event type will not be indexed.
		if len(e.GetType()) == 0 {
			continue
		}

		select {
		case msg := <-partitionConsumer.Messages():
			key, _ := orderedcode.Append(nil, e.Attributes[0].Key, txResult.Height, hash)
			assert.Equal(t, key, msg.Key)
			assert.Equal(t, []byte(e.Attributes[0].Value), msg.Value)
		case err := <-partitionConsumer.Errors():
			t.Errorf("Not able to consume the %s message: %v", topicAccount, err)
		}
	}
	partitionConsumer.Close()
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
