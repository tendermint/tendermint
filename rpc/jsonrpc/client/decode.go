package client

import (
	"encoding/json"
	"errors"
	"fmt"

	tmjson "github.com/tendermint/tendermint/libs/json"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func unmarshalResponseBytes(
	responseBytes []byte,
	expectedID rpctypes.JSONRPCIntID,
	result interface{},
) (interface{}, error) {

	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	response := &rpctypes.RPCResponse{}
	if err := json.Unmarshal(responseBytes, response); err != nil {
		return nil, fmt.Errorf("error unmarshaling: %w", err)
	}

	if response.Error != nil {
		return nil, response.Error
	}

	if err := validateAndVerifyID(response, expectedID); err != nil {
		return nil, fmt.Errorf("wrong ID: %w", err)
	}

	// Unmarshal the RawMessage into the result.
	if err := tmjson.Unmarshal(response.Result, result); err != nil {
		return nil, fmt.Errorf("error unmarshaling result: %w", err)
	}

	return result, nil
}

func unmarshalResponseBytesArray(
	responseBytes []byte,
	expectedIDs []rpctypes.JSONRPCIntID,
	results []interface{},
) ([]interface{}, error) {

	var (
		responses []rpctypes.RPCResponse
	)

	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		return nil, fmt.Errorf("error unmarshaling: %w", err)
	}

	// No response error checking here as there may be a mixture of successful
	// and unsuccessful responses.

	if len(results) != len(responses) {
		return nil, fmt.Errorf(
			"expected %d result objects into which to inject responses, but got %d",
			len(responses),
			len(results),
		)
	}

	// Intersect IDs from responses with expectedIDs.
	ids := make([]rpctypes.JSONRPCIntID, len(responses))
	var ok bool
	for i, resp := range responses {
		ids[i], ok = resp.ID.(rpctypes.JSONRPCIntID)
		if !ok {
			return nil, fmt.Errorf("expected JSONRPCIntID, got %T", resp.ID)
		}
	}
	if err := validateResponseIDs(ids, expectedIDs); err != nil {
		return nil, fmt.Errorf("wrong IDs: %w", err)
	}

	for i := 0; i < len(responses); i++ {
		if err := tmjson.Unmarshal(responses[i].Result, results[i]); err != nil {
			return nil, fmt.Errorf("error unmarshaling #%d result: %w", i, err)
		}
	}

	return results, nil
}

func validateResponseIDs(ids, expectedIDs []rpctypes.JSONRPCIntID) error {
	m := make(map[rpctypes.JSONRPCIntID]bool, len(expectedIDs))
	for _, expectedID := range expectedIDs {
		m[expectedID] = true
	}

	for i, id := range ids {
		if m[id] {
			delete(m, id)
		} else {
			return fmt.Errorf("unsolicited ID #%d: %v", i, id)
		}
	}

	return nil
}

// From the JSON-RPC 2.0 spec:
// id: It MUST be the same as the value of the id member in the Request Object.
func validateAndVerifyID(res *rpctypes.RPCResponse, expectedID rpctypes.JSONRPCIntID) error {
	if err := validateResponseID(res.ID); err != nil {
		return err
	}
	if expectedID != res.ID.(rpctypes.JSONRPCIntID) { // validateResponseID ensured res.ID has the right type
		return fmt.Errorf("response ID (%d) does not match request ID (%d)", res.ID, expectedID)
	}
	return nil
}

func validateResponseID(id interface{}) error {
	if id == nil {
		return errors.New("no ID")
	}
	_, ok := id.(rpctypes.JSONRPCIntID)
	if !ok {
		return fmt.Errorf("expected JSONRPCIntID, but got: %T", id)
	}
	return nil
}
