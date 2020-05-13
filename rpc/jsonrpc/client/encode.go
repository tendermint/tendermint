package client

import (
	"fmt"
	"net/url"
	"reflect"

	amino "github.com/tendermint/go-amino"
)

func argsToURLValues(cdc *amino.Codec, args map[string]interface{}) (url.Values, error) {
	values := make(url.Values)
	if len(args) == 0 {
		return values, nil
	}

	err := argsToJSON(cdc, args)
	if err != nil {
		return nil, err
	}

	for key, val := range args {
		values.Set(key, val.(string))
	}

	return values, nil
}

func argsToJSON(cdc *amino.Codec, args map[string]interface{}) error {
	for k, v := range args {
		rt := reflect.TypeOf(v)
		isByteSlice := rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.Uint8
		if isByteSlice {
			bytes := reflect.ValueOf(v).Bytes()
			args[k] = fmt.Sprintf("0x%X", bytes)
			continue
		}

		data, err := cdc.MarshalJSON(v)
		if err != nil {
			return err
		}
		args[k] = string(data)
	}
	return nil
}
