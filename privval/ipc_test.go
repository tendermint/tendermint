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

var (
	testConnDeadline    = 100 * time.Millisecond
	testConnDeadline2o3 = 66 * time.Millisecond // 2/3 of the other one

	testHeartbeatTimeout    = 10 * time.Millisecond
	testHeartbeatTimeout3o2 = 6 * time.Millisecond // 3/2 of the other one
)

func TestIPCPVVoteSimple(t *testing.T) {
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

	// Wait less than the full testConnDeadline
	time.Sleep(testConnDeadline2o3)

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)

	// This would exceed the deadline if it was not extended by the previous message.
	time.Sleep(testConnDeadline2o3)

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

	// Wait longer than the testConnDeadline.
	// Connection only stays alive because of Ping.
	time.Sleep(testConnDeadline * 2)

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)
}

func TestRetryIPCConnToRemoteSigner(t *testing.T) {
	addr, err := testUnixAddr()
	require.NoError(t, err)

	var (
		logger  = log.TestingLogger()
		chainID = cmn.RandStr(12)
		readyc  = make(chan struct{})

		rs = NewIPCRemoteSigner(
			logger,
			chainID,
			addr,
			types.NewMockPV(),
		)
		sc = NewIPCVal(
			logger,
			addr,
		)
	)

	// Ping every:
	IPCValHeartbeat(testHeartbeatTimeout)(sc)

	IPCValConnTimeout(testConnDeadline)(sc)
	IPCRemoteSignerConnDeadline(testConnDeadline)(rs)
	IPCRemoteSignerConnRetries(10)(rs)

	testStartIPCRemoteSigner(t, readyc, rs)
	<-readyc

	require.NoError(t, sc.Start())
	assert.True(t, sc.IsRunning())
	defer sc.Stop()

	time.Sleep(testConnDeadline / 2)

	rs.Stop()
	rs2 := NewIPCRemoteSigner(
		logger,
		chainID,
		addr,
		types.NewMockPV(),
	)
	// let some pings pass
	time.Sleep(testHeartbeatTimeout3o2)
	require.NoError(t, rs2.Start())
	assert.True(t, rs2.IsRunning())
	defer rs2.Stop()

	// give the client some time to re-establish the conn to the remote signer
	// should see sth like this in the logs:
	//
	// E[10016-01-10|17:12:46.128] Ping                                         err="remote signer timed out"
	// I[10016-01-10|17:16:42.447] Re-created connection to remote signer       impl=IPCVal
	time.Sleep(testConnDeadline * 2)
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

	IPCValConnTimeout(testConnDeadline)(sc)
	IPCValHeartbeat(testHeartbeatTimeout)(sc)

	IPCRemoteSignerConnDeadline(testConnDeadline)(rs)

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
