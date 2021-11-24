package syntax_test

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/tendermint/tendermint/libs/pubsub/query/syntax"
)

func TestScanner(t *testing.T) {
	tests := []struct {
		input string
		want  []syntax.Token
	}{
		// Empty input.
		{"", nil},
		{"  ", nil},
		{"\t\n ", nil},

		{`0 123`, []syntax.Token{syntax.TNumber, syntax.TNumber}},
		{`0.32 3.14`, []syntax.Token{syntax.TNumber, syntax.TNumber}},

		// Tags
		{`foo foo.bar`, []syntax.Token{syntax.TTag, syntax.TTag}},
		{` '' x 'x' 'x y'`, []syntax.Token{syntax.TString, syntax.TTag, syntax.TString, syntax.TString}},
		{` 'you are not your job' `, []syntax.Token{syntax.TString}},
		{`< <= = > >=`, []syntax.Token{
			syntax.TLt, syntax.TLeq, syntax.TEq, syntax.TGt, syntax.TGeq,
		}},
		{`x AND y`, []syntax.Token{syntax.TTag, syntax.TAnd, syntax.TTag}},
		{`x.y CONTAINS 'z'`, []syntax.Token{syntax.TTag, syntax.TContains, syntax.TString}},
		{`foo EXISTS`, []syntax.Token{syntax.TTag, syntax.TExists}},
		{`and AND`, []syntax.Token{syntax.TTag, syntax.TAnd}},
		{`TIME 2021-11-23T15:16:17Z`, []syntax.Token{syntax.TTime}},
		{`DATE 2021-11-23`, []syntax.Token{syntax.TDate}},
	}

	for _, test := range tests {
		s := syntax.NewScanner(strings.NewReader(test.input))
		var got []syntax.Token
		for s.Next() == nil {
			got = append(got, s.Token())
		}
		if err := s.Err(); err != io.EOF {
			t.Errorf("Next: unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Logf("Scanner input: %q", test.input)
			t.Errorf("Wrong tokens:\ngot:  %+v\nwant: %+v", got, test.want)
		}
	}
}

func TestScannerErrors(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`'incomplete string`},
		{`-23`},
		{`&`},
		{`DATE xyz-pdq`},
		{`DATE xyzp-dq-zv`},
		{`DATE 0000-00-00`},
		{`DATE 0000-00-000`},
		{`DATE 2021-01-99`},
		{`TIME 2021-01-01T34:56:78Z`},
		{`TIME 2021-01-99T14:56:08Z`},
		{`TIME 2021-01-99T34:56:08`},
		{`TIME 2021-01-99T34:56:11+3`},
	}
	for _, test := range tests {
		s := syntax.NewScanner(strings.NewReader(test.input))
		if err := s.Next(); err == nil {
			t.Errorf("Next: got %v (%#q), want error", s.Token(), s.Text())
		}
	}
}
