package sync

import "testing"

func TestDefaultValue(t *testing.T) {
	t.Parallel()
	v := New()
	if v.IsSet() {
		t.Fatal("Empty value of AtomicBool should be false")
	}

	v = NewBool(true)
	if !v.IsSet() {
		t.Fatal("NewValue(true) should be true")
	}

	v = NewBool(false)
	if v.IsSet() {
		t.Fatal("NewValue(false) should be false")
	}
}

func TestSetUnSet(t *testing.T) {
	t.Parallel()
	v := New()

	v.Set()
	if !v.IsSet() {
		t.Fatal("AtomicBool.Set() failed")
	}

	v.UnSet()
	if v.IsSet() {
		t.Fatal("AtomicBool.UnSet() failed")
	}
}
