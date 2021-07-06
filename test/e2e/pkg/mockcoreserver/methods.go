package mockcoreserver

import (
	"encoding/json"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"
)

// Endless ...
const Endless = -1

// MethodFunc ...
type MethodFunc func(srv *JRPCServer)

// WithQuorumInfoMethod ...
func WithQuorumInfoMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.QuorumCmd{}
		err := unmarshalCmd(req, &cmd.SubCmd, &cmd.LLMQType, &cmd.QuorumHash, &cmd.IncludeSkShare)
		if err != nil {
			return nil, err
		}
		return cs.QuorumInfo(cmd), nil
	})
	return func(srv *JRPCServer) {
		srv.
			On("quorum info").
			Expect(And(Debug())).
			Times(times).
			Respond(call, JsonContentType())
	}
}

// WithQuorumSignMethod ...
func WithQuorumSignMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.QuorumCmd{}
		err := unmarshalCmd(req, &cmd.SubCmd, &cmd.LLMQType, &cmd.RequestID, &cmd.MessageHash, &cmd.QuorumHash, &cmd.Submit)
		if err != nil {
			return nil, err
		}
		return cs.QuorumSign(cmd), nil
	})
	return func(srv *JRPCServer) {
		srv.
			On("quorum sign").
			Expect(And(Debug())).
			Times(times).
			Respond(call, JsonContentType())
	}
}

// WithQuorumVerifyMethod ...
func WithQuorumVerifyMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.QuorumCmd{}
		fmt.Printf("request is %v\n", req)
		err := unmarshalCmd(req, &cmd.SubCmd, &cmd.LLMQType, &cmd.RequestID, &cmd.MessageHash, &cmd.QuorumHash, &cmd.Signature)
		fmt.Printf("cmd is %v\n", cmd)
		if err != nil {
			return nil, err
		}
		return cs.QuorumVerify(cmd), nil
	})
	return func(srv *JRPCServer) {
		srv.
			On("quorum verify").
			Expect(And(Debug())).
			Times(times).
			Respond(call, JsonContentType())
	}
}

// WithMasternodeMethod ...
func WithMasternodeMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.MasternodeCmd{}
		err := unmarshalCmd(req, &cmd.SubCmd)
		if err != nil {
			return nil, err
		}
		return cs.MasternodeStatus(cmd), nil
	})
	return func(srv *JRPCServer) {
		srv.
			On("masternode status").
			Expect(And(Debug())).
			Times(times).
			Respond(call, JsonContentType())
	}
}

// WithGetNetworkInfoMethod ...
func WithGetNetworkInfoMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.GetNetworkInfoCmd{}
		return cs.GetNetworkInfo(cmd), nil
	})
	return func(srv *JRPCServer) {
		srv.
			On("getnetworkinfo").
			Expect(And(Debug())).
			Times(times).
			Respond(call, JsonContentType())
	}
}

// WithPingMethod ...
func WithPingMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.PingCmd{}
		return cs.Ping(cmd), nil
	})
	return func(srv *JRPCServer) {
		srv.
			On("ping").
			Expect(And(Debug())).
			Times(times).
			Respond(call, JsonContentType())
	}
}

// WithGetPeerInfoMethod ...
func WithGetPeerInfoMethod(times int) MethodFunc {
	result := []btcjson.GetPeerInfoResult{{}}
	return func(srv *JRPCServer) {
		srv.
			On("getpeerinfo").
			Expect(And(JRPCParamsEmpty())).
			Times(times).
			Respond(JRPCResult(result), JsonContentType())
	}
}

// WithMethods ...
func WithMethods(srv *JRPCServer, methods ...func(srv *JRPCServer)) *JRPCServer {
	for _, fn := range methods {
		fn(srv)
	}
	return srv
}

func unmarshalCmd(req btcjson.Request, fields ...interface{}) error {
	for i, field := range fields {
		err := json.Unmarshal(req.Params[i], field)
		if err != nil {
			return err
		}
	}
	return nil
}
