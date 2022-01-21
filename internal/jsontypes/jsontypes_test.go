package jsontypes_test

import (
	"testing"

	"github.com/tendermint/tendermint/internal/jsontypes"
)

type testPtrType struct {
	Field string `json:"field"`
}

func (*testPtrType) TypeTag() string { return "test/PointerType" }
func (t *testPtrType) Value() string { return t.Field }

type testBareType struct {
	Field string `json:"field"`
}

func (testBareType) TypeTag() string { return "test/BareType" }
func (t testBareType) Value() string { return t.Field }

type fielder interface{ Value() string }

func TestRoundTrip(t *testing.T) {
	t.Run("MustRegister_ok", func(t *testing.T) {
		defer func() {
			if x := recover(); x != nil {
				t.Fatalf("Registration panicked: %v", x)
			}
		}()
		jsontypes.MustRegister((*testPtrType)(nil))
		jsontypes.MustRegister(testBareType{})
	})

	t.Run("MustRegister_fail", func(t *testing.T) {
		defer func() {
			if x := recover(); x != nil {
				t.Logf("Got expected panic: %v", x)
			}
		}()
		jsontypes.MustRegister((*testPtrType)(nil))
		t.Fatal("Registration should not have succeeded")
	})

	t.Run("Marshal_nilTagged", func(t *testing.T) {
		bits, err := jsontypes.Marshal(nil)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if got := string(bits); got != "null" {
			t.Errorf("Marshal nil: got %#q, want null", got)
		}
	})

	t.Run("RoundTrip_pointerType", func(t *testing.T) {
		const wantEncoded = `{"type":"test/PointerType","value":{"field":"hello"}}`

		obj := testPtrType{Field: "hello"}
		bits, err := jsontypes.Marshal(&obj)
		if err != nil {
			t.Fatalf("Marshal %T failed: %v", obj, err)
		}
		if got := string(bits); got != wantEncoded {
			t.Errorf("Marshal %T: got %#q, want %#q", obj, got, wantEncoded)
		}

		var cmp testPtrType
		if err := jsontypes.Unmarshal(bits, &cmp); err != nil {
			t.Errorf("Unmarshal %#q failed: %v", string(bits), err)
		}
		if obj != cmp {
			t.Errorf("Unmarshal %#q: got %+v, want %+v", string(bits), cmp, obj)
		}
	})

	t.Run("RoundTrip_bareType", func(t *testing.T) {
		const wantEncoded = `{"type":"test/BareType","value":{"field":"hello"}}`

		obj := testBareType{Field: "hello"}
		bits, err := jsontypes.Marshal(&obj)
		if err != nil {
			t.Fatalf("Marshal %T failed: %v", obj, err)
		}
		if got := string(bits); got != wantEncoded {
			t.Errorf("Marshal %T: got %#q, want %#q", obj, got, wantEncoded)
		}

		var cmp testBareType
		if err := jsontypes.Unmarshal(bits, &cmp); err != nil {
			t.Errorf("Unmarshal %#q failed: %v", string(bits), err)
		}
		if obj != cmp {
			t.Errorf("Unmarshal %#q: got %+v, want %+v", string(bits), cmp, obj)
		}
	})

	t.Run("Unmarshal_nilPointer", func(t *testing.T) {
		var obj *testBareType

		// Unmarshaling to a nil pointer target should report an error.
		if err := jsontypes.Unmarshal([]byte(`null`), obj); err == nil {
			t.Errorf("Unmarshal nil: got %+v, wanted error", obj)
		} else {
			t.Logf("Unmarshal correctly failed: %v", err)
		}
	})

	t.Run("Unmarshal_bareType", func(t *testing.T) {
		const want = "foobar"
		const input = `{"type":"test/BareType","value":{"field":"` + want + `"}}`

		var obj testBareType
		if err := jsontypes.Unmarshal([]byte(input), &obj); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if obj.Field != want {
			t.Errorf("Unmarshal result: got %q, want %q", obj.Field, want)
		}
	})

	t.Run("Unmarshal_bareType_interface", func(t *testing.T) {
		const want = "foobar"
		const input = `{"type":"test/BareType","value":{"field":"` + want + `"}}`

		var obj fielder
		if err := jsontypes.Unmarshal([]byte(input), &obj); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if got := obj.Value(); got != want {
			t.Errorf("Unmarshal result: got %q, want %q", got, want)
		}
	})

	t.Run("Unmarshal_pointerType", func(t *testing.T) {
		const want = "bazquux"
		const input = `{"type":"test/PointerType","value":{"field":"` + want + `"}}`

		var obj testPtrType
		if err := jsontypes.Unmarshal([]byte(input), &obj); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if obj.Field != want {
			t.Errorf("Unmarshal result: got %q, want %q", obj.Field, want)
		}
	})

	t.Run("Unmarshal_pointerType_interface", func(t *testing.T) {
		const want = "foobar"
		const input = `{"type":"test/PointerType","value":{"field":"` + want + `"}}`

		var obj fielder
		if err := jsontypes.Unmarshal([]byte(input), &obj); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if got := obj.Value(); got != want {
			t.Errorf("Unmarshal result: got %q, want %q", got, want)
		}
	})

	t.Run("Unmarshal_unknownTypeTag", func(t *testing.T) {
		const input = `{"type":"test/Nonesuch","value":null}`

		// An unregistered type tag in a valid envelope should report an error.
		var obj interface{}
		if err := jsontypes.Unmarshal([]byte(input), &obj); err == nil {
			t.Errorf("Unmarshal: got %+v, wanted error", obj)
		} else {
			t.Logf("Unmarshal correctly failed: %v", err)
		}
	})

	t.Run("Unmarshal_similarTarget", func(t *testing.T) {
		const want = "zootie-zoot-zoot"
		const input = `{"type":"test/PointerType","value":{"field":"` + want + `"}}`

		// The target has a compatible (i.e., assignable) shape to the registered
		// type. This should work even though it's not the original named type.
		var cmp struct {
			Field string `json:"field"`
		}
		if err := jsontypes.Unmarshal([]byte(input), &cmp); err != nil {
			t.Errorf("Unmarshal %#q failed: %v", input, err)
		} else if cmp.Field != want {
			t.Errorf("Unmarshal result: got %q, want %q", cmp.Field, want)
		}
	})
}
