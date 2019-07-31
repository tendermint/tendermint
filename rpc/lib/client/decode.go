package rpcclient

import (
	"encoding/json"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"

	types "github.com/tendermint/tendermint/rpc/lib/types"
)

func unmarshalResponseBytes(cdc *amino.Codec, responseBytes []byte,
	expectedID types.JSONRPCIntID, result interface{}) (interface{}, error) {

	// Read response. If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	response := &types.RPCResponse{}
	err := json.Unmarshal(responseBytes, response)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling response")
	}

	if response.Error != nil {
		return nil, errors.Wrap(response.Error, "response error")
	}

	if err = assertResponseIDEqual(response, expectedID); err != nil {
		return nil, errors.Wrap(err, "error in response ID")
	}

	// Unmarshal the RawMessage into the result.
	err = cdc.UnmarshalJSON(response.Result, result)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling rpc response result")
	}

	return result, nil
}

func unmarshalResponseBytesArray(cdc *amino.Codec, responseBytes []byte,
	expectedIDs []types.JSONRPCIntID, results []interface{}) ([]interface{}, error) {

	var (
		err       error
		responses []types.RPCResponse
	)
	err = json.Unmarshal(responseBytes, &responses)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling rpc response")
	}

	// No response error checking here as there may be a mixture of successful
	// and unsuccessful responses.

	if len(results) != len(responses) {
		return nil, errors.Errorf("expected %d result objects into which to inject responses, but got %d", len(responses), len(results))
	}

	for i, response := range responses {
		if err := validateResponseID(response.ID); err != nil {
			return nil, errors.Wrap(err, "invalid ID")
		}
		if !responseIDIsExpected(response.ID.(types.JSONRPCIntID), expectedIDs) {
			return nil, errors.Errorf("unsolicited response ID: %v", response.ID)
		}
		if err = cdc.UnmarshalJSON(responses[i].Result, results[i]); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling response result")
		}
	}

	return results, nil
}

func responseIDIsExpected(id types.JSONRPCIntID, expectedIDs []types.JSONRPCIntID) bool {
	for _, expectedID := range expectedIDs {
		if id == expectedID {
			return true
		}
	}
	return false
}

// From the JSON-RPC 2.0 spec:
//  id: It MUST be the same as the value of the id member in the Request Object.
func assertResponseIDEqual(res *types.RPCResponse, expectedID types.JSONRPCIntID) error {
	// URIClient does not have ID in response
	if expectedID == -1 {
		return nil
	}
	if err := validateResponseID(res.ID); err != nil {
		return err
	}
	if expectedID != res.ID {
		return errors.Errorf("response ID (%d) does not match request ID (%d)", res.ID, expectedID)
	}
	return nil
}

func validateResponseID(id interface{}) error {
	if id == nil {
		return errors.New("no ID")
	}
	id, ok := id.(types.JSONRPCIntID)
	if !ok {
		return errors.Errorf("expected int, but got: %T", id)
	}
	return nil
}
