package privval

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/types"
)

var (
	testAcceptDeadline = defaultAcceptDeadlineSeconds * time.Second

	testConnDeadline    = 100 * time.Millisecond
	testConnDeadline2o3 = 66 * time.Millisecond // 2/3 of the other one

	testHeartbeatTimeout    = 10 * time.Millisecond
	testHeartbeatTimeout3o2 = 6 * time.Millisecond // 3/2 of the other one
)

type socketTestCase struct {
	addr   string
	dialer Dialer
}

func socketTestCases(t *testing.T) []socketTestCase {
	tcpAddr := fmt.Sprintf("tcp://%s", testFreeTCPAddr(t))
	unixFilePath, err := testUnixAddr()
	require.NoError(t, err)
	unixAddr := fmt.Sprintf("unix://%s", unixFilePath)
	return []socketTestCase{
		{
			addr:   tcpAddr,
			dialer: DialTCPFn(tcpAddr, testConnDeadline, ed25519.GenPrivKey()),
		},
		{
			addr:   unixAddr,
			dialer: DialUnixFn(unixFilePath),
		},
	}
}

func TestSocketPVAddress(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		// Execute the test within a closure to ensure the deferred statements
		// are called between each for loop iteration, for isolated test cases.
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV(), tc.addr, tc.dialer)
			)
			defer sc.Stop()
			defer rs.Stop()

			serverAddr := rs.privVal.GetPubKey().Address()
			clientAddr := sc.GetPubKey().Address()

			assert.Equal(t, serverAddr, clientAddr)
		}()
	}
}

func TestSocketPVPubKey(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV(), tc.addr, tc.dialer)
			)
			defer sc.Stop()
			defer rs.Stop()

			clientKey := sc.GetPubKey()

			privvalPubKey := rs.privVal.GetPubKey()

			assert.Equal(t, privvalPubKey, clientKey)
		}()
	}
}

func TestSocketPVProposal(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV(), tc.addr, tc.dialer)

				ts             = time.Now()
				privProposal   = &types.Proposal{Timestamp: ts}
				clientProposal = &types.Proposal{Timestamp: ts}
			)
			defer sc.Stop()
			defer rs.Stop()

			require.NoError(t, rs.privVal.SignProposal(chainID, privProposal))
			require.NoError(t, sc.SignProposal(chainID, clientProposal))
			assert.Equal(t, privProposal.Signature, clientProposal.Signature)
		}()
	}
}

func TestSocketPVVote(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV(), tc.addr, tc.dialer)

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
		}()
	}
}

func TestSocketPVVoteResetDeadline(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV(), tc.addr, tc.dialer)

				ts    = time.Now()
				vType = types.PrecommitType
				want  = &types.Vote{Timestamp: ts, Type: vType}
				have  = &types.Vote{Timestamp: ts, Type: vType}
			)
			defer sc.Stop()
			defer rs.Stop()

			time.Sleep(testConnDeadline2o3)

			require.NoError(t, rs.privVal.SignVote(chainID, want))
			require.NoError(t, sc.SignVote(chainID, have))
			assert.Equal(t, want.Signature, have.Signature)

			// This would exceed the deadline if it was not extended by the previous message
			time.Sleep(testConnDeadline2o3)

			require.NoError(t, rs.privVal.SignVote(chainID, want))
			require.NoError(t, sc.SignVote(chainID, have))
			assert.Equal(t, want.Signature, have.Signature)
		}()
	}
}

func TestSocketPVVoteKeepalive(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV(), tc.addr, tc.dialer)

				ts    = time.Now()
				vType = types.PrecommitType
				want  = &types.Vote{Timestamp: ts, Type: vType}
				have  = &types.Vote{Timestamp: ts, Type: vType}
			)
			defer sc.Stop()
			defer rs.Stop()

			time.Sleep(testConnDeadline * 2)

			require.NoError(t, rs.privVal.SignVote(chainID, want))
			require.NoError(t, sc.SignVote(chainID, have))
			assert.Equal(t, want.Signature, have.Signature)
		}()
	}
}

func TestSocketPVDeadline(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				listenc         = make(chan struct{})
				thisConnTimeout = 100 * time.Millisecond
				sc              = newSocketVal(log.TestingLogger(), tc.addr, thisConnTimeout)
			)

			go func(sc *SocketVal) {
				defer close(listenc)

				// Note: the TCP connection times out at the accept() phase,
				// whereas the Unix domain sockets connection times out while
				// attempting to fetch the remote signer's public key.
				assert.True(t, IsConnTimeout(sc.Start()))

				assert.False(t, sc.IsRunning())
			}(sc)

			for {
				_, err := cmn.Connect(tc.addr)
				if err == nil {
					break
				}
			}

			<-listenc
		}()
	}
}

func TestRemoteSignVoteErrors(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewErroringMockPV(), tc.addr, tc.dialer)

				ts    = time.Now()
				vType = types.PrecommitType
				vote  = &types.Vote{Timestamp: ts, Type: vType}
			)
			defer sc.Stop()
			defer rs.Stop()

			err := sc.SignVote("", vote)
			require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

			err = rs.privVal.SignVote(chainID, vote)
			require.Error(t, err)
			err = sc.SignVote(chainID, vote)
			require.Error(t, err)
		}()
	}
}

func TestRemoteSignProposalErrors(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				chainID = cmn.RandStr(12)
				sc, rs  = testSetupSocketPair(t, chainID, types.NewErroringMockPV(), tc.addr, tc.dialer)

				ts       = time.Now()
				proposal = &types.Proposal{Timestamp: ts}
			)
			defer sc.Stop()
			defer rs.Stop()

			err := sc.SignProposal("", proposal)
			require.Equal(t, err.(*RemoteSignerError).Description, types.ErroringMockPVErr.Error())

			err = rs.privVal.SignProposal(chainID, proposal)
			require.Error(t, err)

			err = sc.SignProposal(chainID, proposal)
			require.Error(t, err)
		}()
	}
}

