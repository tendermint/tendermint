package jsontypes_test

import (
	"testing"

	"github.com/tendermint/tendermint/internal/jsontypes"
)

type testType struct {
	Field string `json:"field"`
}

func (*testType) TypeTag() string { return "test/TaggedType" }

func TestRoundTrip(t *testing.T) {
	const wantEncoded = `{"type":"test/TaggedType","value":{"field":"hello"}}`

	t.Run("MustRegisterOK", func(t *testing.T) {
		defer func() {
			if x := recover(); x != nil {
				t.Fatalf("Registration panicked: %v", x)
			}
		}()
		jsontypes.MustRegister((*testType)(nil))
	})

	t.Run("MustRegisterFail", func(t *testing.T) {
		defer func() {
			if x := recover(); x != nil {
				t.Logf("Got expected panic: %v", x)
			}
		}()
		jsontypes.MustRegister((*testType)(nil))
		t.Fatal("Registration should not have succeeded")
	})

	t.Run("MarshalNil", func(t *testing.T) {
		bits, err := jsontypes.Marshal(nil)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if got := string(bits); got != "null" {
			t.Errorf("Marshal nil: got %#q, want null", got)
		}
	})

	t.Run("RoundTrip", func(t *testing.T) {
		obj := testType{Field: "hello"}
		bits, err := jsontypes.Marshal(&obj)
		if err != nil {
			t.Fatalf("Marshal %T failed: %v", obj, err)
		}
		if got := string(bits); got != wantEncoded {
			t.Errorf("Marshal %T: got %#q, want %#q", obj, got, wantEncoded)
		}

		var cmp testType
		if err := jsontypes.Unmarshal(bits, &cmp); err != nil {
			t.Errorf("Unmarshal %#q failed: %v", string(bits), err)
		}
		if obj != cmp {
			t.Errorf("Unmarshal %#q: got %+v, want %+v", string(bits), cmp, obj)
		}
	})

	t.Run("Unregistered", func(t *testing.T) {
		obj := testType{Field: "hello"}
		bits, err := jsontypes.Marshal(&obj)
		if err != nil {
			t.Fatalf("Marshal %T failed: %v", obj, err)
		}
		if got := string(bits); got != wantEncoded {
			t.Errorf("Marshal %T: got %#q, want %#q", obj, got, wantEncoded)
		}

		var cmp struct {
			Field string `json:"field"`
		}
		if err := jsontypes.Unmarshal(bits, &cmp); err != nil {
			t.Errorf("Unmarshal %#q: got %+v, want %+v", string(bits), cmp, obj)
		}
	})
}
