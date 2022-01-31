package client

import (
	"encoding/json"
	"fmt"

	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func unmarshalResponseBytes(responseBytes []byte, expectedID string, result interface{}) error {
	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	var response rpctypes.RPCResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	if response.Error != nil {
		return response.Error
	}

	if got := response.ID(); got != expectedID {
		return fmt.Errorf("got response ID %q, wanted %q", got, expectedID)
	}

	// Unmarshal the RawMessage into the result.
	if err := json.Unmarshal(response.Result, result); err != nil {
		return fmt.Errorf("error unmarshaling result: %w", err)
	}
	return nil
}

func unmarshalResponseBytesArray(responseBytes []byte, expectedIDs []string, results []interface{}) error {
	var responses []rpctypes.RPCResponse
	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		return fmt.Errorf("unmarshaling responses: %w", err)
	} else if len(responses) != len(results) {
		return fmt.Errorf("got %d results, wanted %d", len(responses), len(results))
	}

	// Intersect IDs from responses with expectedIDs.
	ids := make([]string, len(responses))
	for i, resp := range responses {
		ids[i] = resp.ID()
	}
	if err := validateResponseIDs(ids, expectedIDs); err != nil {
		return fmt.Errorf("wrong IDs: %w", err)
	}

	for i, resp := range responses {
		if err := json.Unmarshal(resp.Result, results[i]); err != nil {
			return fmt.Errorf("unmarshaling result %d: %w", i, err)
		}
	}
	return nil
}

func validateResponseIDs(ids, expectedIDs []string) error {
	m := make(map[string]struct{}, len(expectedIDs))
	for _, id := range expectedIDs {
		m[id] = struct{}{}
	}

	for i, id := range ids {
		if _, ok := m[id]; !ok {
			return fmt.Errorf("unexpected response ID %d: %q", i, id)
		}
	}
	return nil
}
