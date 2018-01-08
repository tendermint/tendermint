package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wire "github.com/tendermint/go-wire"
)

func TestSimpleBinary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cases := []struct {
		foo Fooer
	}{
		{foo: Bar{Name: "Fly"}},
		{foo: Baz{Name: "For Bar"}},
	}

	for _, tc := range cases {
		assert.NotEmpty(tc.foo.Foo())
		wrap := FooerS{tc.foo}
		parsed := FooerS{}
		d := wire.BinaryBytes(wrap)
		require.NotEmpty(d)
		err := wire.ReadBinaryBytes(d, &parsed)
		require.Nil(err, "%+v", err)
		assert.Equal(tc.foo.Foo(), parsed.Foo())
	}
}

func TestNestedBinary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cases := []struct {
		expected string
		foo      Fooer
	}{
		{"Bar Fly", Bar{Name: "Fly"}},
		{"Foz Baz", Baz{Name: "For Bar"}},
		{"My: Bar None", Nested{"My", FooerS{Bar{"None"}}}},
	}

	for _, tc := range cases {
		assert.Equal(tc.expected, tc.foo.Foo())
		wrap := FooerS{tc.foo}
		parsed := FooerS{}
		d := wire.BinaryBytes(wrap)
		require.NotEmpty(d)
		err := wire.ReadBinaryBytes(d, &parsed)
		require.Nil(err, "%+v", err)
		assert.Equal(tc.expected, parsed.Foo())
	}
}
