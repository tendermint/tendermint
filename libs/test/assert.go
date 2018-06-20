package test

import (
	"testing"
)

func AssertPanics(t *testing.T, msg string, f func()) {
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("Should have panic'd, but didn't: %v", msg)
		}
	}()
	f()
}
