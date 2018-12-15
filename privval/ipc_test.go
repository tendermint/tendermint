package privval

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

func TestIPCPVVote(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupIPCSocketPair(t, chainID, types.NewMockPV())

		ts    = time.Now()
		vType = types.PrecommitType
		want  = &types.Vote{Timestamp: ts, Type: vType}
		have  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)
}

func TestIPCPVVoteResetDeadline(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupIPCSocketPair(t, chainID, types.NewMockPV())

		ts    = time.Now()
		vType = types.PrecommitType
		want  = &types.Vote{Timestamp: ts, Type: vType}
		have  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer rs.Stop()

	time.Sleep(3 * time.Millisecond)

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)

	// This would exceed the deadline if it was not extended by the previous message
	time.Sleep(3 * time.Millisecond)

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)
}

func TestIPCPVVoteKeepalive(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupIPCSocketPair(t, chainID, types.NewMockPV())

		ts    = time.Now()
		vType = types.PrecommitType
		want  = &types.Vote{Timestamp: ts, Type: vType}
		have  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer rs.Stop()

	time.Sleep(10 * time.Millisecond)

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)
}

func testSetupIPCSocketPair(
	t *testing.T,
	chainID string,
	privValidator types.PrivValidator,
) (*IPCVal, *IPCRemoteSigner) {
	addr, err := testUnixAddr()
	require.NoError(t, err)

	var (
		logger  = log.TestingLogger()
		privVal = privValidator
		readyc  = make(chan struct{})
		rs      = NewIPCRemoteSigner(
			logger,
			chainID,
			addr,
			privVal,
		)
		sc = NewIPCVal(
			logger,
			addr,
		)
	)

	IPCValConnTimeout(5 * time.Millisecond)(sc)
	IPCValHeartbeat(time.Millisecond)(sc)

	IPCRemoteSignerConnDeadline(time.Millisecond * 5)(rs)

	testStartIPCRemoteSigner(t, readyc, rs)

	<-readyc

	require.NoError(t, sc.Start())
	assert.True(t, sc.IsRunning())

	return sc, rs
}

func testStartIPCRemoteSigner(t *testing.T, readyc chan struct{}, rs *IPCRemoteSigner) {
	go func(rs *IPCRemoteSigner) {
		require.NoError(t, rs.Start())
		assert.True(t, rs.IsRunning())

		readyc <- struct{}{}
	}(rs)
}

func testUnixAddr() (string, error) {
	f, err := ioutil.TempFile("/tmp", "nettest")
	if err != nil {
		return "", err
	}

	addr := f.Name()
	err = f.Close()
	if err != nil {
		return "", err
	}
	err = os.Remove(addr)
	if err != nil {
		return "", err
	}

	return addr, nil
}
