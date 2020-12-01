package grpc_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	privproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

type signerTestCase struct {
	chainID      string
	mockPV       types.PrivValidator
	signerClient *tmgrpc.SignerClient
	signerServer *tmgrpc.SignerServer
}

func getSignerTestCases(t *testing.T) []signerTestCase {
	testCases := make([]signerTestCase, 0)

	unixFilePath, err := testUnixAddr()
	require.NoError(t, err)

	addresses := []string{privval.GetFreeLocalhostAddrPort(), fmt.Sprintf("unix://%s", unixFilePath)}

	// Get test cases for each possible dialer (DialTCP / DialUnix / etc)
	for _, dtc := range addresses {
		chainID := tmrand.Str(12)
		mockPV := types.NewMockPV()

		// get a pair of signer listener, signer dialer endpoints
		sl, sd := getMockEndpoints(t, dtc.addr, dtc.dialer)
		sc, err := NewSignerClient(sl, chainID)
		require.NoError(t, err)
		ss := NewSignerServer(sd, chainID, mockPV)

		err = ss.Start()
		require.NoError(t, err)

		tc := signerTestCase{
			chainID:      chainID,
			mockPV:       mockPV,
			signerClient: sc,
			signerServer: ss,
		}

		testCases = append(testCases, tc)
	}

	return testCases
}

func TestGetPubKey(t *testing.T) {
	mockPV := types.NewMockPV()

	s := tmgrpc.SignerServer{
		ChainID: "123",
		PrivVal: mockPV,
	}

	req := &privproto.PubKeyRequest{ChainId: s.ChainID}
	resp, err := s.GetPubKey(context.Background(), req)
	if err != nil {
		t.Errorf("got unexpected error")
	}

	if resp.PubKey.GetEd25519() == nil {
		t.Errorf("wanted ed25519 key, got %s", resp.PubKey.String())
	}
}

// testUnixAddr will attempt to obtain a platform-independent temporary file
// name for a Unix socket
func testUnixAddr() (string, error) {
	f, err := ioutil.TempFile("", "tendermint-privval-test-*")
	if err != nil {
		return "", err
	}
	addr := f.Name()
	f.Close()
	os.Remove(addr)
	return addr, nil
}
