package strings

import (
	"fmt"
	"strings"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type lazyStringf struct {
	tmpl string
	args []interface{}
	out  string
}

func (s *lazyStringf) String() string {
	if s.out == "" && s.tmpl != "" {
		s.out = fmt.Sprintf(s.tmpl, s.args)
		s.args = nil
		s.tmpl = ""
	}
	return s.out
}

// LazySprintf creates a fmt.Stringer implementation with similar
// semantics as fmt.Sprintf, *except* that the string is built when
// String() is called on the object. This means that format arguments
// are resolved/captured into string format when String() is called,
// and not, as in fmt.Sprintf when that function returns.
//
// As a result, if you use this type in go routines or defer
// statements it's possible to pass an argument to LazySprintf which
// has one value at the call site and a different value when the
// String() is evaluated, which may lead to unexpected outcomes. In
// these situations, either be *extremely* careful about the arguments
// passed to this function or use fmt.Sprintf.
//
// The implementation also caches the output of the underlying
// fmt.Sprintf statement when String() is called, so subsequent calls
// will produce the same result.
func LazySprintf(t string, args ...interface{}) fmt.Stringer {
	return &lazyStringf{tmpl: t, args: args}
}

type lazyStringer struct {
	val fmt.Stringer
	out string
}

func (l *lazyStringer) String() string {
	if l.out == "" && l.val != nil {
		l.out = l.val.String()
		l.val = nil
	}
	return l.out
}

// LazyStringer captures a fmt.Stringer implementation resolving the
// underlying string *only* when the String() method is called and
// caching the result for future use.
func LazyStringer(v fmt.Stringer) fmt.Stringer { return &lazyStringer{val: v} }

type lazyBlockHash struct {
	block interface{ Hash() tmbytes.HexBytes }
	out   string
}

// LazyBlockHash defers block Hash until the Stringer interface is invoked.
// This is particularly useful for avoiding calling Sprintf when debugging is not
// active.
//
// As a result, if you use this type in go routines or defer
// statements it's possible to pass an argument to LazyBlockHash that
// has one value at the call site and a different value when the
// String() is evaluated, which may lead to unexpected outcomes. In
// these situations, either be *extremely* careful about the arguments
// passed to this function or use fmt.Sprintf.
//
// The implementation also caches the output of the string form of the
// block hash when String() is called, so subsequent calls will
// produce the same result.
func LazyBlockHash(block interface{ Hash() tmbytes.HexBytes }) fmt.Stringer {
	return &lazyBlockHash{block: block}
}

func (l *lazyBlockHash) String() string {
	if l.out == "" && l.block != nil {
		l.out = l.block.Hash().String()
		l.block = nil
	}
	return l.out
}

// SplitAndTrimEmpty slices s into all subslices separated by sep and returns a
// slice of the string s with all leading and trailing Unicode code points
// contained in cutset removed. If sep is empty, SplitAndTrim splits after each
// UTF-8 sequence. First part is equivalent to strings.SplitN with a count of
// -1.  also filter out empty strings, only return non-empty strings.
func SplitAndTrimEmpty(s, sep, cutset string) []string {
	if s == "" {
		return []string{}
	}

	spl := strings.Split(s, sep)
	nonEmptyStrings := make([]string, 0, len(spl))

	for i := 0; i < len(spl); i++ {
		element := strings.Trim(spl[i], cutset)
		if element != "" {
			nonEmptyStrings = append(nonEmptyStrings, element)
		}
	}

	return nonEmptyStrings
}

// ASCIITrim removes spaces from an a ASCII string, erroring if the
// sequence is not an ASCII string.
func ASCIITrim(s string) (string, error) {
	if len(s) == 0 {
		return "", nil
	}
	r := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		switch {
		case b == 32:
			continue // skip space
		case 32 < b && b <= 126:
			r = append(r, b)
		default:
			return "", fmt.Errorf("non-ASCII (non-tab) char 0x%X", b)
		}
	}
	return string(r), nil
}

// StringSliceEqual checks if string slices a and b are equal
func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
