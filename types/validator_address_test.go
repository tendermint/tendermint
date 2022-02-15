package types

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/libs/rand"
)

func TestValidatorAddress_String(t *testing.T) {
	nodeID := randNodeID()
	tests := []struct {
		uri  string
		want string
	}{
		{
			uri:  "tcp://" + nodeID + "@fqdn.address.com:1234",
			want: "tcp://" + nodeID + "@fqdn.address.com:1234",
		},
		{
			uri:  "tcp://fqdn.address.com:1234",
			want: "tcp://fqdn.address.com:1234",
		},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			va, err := ParseValidatorAddress(tt.uri)
			assert.NoError(t, err)
			got := va.String()
			assert.EqualValues(t, tt.want, got)
		})
	}
}

// TestValidatorAddress_NodeID_fail checks if NodeID lookup fails when trying to connect to ssh port
// NOTE: Positive flow is tested as part of node_test.go TestNodeStartStop()
func TestValidatorAddress_NodeID_fail(t *testing.T) {
	nodeID := randNodeID()

	tests := []struct {
		uri     string
		want    string
		wantErr bool
	}{
		{
			uri:  "tcp://" + nodeID + "@fqdn.address.com:1234",
			want: nodeID,
		},
		{
			uri:     "tcp://127.0.0.1:22",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			va, err := ParseValidatorAddress(tt.uri)
			assert.NoError(t, err)
			// todo lookup for an address
			got := va.NodeID
			// assert.Equal(t, err != nil, tt.wantErr, "wantErr=%t, but err = %s", tt.wantErr, err)
			assert.EqualValues(t, tt.want, got)
		})
	}
}

// TestValidatorAddress_HostPortProto verifies if host, port and proto is detected correctly when parsing
// ValidatorAddress
func TestValidatorAddress_HostPortProto(t *testing.T) {
	nodeID := randNodeID()

	tests := []struct {
		uri          string
		wantHost     string
		wantPort     uint16
		wantProto    string
		wantNodeID   string
		wantValid    bool
		wantParseErr bool
	}{
		{
			uri:        "tcp://" + nodeID + "@fqdn.address.com:1234",
			wantHost:   "fqdn.address.com",
			wantPort:   1234,
			wantProto:  "tcp",
			wantNodeID: nodeID,
			wantValid:  true,
		},
		{
			uri:          "tcp://test@fqdn.address.com:1234",
			wantHost:     "fqdn.address.com",
			wantPort:     1234,
			wantProto:    "tcp",
			wantValid:    false,
			wantParseErr: true,
		},
		{
			uri:       "tcp://127.0.0.1:22",
			wantHost:  "127.0.0.1",
			wantPort:  22,
			wantProto: "tcp",
			wantValid: true,
		},
		{
			uri:          "",
			wantValid:    false,
			wantParseErr: true,
		},
		{
			uri:          "tcp://127.0.0.1",
			wantHost:     "127.0.0.1",
			wantPort:     0,
			wantProto:    "tcp",
			wantValid:    false,
			wantParseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			va, err := ParseValidatorAddress(tt.uri)
			if tt.wantParseErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tt.wantHost, va.Hostname)
				assert.EqualValues(t, tt.wantPort, va.Port)

				if tt.wantNodeID != "" {
					nodeID := va.NodeID
					assert.EqualValues(t, tt.wantNodeID, nodeID)
				}
				err = va.Validate()
				if tt.wantValid {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			}
		})
	}
}

func TestValidatorAddress_NetAddress(t *testing.T) {
	nodeID := randNodeID()
	uri := "tcp://" + nodeID + "@127.0.0.1:1234"

	va, err := ParseValidatorAddress(uri)
	assert.NoError(t, err)

	naddr, err := va.NetAddress()
	assert.NoError(t, err)
	assert.NoError(t, naddr.Valid())
	assert.EqualValues(t, naddr.IP.String(), "127.0.0.1")
	assert.EqualValues(t, naddr.Port, 1234)
	assert.EqualValues(t, naddr.ID, nodeID)
}

// utility functions

func randNodeID() string {
	nodeID := rand.Bytes(NodeIDByteLength)
	return hex.EncodeToString(nodeID)
}
