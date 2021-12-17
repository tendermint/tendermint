package client

import (
	"fmt"
	"reflect"

	tmjson "github.com/tendermint/tendermint/libs/json"
)

func argsToJSON(args map[string]interface{}) error {
	for k, v := range args {
		rt := reflect.TypeOf(v)
		isByteSlice := rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.Uint8
		if isByteSlice {
			bytes := reflect.ValueOf(v).Bytes()
			args[k] = fmt.Sprintf("0x%X", bytes)
			continue
		}

		data, err := tmjson.Marshal(v)
		if err != nil {
			return err
		}
		args[k] = string(data)
	}
	return nil
}
