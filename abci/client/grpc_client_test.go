package abcicli_test

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"

	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
)

func TestGRPC(t *testing.T) {
	app := types.NewBaseApplication()
	numCheckTxs := 2000
	socketFile := fmt.Sprintf("/tmp/test-%08x.sock", rand.Int31n(1<<30))
	defer os.Remove(socketFile)
	socket := fmt.Sprintf("unix://%v", socketFile)

	// Start the listener
	server := abciserver.NewGRPCServer(socket, app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	err := server.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
	})

	// Connect to the socket
	//nolint:staticcheck // SA1019 Existing use of deprecated but supported dial option.
	conn, err := grpc.Dial(socket, grpc.WithInsecure(), grpc.WithContextDialer(dialerFunc))
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})

	client := types.NewABCIClient(conn)

	// Write requests
	for counter := 0; counter < numCheckTxs; counter++ {
		// Send request
		response, err := client.CheckTx(context.Background(), &types.RequestCheckTx{Tx: []byte("test")})
		require.NoError(t, err)
		counter++
		if response.Code != 0 {
			t.Error("CheckTx failed with ret_code", response.Code)
		}
		if counter > numCheckTxs {
			t.Fatal("Too many CheckTx responses")
		}
		t.Log("response", counter)
		if counter == numCheckTxs {
			go func() {
				time.Sleep(time.Second * 1) // Wait for a bit to allow counter overflow
			}()
		}

	}
}

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return tmnet.Connect(addr)
}
