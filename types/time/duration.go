package time

import (
	"encoding/json"
	"errors"
	"time"
)

// DurationPretty is a wrapper around time.Duration implementing custom
// marshaller/unmarshaller which make it pretty (e.g. "10s", not
// "10000000000").
type DurationPretty struct {
	time.Duration
}

// MarshalJSON implements json.Marshaller.
func (d DurationPretty) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaller.
func (d *DurationPretty) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}