func TestErrUnexpectedResponse(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				logger  = log.TestingLogger()
				chainID = cmn.RandStr(12)
				readyc  = make(chan struct{})
				errc    = make(chan error, 1)

				rs = NewRemoteSigner(
					logger,
					chainID,
					types.NewMockPV(),
					tc.dialer,
				)
				sc = newSocketVal(logger, tc.addr, testConnDeadline)
			)

			testStartSocketPV(t, readyc, sc)
			defer sc.Stop()
			RemoteSignerConnDeadline(time.Millisecond)(rs)
			RemoteSignerConnRetries(100)(rs)
			// we do not want to Start() the remote signer here and instead use the connection to
			// reply with intentionally wrong replies below:
			rsConn, err := rs.connect()
			defer rsConn.Close()
			require.NoError(t, err)
			require.NotNil(t, rsConn)
			// send over public key to get the remote signer running:
			go testReadWriteResponse(t, &PubKeyResponse{}, rsConn)
			<-readyc

			// Proposal:
			go func(errc chan error) {
				errc <- sc.SignProposal(chainID, &types.Proposal{})
			}(errc)
			// read request and write wrong response:
			go testReadWriteResponse(t, &SignedVoteResponse{}, rsConn)
			err = <-errc
			require.Error(t, err)
			require.Equal(t, err, ErrUnexpectedResponse)

			// Vote:
			go func(errc chan error) {
				errc <- sc.SignVote(chainID, &types.Vote{})
			}(errc)
			// read request and write wrong response:
			go testReadWriteResponse(t, &SignedProposalResponse{}, rsConn)
			err = <-errc
			require.Error(t, err)
			require.Equal(t, err, ErrUnexpectedResponse)
		}()
	}
}

func TestRetryConnToRemoteSigner(t *testing.T) {
	for _, tc := range socketTestCases(t) {
		func() {
			var (
				logger  = log.TestingLogger()
				chainID = cmn.RandStr(12)
				readyc  = make(chan struct{})

				rs = NewRemoteSigner(
					logger,
					chainID,
					types.NewMockPV(),
					tc.dialer,
				)
				thisConnTimeout = testConnDeadline
				sc              = newSocketVal(logger, tc.addr, thisConnTimeout)
			)
			// Ping every:
			SocketValHeartbeat(testHeartbeatTimeout)(sc)

			RemoteSignerConnDeadline(testConnDeadline)(rs)
			RemoteSignerConnRetries(10)(rs)

			testStartSocketPV(t, readyc, sc)
			defer sc.Stop()
			require.NoError(t, rs.Start())
			assert.True(t, rs.IsRunning())

			<-readyc
			time.Sleep(testHeartbeatTimeout * 2)

			rs.Stop()
			rs2 := NewRemoteSigner(
				logger,
				chainID,
				types.NewMockPV(),
				tc.dialer,
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
			// I[10016-01-10|17:16:42.447] Re-created connection to remote signer       impl=SocketVal
			time.Sleep(testConnDeadline * 2)
		}()
	}
}

func newSocketVal(logger log.Logger, addr string, connDeadline time.Duration) *SocketVal {
	proto, address := cmn.ProtocolAndAddress(addr)
	ln, err := net.Listen(proto, address)
	logger.Info("Listening at", "proto", proto, "address", address)
	if err != nil {
		panic(err)
	}
	var svln net.Listener
	if proto == "unix" {
		unixLn := NewUnixListener(ln)
		UnixListenerAcceptDeadline(testAcceptDeadline)(unixLn)
		UnixListenerConnDeadline(connDeadline)(unixLn)
		svln = unixLn
	} else {
		tcpLn := NewTCPListener(ln, ed25519.GenPrivKey())
		TCPListenerAcceptDeadline(testAcceptDeadline)(tcpLn)
		TCPListenerConnDeadline(connDeadline)(tcpLn)
		svln = tcpLn
	}
	return NewSocketVal(logger, svln)
}

func testSetupSocketPair(
	t *testing.T,
	chainID string,
	privValidator types.PrivValidator,
	addr string,
	dialer Dialer,
) (*SocketVal, *RemoteSigner) {
	var (
		logger  = log.TestingLogger()
		privVal = privValidator
		readyc  = make(chan struct{})
		rs      = NewRemoteSigner(
			logger,
			chainID,
			privVal,
			dialer,
		)

		thisConnTimeout = testConnDeadline
		sc              = newSocketVal(logger, addr, thisConnTimeout)
	)

	SocketValHeartbeat(testHeartbeatTimeout)(sc)
	RemoteSignerConnDeadline(testConnDeadline)(rs)
	RemoteSignerConnRetries(1e6)(rs)

	testStartSocketPV(t, readyc, sc)

	require.NoError(t, rs.Start())
	assert.True(t, rs.IsRunning())

	<-readyc

	return sc, rs
}

func testReadWriteResponse(t *testing.T, resp RemoteSignerMsg, rsConn net.Conn) {
	_, err := readMsg(rsConn)
	require.NoError(t, err)

	err = writeMsg(rsConn, resp)
	require.NoError(t, err)
}

func testStartSocketPV(t *testing.T, readyc chan struct{}, sc *SocketVal) {
	go func(sc *SocketVal) {
		require.NoError(t, sc.Start())
		assert.True(t, sc.IsRunning())

		readyc <- struct{}{}
	}(sc)
}

// testFreeTCPAddr claims a free port so we don't block on listener being ready.
func testFreeTCPAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}
