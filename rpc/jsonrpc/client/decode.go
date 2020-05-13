package client

import (
	"encoding/json"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	types "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func unmarshalResponseBytes(
	cdc *amino.Codec,
	responseBytes []byte,
	expectedID types.JSONRPCIntID,
	result interface{},
) (interface{}, error) {

	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	response := &types.RPCResponse{}
	if err := json.Unmarshal(responseBytes, response); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling")
	}

	if response.Error != nil {
		return nil, response.Error
	}

	if err := validateAndVerifyID(response, expectedID); err != nil {
		return nil, errors.Wrap(err, "wrong ID")
	}

	// Unmarshal the RawMessage into the result.
	if err := cdc.UnmarshalJSON(response.Result, result); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling result")
	}

	return result, nil
}

func unmarshalResponseBytesArray(
	cdc *amino.Codec,
	responseBytes []byte,
	expectedIDs []types.JSONRPCIntID,
	results []interface{},
) ([]interface{}, error) {

	var (
		responses []types.RPCResponse
	)

	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling")
	}

	// No response error checking here as there may be a mixture of successful
	// and unsuccessful responses.

	if len(results) != len(responses) {
		return nil, errors.Errorf(
			"expected %d result objects into which to inject responses, but got %d",
			len(responses),
			len(results),
		)
	}

	// Intersect IDs from responses with expectedIDs.
	ids := make([]types.JSONRPCIntID, len(responses))
	var ok bool
	for i, resp := range responses {
		ids[i], ok = resp.ID.(types.JSONRPCIntID)
		if !ok {
			return nil, errors.Errorf("expected JSONRPCIntID, got %T", resp.ID)
		}
	}
	if err := validateResponseIDs(ids, expectedIDs); err != nil {
		return nil, errors.Wrap(err, "wrong IDs")
	}

	for i := 0; i < len(responses); i++ {
		if err := cdc.UnmarshalJSON(responses[i].Result, results[i]); err != nil {
			return nil, errors.Wrapf(err, "error unmarshalling #%d result", i)
		}
	}

	return results, nil
}

func validateResponseIDs(ids, expectedIDs []types.JSONRPCIntID) error {
	m := make(map[types.JSONRPCIntID]bool, len(expectedIDs))
	for _, expectedID := range expectedIDs {
		m[expectedID] = true
	}

	for i, id := range ids {
		if m[id] {
			delete(m, id)
		} else {
			return errors.Errorf("unsolicited ID #%d: %v", i, id)
		}
	}

	return nil
}

// From the JSON-RPC 2.0 spec:
// id: It MUST be the same as the value of the id member in the Request Object.
func validateAndVerifyID(res *types.RPCResponse, expectedID types.JSONRPCIntID) error {
	if err := validateResponseID(res.ID); err != nil {
		return err
	}
	if expectedID != res.ID.(types.JSONRPCIntID) { // validateResponseID ensured res.ID has the right type
		return errors.Errorf("response ID (%d) does not match request ID (%d)", res.ID, expectedID)
	}
	return nil
}

func validateResponseID(id interface{}) error {
	if id == nil {
		return errors.New("no ID")
	}
	_, ok := id.(types.JSONRPCIntID)
	if !ok {
		return errors.Errorf("expected JSONRPCIntID, but got: %T", id)
	}
	return nil
}
