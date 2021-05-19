package mockcoreserver

import (
	"encoding/json"

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
		fields := []interface{}{cmd.SubCmd, cmd.LLMQType, cmd.QuorumHash, cmd.IncludeSkShare}
		for i, field := range fields {
			err := json.Unmarshal(req.Params[i], &field)
			if err != nil {
				return err, nil
			}
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

// WithMasternodeMethod ...
func WithMasternodeMethod(cs CoreServer, times int) MethodFunc {
	call := OnMethod(func(req btcjson.Request) (interface{}, error) {
		cmd := btcjson.MasternodeCmd{}
		fields := []interface{}{cmd.SubCmd}
		for i, field := range fields {
			err := json.Unmarshal(req.Params[i], &field)
			if err != nil {
				return err, nil
			}
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
func WithPingMethod(times int) MethodFunc {
	return func(srv *JRPCServer) {
		srv.
			On("ping").
			Expect(JRPCParamsEmpty()).
			Times(times).
			Respond(JRPCResult(""), JsonContentType())
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
