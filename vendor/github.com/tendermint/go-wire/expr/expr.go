
package expr

import (
	"bytes"
  "encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
  "strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var g = &grammar {
	rules: []*rule{
{
	name: "Start",
	pos: position{line: 19, col: 1, offset: 148},
	expr: &actionExpr{
	pos: position{line: 19, col: 9, offset: 158},
	run: (*parser).callonStart1,
	expr: &seqExpr{
	pos: position{line: 19, col: 9, offset: 158},
	exprs: []interface{}{
&labeledExpr{
	pos: position{line: 19, col: 9, offset: 158},
	label: "items_",
	expr: &oneOrMoreExpr{
	pos: position{line: 19, col: 16, offset: 165},
	expr: &ruleRefExpr{
	pos: position{line: 19, col: 16, offset: 165},
	name: "Item",
},
},
},
&ruleRefExpr{
	pos: position{line: 19, col: 22, offset: 171},
	name: "_",
},
&ruleRefExpr{
	pos: position{line: 19, col: 24, offset: 173},
	name: "EOF",
},
	},
},
},
},
{
	name: "Item",
	pos: position{line: 27, col: 1, offset: 307},
	expr: &actionExpr{
	pos: position{line: 27, col: 8, offset: 316},
	run: (*parser).callonItem1,
	expr: &seqExpr{
	pos: position{line: 27, col: 8, offset: 316},
	exprs: []interface{}{
&ruleRefExpr{
	pos: position{line: 27, col: 8, offset: 316},
	name: "_",
},
&labeledExpr{
	pos: position{line: 27, col: 10, offset: 318},
	label: "it",
	expr: &choiceExpr{
	pos: position{line: 27, col: 14, offset: 322},
	alternatives: []interface{}{
&ruleRefExpr{
	pos: position{line: 27, col: 14, offset: 322},
	name: "Array",
},
&ruleRefExpr{
	pos: position{line: 27, col: 22, offset: 330},
	name: "Tuple",
},
&ruleRefExpr{
	pos: position{line: 27, col: 30, offset: 338},
	name: "Hex",
},
&ruleRefExpr{
	pos: position{line: 27, col: 36, offset: 344},
	name: "TypedNumeric",
},
&ruleRefExpr{
	pos: position{line: 27, col: 51, offset: 359},
	name: "UntypedNumeric",
},
&ruleRefExpr{
	pos: position{line: 27, col: 68, offset: 376},
	name: "Placeholder",
},
&ruleRefExpr{
	pos: position{line: 27, col: 82, offset: 390},
	name: "String",
},
	},
},
},
	},
},
},
},
{
	name: "Array",
	pos: position{line: 31, col: 1, offset: 422},
	expr: &actionExpr{
	pos: position{line: 31, col: 9, offset: 432},
	run: (*parser).callonArray1,
	expr: &seqExpr{
	pos: position{line: 31, col: 9, offset: 432},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 31, col: 9, offset: 432},
	val: "[",
	ignoreCase: false,
},
&labeledExpr{
	pos: position{line: 31, col: 13, offset: 436},
	label: "items",
	expr: &zeroOrMoreExpr{
	pos: position{line: 31, col: 19, offset: 442},
	expr: &ruleRefExpr{
	pos: position{line: 31, col: 19, offset: 442},
	name: "Item",
},
},
},
&ruleRefExpr{
	pos: position{line: 31, col: 25, offset: 448},
	name: "_",
},
&litMatcher{
	pos: position{line: 31, col: 27, offset: 450},
	val: "]",
	ignoreCase: false,
},
	},
},
},
},
{
	name: "Tuple",
	pos: position{line: 35, col: 1, offset: 504},
	expr: &actionExpr{
	pos: position{line: 35, col: 9, offset: 514},
	run: (*parser).callonTuple1,
	expr: &seqExpr{
	pos: position{line: 35, col: 9, offset: 514},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 35, col: 9, offset: 514},
	val: "(",
	ignoreCase: false,
},
&labeledExpr{
	pos: position{line: 35, col: 13, offset: 518},
	label: "items",
	expr: &oneOrMoreExpr{
	pos: position{line: 35, col: 19, offset: 524},
	expr: &ruleRefExpr{
	pos: position{line: 35, col: 19, offset: 524},
	name: "Item",
},
},
},
&ruleRefExpr{
	pos: position{line: 35, col: 25, offset: 530},
	name: "_",
},
&litMatcher{
	pos: position{line: 35, col: 27, offset: 532},
	val: ")",
	ignoreCase: false,
},
	},
},
},
},
{
	name: "UntypedNumeric",
	pos: position{line: 39, col: 1, offset: 586},
	expr: &actionExpr{
	pos: position{line: 39, col: 18, offset: 605},
	run: (*parser).callonUntypedNumeric1,
	expr: &labeledExpr{
	pos: position{line: 39, col: 18, offset: 605},
	label: "number",
	expr: &ruleRefExpr{
	pos: position{line: 39, col: 25, offset: 612},
	name: "Integer",
},
},
},
},
{
	name: "TypedNumeric",
	pos: position{line: 46, col: 1, offset: 704},
	expr: &actionExpr{
	pos: position{line: 46, col: 16, offset: 721},
	run: (*parser).callonTypedNumeric1,
	expr: &seqExpr{
	pos: position{line: 46, col: 16, offset: 721},
	exprs: []interface{}{
&labeledExpr{
	pos: position{line: 46, col: 16, offset: 721},
	label: "t",
	expr: &ruleRefExpr{
	pos: position{line: 46, col: 18, offset: 723},
	name: "Type",
},
},
&litMatcher{
	pos: position{line: 46, col: 23, offset: 728},
	val: ":",
	ignoreCase: false,
},
&labeledExpr{
	pos: position{line: 46, col: 27, offset: 732},
	label: "number",
	expr: &ruleRefExpr{
	pos: position{line: 46, col: 34, offset: 739},
	name: "Integer",
},
},
	},
},
},
},
{
	name: "Hex",
	pos: position{line: 53, col: 1, offset: 838},
	expr: &choiceExpr{
	pos: position{line: 53, col: 7, offset: 846},
	alternatives: []interface{}{
&ruleRefExpr{
	pos: position{line: 53, col: 7, offset: 846},
	name: "HexLengthPrefixed",
},
&ruleRefExpr{
	pos: position{line: 53, col: 27, offset: 866},
	name: "HexRaw",
},
	},
},
},
{
	name: "HexLengthPrefixed",
	pos: position{line: 55, col: 1, offset: 874},
	expr: &actionExpr{
	pos: position{line: 55, col: 21, offset: 896},
	run: (*parser).callonHexLengthPrefixed1,
	expr: &seqExpr{
	pos: position{line: 55, col: 21, offset: 896},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 55, col: 21, offset: 896},
	val: "0x",
	ignoreCase: false,
},
&labeledExpr{
	pos: position{line: 55, col: 26, offset: 901},
	label: "hexbytes",
	expr: &ruleRefExpr{
	pos: position{line: 55, col: 35, offset: 910},
	name: "HexBytes",
},
},
	},
},
},
},
{
	name: "HexRaw",
	pos: position{line: 59, col: 1, offset: 974},
	expr: &actionExpr{
	pos: position{line: 59, col: 10, offset: 985},
	run: (*parser).callonHexRaw1,
	expr: &seqExpr{
	pos: position{line: 59, col: 10, offset: 985},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 59, col: 10, offset: 985},
	val: "x",
	ignoreCase: false,
},
&labeledExpr{
	pos: position{line: 59, col: 14, offset: 989},
	label: "hexbytes",
	expr: &ruleRefExpr{
	pos: position{line: 59, col: 23, offset: 998},
	name: "HexBytes",
},
},
	},
},
},
},
{
	name: "HexBytes",
	pos: position{line: 63, col: 1, offset: 1063},
	expr: &actionExpr{
	pos: position{line: 63, col: 12, offset: 1076},
	run: (*parser).callonHexBytes1,
	expr: &oneOrMoreExpr{
	pos: position{line: 63, col: 12, offset: 1076},
	expr: &charClassMatcher{
	pos: position{line: 63, col: 12, offset: 1076},
	val: "[0-9abcdefABCDEF]",
	chars: []rune{'a','b','c','d','e','f','A','B','C','D','E','F',},
	ranges: []rune{'0','9',},
	ignoreCase: false,
	inverted: false,
},
},
},
},
{
	name: "Type",
	pos: position{line: 71, col: 1, offset: 1223},
	expr: &actionExpr{
	pos: position{line: 71, col: 8, offset: 1232},
	run: (*parser).callonType1,
	expr: &choiceExpr{
	pos: position{line: 71, col: 9, offset: 1233},
	alternatives: []interface{}{
&litMatcher{
	pos: position{line: 71, col: 9, offset: 1233},
	val: "u64",
	ignoreCase: false,
},
&litMatcher{
	pos: position{line: 71, col: 17, offset: 1241},
	val: "i64",
	ignoreCase: false,
},
	},
},
},
},
{
	name: "Integer",
	pos: position{line: 75, col: 1, offset: 1284},
	expr: &actionExpr{
	pos: position{line: 75, col: 11, offset: 1296},
	run: (*parser).callonInteger1,
	expr: &seqExpr{
	pos: position{line: 75, col: 11, offset: 1296},
	exprs: []interface{}{
&zeroOrOneExpr{
	pos: position{line: 75, col: 11, offset: 1296},
	expr: &litMatcher{
	pos: position{line: 75, col: 11, offset: 1296},
	val: "-",
	ignoreCase: false,
},
},
&oneOrMoreExpr{
	pos: position{line: 75, col: 16, offset: 1301},
	expr: &charClassMatcher{
	pos: position{line: 75, col: 16, offset: 1301},
	val: "[0-9]",
	ranges: []rune{'0','9',},
	ignoreCase: false,
	inverted: false,
},
},
	},
},
},
},
{
	name: "Label",
	pos: position{line: 79, col: 1, offset: 1344},
	expr: &actionExpr{
	pos: position{line: 79, col: 9, offset: 1354},
	run: (*parser).callonLabel1,
	expr: &oneOrMoreExpr{
	pos: position{line: 79, col: 9, offset: 1354},
	expr: &charClassMatcher{
	pos: position{line: 79, col: 9, offset: 1354},
	val: "[0-9a-zA-Z:]",
	chars: []rune{':',},
	ranges: []rune{'0','9','a','z','A','Z',},
	ignoreCase: false,
	inverted: false,
},
},
},
},
{
	name: "Placeholder",
	pos: position{line: 83, col: 1, offset: 1404},
	expr: &actionExpr{
	pos: position{line: 83, col: 15, offset: 1420},
	run: (*parser).callonPlaceholder1,
	expr: &seqExpr{
	pos: position{line: 83, col: 15, offset: 1420},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 83, col: 15, offset: 1420},
	val: "<",
	ignoreCase: false,
},
&labeledExpr{
	pos: position{line: 83, col: 19, offset: 1424},
	label: "label",
	expr: &ruleRefExpr{
	pos: position{line: 83, col: 25, offset: 1430},
	name: "Label",
},
},
&litMatcher{
	pos: position{line: 83, col: 31, offset: 1436},
	val: ">",
	ignoreCase: false,
},
	},
},
},
},
{
	name: "String",
	pos: position{line: 89, col: 1, offset: 1509},
	expr: &actionExpr{
	pos: position{line: 89, col: 10, offset: 1520},
	run: (*parser).callonString1,
	expr: &seqExpr{
	pos: position{line: 89, col: 10, offset: 1520},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 89, col: 10, offset: 1520},
	val: "\"",
	ignoreCase: false,
},
&zeroOrMoreExpr{
	pos: position{line: 89, col: 14, offset: 1524},
	expr: &choiceExpr{
	pos: position{line: 89, col: 16, offset: 1526},
	alternatives: []interface{}{
&seqExpr{
	pos: position{line: 89, col: 16, offset: 1526},
	exprs: []interface{}{
&notExpr{
	pos: position{line: 89, col: 16, offset: 1526},
	expr: &ruleRefExpr{
	pos: position{line: 89, col: 17, offset: 1527},
	name: "EscapedChar",
},
},
&anyMatcher{
	line: 89, col: 29, offset: 1539,
},
	},
},
&seqExpr{
	pos: position{line: 89, col: 33, offset: 1543},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 89, col: 33, offset: 1543},
	val: "\\",
	ignoreCase: false,
},
&ruleRefExpr{
	pos: position{line: 89, col: 38, offset: 1548},
	name: "EscapeSequence",
},
	},
},
	},
},
},
&litMatcher{
	pos: position{line: 89, col: 56, offset: 1566},
	val: "\"",
	ignoreCase: false,
},
	},
},
},
},
{
	name: "EscapedChar",
	pos: position{line: 100, col: 1, offset: 1839},
	expr: &charClassMatcher{
	pos: position{line: 100, col: 15, offset: 1855},
	val: "[\\x00-\\x1f\"\\\\]",
	chars: []rune{'"','\\',},
	ranges: []rune{'\x00','\x1f',},
	ignoreCase: false,
	inverted: false,
},
},
{
	name: "EscapeSequence",
	pos: position{line: 102, col: 1, offset: 1871},
	expr: &choiceExpr{
	pos: position{line: 102, col: 18, offset: 1890},
	alternatives: []interface{}{
&ruleRefExpr{
	pos: position{line: 102, col: 18, offset: 1890},
	name: "SingleCharEscape",
},
&ruleRefExpr{
	pos: position{line: 102, col: 37, offset: 1909},
	name: "UnicodeEscape",
},
	},
},
},
{
	name: "SingleCharEscape",
	pos: position{line: 104, col: 1, offset: 1924},
	expr: &charClassMatcher{
	pos: position{line: 104, col: 20, offset: 1945},
	val: "[\"\\\\/bfnrt]",
	chars: []rune{'"','\\','/','b','f','n','r','t',},
	ignoreCase: false,
	inverted: false,
},
},
{
	name: "UnicodeEscape",
	pos: position{line: 106, col: 1, offset: 1958},
	expr: &seqExpr{
	pos: position{line: 106, col: 17, offset: 1976},
	exprs: []interface{}{
&litMatcher{
	pos: position{line: 106, col: 17, offset: 1976},
	val: "u",
	ignoreCase: false,
},
&ruleRefExpr{
	pos: position{line: 106, col: 21, offset: 1980},
	name: "HexDigit",
},
&ruleRefExpr{
	pos: position{line: 106, col: 30, offset: 1989},
	name: "HexDigit",
},
&ruleRefExpr{
	pos: position{line: 106, col: 39, offset: 1998},
	name: "HexDigit",
},
&ruleRefExpr{
	pos: position{line: 106, col: 48, offset: 2007},
	name: "HexDigit",
},
	},
},
},
{
	name: "HexDigit",
	pos: position{line: 108, col: 1, offset: 2017},
	expr: &charClassMatcher{
	pos: position{line: 108, col: 12, offset: 2030},
	val: "[0-9a-f]i",
	ranges: []rune{'0','9','a','f',},
	ignoreCase: true,
	inverted: false,
},
},
{
	name: "_",
	displayName: "\"whitespace\"",
	pos: position{line: 110, col: 1, offset: 2041},
	expr: &zeroOrMoreExpr{
	pos: position{line: 110, col: 18, offset: 2060},
	expr: &charClassMatcher{
	pos: position{line: 110, col: 18, offset: 2060},
	val: "[ \\n\\t\\r]",
	chars: []rune{' ','\n','\t','\r',},
	ignoreCase: false,
	inverted: false,
},
},
},
{
	name: "EOF",
	pos: position{line: 112, col: 1, offset: 2072},
	expr: &notExpr{
	pos: position{line: 112, col: 7, offset: 2080},
	expr: &anyMatcher{
	line: 112, col: 8, offset: 2081,
},
},
},
	},
}
func (c *current) onStart1(items_ interface{}) (interface{}, error) {
    items := items_.([]interface{})
    if len(items) == 1 {
        return items[0], nil
    }
    return Tuple(items), nil
}

func (p *parser) callonStart1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onStart1(stack["items_"])
}

func (c *current) onItem1(it interface{}) (interface{}, error) {
    return it, nil
}

func (p *parser) callonItem1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onItem1(stack["it"])
}

func (c *current) onArray1(items interface{}) (interface{}, error) {
    return Array(items.([]interface{})), nil
}

func (p *parser) callonArray1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArray1(stack["items"])
}

func (c *current) onTuple1(items interface{}) (interface{}, error) {
    return Tuple(items.([]interface{})), nil
}

func (p *parser) callonTuple1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onTuple1(stack["items"])
}

func (c *current) onUntypedNumeric1(number interface{}) (interface{}, error) {
    return Numeric{
      Type: "i",
      Number: number.(string),
    }, nil
}

func (p *parser) callonUntypedNumeric1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onUntypedNumeric1(stack["number"])
}

func (c *current) onTypedNumeric1(t, number interface{}) (interface{}, error) {
    return Numeric{
      Type: t.(string),
      Number: number.(string),
    }, nil
}

func (p *parser) callonTypedNumeric1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onTypedNumeric1(stack["t"], stack["number"])
}

func (c *current) onHexLengthPrefixed1(hexbytes interface{}) (interface{}, error) {
    return NewBytes(hexbytes.([]byte), true), nil
}

func (p *parser) callonHexLengthPrefixed1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onHexLengthPrefixed1(stack["hexbytes"])
}

func (c *current) onHexRaw1(hexbytes interface{}) (interface{}, error) {
    return NewBytes(hexbytes.([]byte), false), nil
}

func (p *parser) callonHexRaw1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onHexRaw1(stack["hexbytes"])
}

func (c *current) onHexBytes1() (interface{}, error) {
    bytez, err := hex.DecodeString(string(c.text))
    if err != nil {
        return nil, err
    }
    return bytez, nil
}

func (p *parser) callonHexBytes1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onHexBytes1()
}

func (c *current) onType1() (interface{}, error) {
    return string(c.text), nil
}

func (p *parser) callonType1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onType1()
}

func (c *current) onInteger1() (interface{}, error) {
    return string(c.text), nil
}

func (p *parser) callonInteger1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInteger1()
}

func (c *current) onLabel1() (interface{}, error) {
    return string(c.text), nil
}

func (p *parser) callonLabel1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLabel1()
}

func (c *current) onPlaceholder1(label interface{}) (interface{}, error) {
    return Placeholder{
      Label: label.(string),
    }, nil
}

func (p *parser) callonPlaceholder1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onPlaceholder1(stack["label"])
}

func (c *current) onString1() (interface{}, error) {
    // TODO : the forward slash (solidus) is not a valid escape in Go, it will
    // fail if there's one in the string
    text, err := strconv.Unquote(string(c.text))
    if err != nil {
      return nil, err
    } else {
      return NewString(text), nil
    }
}

func (p *parser) callonString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onString1()
}


var (
	// errNoRule is returned when the grammar to parse has no rule.
	errNoRule          = errors.New("grammar has no rule")

	// errInvalidEncoding is returned when the source is not properly
	// utf8-encoded.
	errInvalidEncoding = errors.New("invalid encoding")

	// errNoMatch is returned if no match could be found.
	errNoMatch         = errors.New("no match found")
)

// Option is a function that can set an option on the parser. It returns
// the previous setting as an Option.
type Option func(*parser) Option

// Debug creates an Option to set the debug flag to b. When set to true,
// debugging information is printed to stdout while parsing.
//
// The default is false.
func Debug(b bool) Option {
	return func(p *parser) Option {
		old := p.debug
		p.debug = b
		return Debug(old)
	}
}

// Memoize creates an Option to set the memoize flag to b. When set to true,
// the parser will cache all results so each expression is evaluated only
// once. This guarantees linear parsing time even for pathological cases,
// at the expense of more memory and slower times for typical cases.
//
// The default is false.
func Memoize(b bool) Option {
	return func(p *parser) Option {
		old := p.memoize
		p.memoize = b
		return Memoize(old)
	}
}

// Recover creates an Option to set the recover flag to b. When set to
// true, this causes the parser to recover from panics and convert it
// to an error. Setting it to false can be useful while debugging to
// access the full stack trace.
//
// The default is true.
func Recover(b bool) Option {
	return func(p *parser) Option {
		old := p.recover
		p.recover = b
		return Recover(old)
	}
}

// ParseFile parses the file identified by filename.
func ParseFile(filename string, opts ...Option) (interface{}, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseReader(filename, f, opts...)
}

// ParseReader parses the data from r using filename as information in the
// error messages.
func ParseReader(filename string, r io.Reader, opts ...Option) (interface{}, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return Parse(filename, b, opts...)
}

// Parse parses the data from b using filename as information in the
// error messages.
func Parse(filename string, b []byte, opts ...Option) (interface{}, error) {
	return newParser(filename, b, opts...).parse(g)
}

// position records a position in the text.
type position struct {
	line, col, offset int
}

func (p position) String() string {
	return fmt.Sprintf("%d:%d [%d]", p.line, p.col, p.offset)
}

// savepoint stores all state required to go back to this point in the
// parser.
type savepoint struct {
	position
	rn rune
	w  int
}

type current struct {
	pos  position // start position of the match
	text []byte   // raw text of the match
}

// the AST types...

type grammar struct {
	pos   position
	rules []*rule
}

type rule struct {
	pos         position
	name        string
	displayName string
	expr        interface{}
}

type choiceExpr struct {
	pos          position
	alternatives []interface{}
}

type actionExpr struct {
	pos    position
	expr   interface{}
	run    func(*parser) (interface{}, error)
}

type seqExpr struct {
	pos   position
	exprs []interface{}
}

type labeledExpr struct {
	pos   position
	label string
	expr  interface{}
}

type expr struct {
	pos  position
	expr interface{}
}

type andExpr expr
type notExpr expr
type zeroOrOneExpr expr
type zeroOrMoreExpr expr
type oneOrMoreExpr expr

type ruleRefExpr struct {
	pos  position
	name string
}

type andCodeExpr struct {
	pos position
	run func(*parser) (bool, error)
}

type notCodeExpr struct {
	pos position
	run func(*parser) (bool, error)
}

type litMatcher struct {
	pos        position
	val        string
	ignoreCase bool
}

type charClassMatcher struct {
	pos        position
	val        string
	chars      []rune
	ranges     []rune
	classes    []*unicode.RangeTable
	ignoreCase bool
	inverted   bool
}

type anyMatcher position

// errList cumulates the errors found by the parser.
type errList []error

func (e *errList) add(err error) {
	*e = append(*e, err)
}

func (e errList) err() error {
	if len(e) == 0 {
		return nil
	}
	e.dedupe()
	return e
}

func (e *errList) dedupe() {
	var cleaned []error
	set := make(map[string]bool)
	for _, err := range *e {
		if msg := err.Error(); !set[msg] {
			set[msg] = true
			cleaned = append(cleaned, err)
		}
	}
	*e = cleaned
}

func (e errList) Error() string {
	switch len(e) {
	case 0:
		return ""
	case 1:
		return e[0].Error()
	default:
		var buf bytes.Buffer

		for i, err := range e {
			if i > 0 {
				buf.WriteRune('\n')
			}
			buf.WriteString(err.Error())
		}
		return buf.String()
	}
}

// parserError wraps an error with a prefix indicating the rule in which
// the error occurred. The original error is stored in the Inner field.
type parserError struct {
	Inner  error
	pos    position
	prefix string
}

// Error returns the error message.
func (p *parserError) Error() string {
	return p.prefix + ": " + p.Inner.Error()
}

// newParser creates a parser with the specified input source and options.
func newParser(filename string, b []byte, opts ...Option) *parser {
	p := &parser{
		filename: filename,
		errs: new(errList),
		data: b,
		pt: savepoint{position: position{line: 1}},
		recover: true,
	}
	p.setOptions(opts)
	return p
}

// setOptions applies the options to the parser.
func (p *parser) setOptions(opts []Option) {
	for _, opt := range opts {
		opt(p)
	}
}

type resultTuple struct {
	v interface{}
	b bool
	end savepoint
}

type parser struct {
	filename string
	pt       savepoint
	cur      current

	data []byte
	errs *errList

	recover bool
	debug bool
	depth  int

	memoize bool
	// memoization table for the packrat algorithm:
	// map[offset in source] map[expression or rule] {value, match}
	memo map[int]map[interface{}]resultTuple

	// rules table, maps the rule identifier to the rule node
	rules  map[string]*rule
	// variables stack, map of label to value
	vstack []map[string]interface{}
	// rule stack, allows identification of the current rule in errors
	rstack []*rule

	// stats
	exprCnt int
}

// push a variable set on the vstack.
func (p *parser) pushV() {
	if cap(p.vstack) == len(p.vstack) {
		// create new empty slot in the stack
		p.vstack = append(p.vstack, nil)
	} else {
		// slice to 1 more
		p.vstack = p.vstack[:len(p.vstack)+1]
	}

	// get the last args set
	m := p.vstack[len(p.vstack)-1]
	if m != nil && len(m) == 0 {
		// empty map, all good
		return
	}

	m = make(map[string]interface{})
	p.vstack[len(p.vstack)-1] = m
}

// pop a variable set from the vstack.
func (p *parser) popV() {
	// if the map is not empty, clear it
	m := p.vstack[len(p.vstack)-1]
	if len(m) > 0 {
		// GC that map
		p.vstack[len(p.vstack)-1] = nil
	}
	p.vstack = p.vstack[:len(p.vstack)-1]
}

func (p *parser) print(prefix, s string) string {
	if !p.debug {
		return s
	}

	fmt.Printf("%s %d:%d:%d: %s [%#U]\n",
		prefix, p.pt.line, p.pt.col, p.pt.offset, s, p.pt.rn)
	return s
}

func (p *parser) in(s string) string {
	p.depth++
	return p.print(strings.Repeat(" ", p.depth) + ">", s)
}

func (p *parser) out(s string) string {
	p.depth--
	return p.print(strings.Repeat(" ", p.depth) + "<", s)
}

func (p *parser) addErr(err error) {
	p.addErrAt(err, p.pt.position)
}

func (p *parser) addErrAt(err error, pos position) {
	var buf bytes.Buffer
	if p.filename != "" {
		buf.WriteString(p.filename)
	}
	if buf.Len() > 0 {
		buf.WriteString(":")
	}
	buf.WriteString(fmt.Sprintf("%d:%d (%d)", pos.line, pos.col, pos.offset))
	if len(p.rstack) > 0 {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}
		rule := p.rstack[len(p.rstack)-1]
		if rule.displayName != "" {
			buf.WriteString("rule " + rule.displayName)
		} else {
			buf.WriteString("rule " + rule.name)
		}
	}
	pe := &parserError{Inner: err, prefix: buf.String()}
	p.errs.add(pe)
}

// read advances the parser to the next rune.
func (p *parser) read() {
	p.pt.offset += p.pt.w
	rn, n := utf8.DecodeRune(p.data[p.pt.offset:])
	p.pt.rn = rn
	p.pt.w = n
	p.pt.col++
	if rn == '\n' {
		p.pt.line++
		p.pt.col = 0
	}

	if rn == utf8.RuneError {
		if n > 0 {
			p.addErr(errInvalidEncoding)
		}
	}
}

// restore parser position to the savepoint pt.
func (p *parser) restore(pt savepoint) {
	if p.debug {
		defer p.out(p.in("restore"))
	}
	if pt.offset == p.pt.offset {
		return
	}
	p.pt = pt
}

// get the slice of bytes from the savepoint start to the current position.
func (p *parser) sliceFrom(start savepoint) []byte {
	return p.data[start.position.offset:p.pt.position.offset]
}

func (p *parser) getMemoized(node interface{}) (resultTuple, bool) {
	if len(p.memo) == 0 {
		return resultTuple{}, false
	}
	m := p.memo[p.pt.offset]
	if len(m) == 0 {
		return resultTuple{}, false
	}
	res, ok := m[node]
	return res, ok
}

func (p *parser) setMemoized(pt savepoint, node interface{}, tuple resultTuple) {
	if p.memo == nil {
		p.memo = make(map[int]map[interface{}]resultTuple)
	}
	m := p.memo[pt.offset]
	if m == nil {
		m = make(map[interface{}]resultTuple)
		p.memo[pt.offset] = m
	}
	m[node] = tuple
}

func (p *parser) buildRulesTable(g *grammar) {
	p.rules = make(map[string]*rule, len(g.rules))
	for _, r := range g.rules {
		p.rules[r.name] = r
	}
}

func (p *parser) parse(g *grammar) (val interface{}, err error) {
	if len(g.rules) == 0 {
		p.addErr(errNoRule)
		return nil, p.errs.err()
	}

	// TODO : not super critical but this could be generated
	p.buildRulesTable(g)

	if p.recover {
		// panic can be used in action code to stop parsing immediately
		// and return the panic as an error.
		defer func() {
			if e := recover(); e != nil {
				if p.debug {
					defer p.out(p.in("panic handler"))
				}
				val = nil
				switch e := e.(type) {
				case error:
					p.addErr(e)
				default:
					p.addErr(fmt.Errorf("%v", e))
				}
				err = p.errs.err()
			}
		}()
	}

	// start rule is rule [0]
	p.read() // advance to first rune
	val, ok := p.parseRule(g.rules[0])
	if !ok {
		if len(*p.errs) == 0 {
			// make sure this doesn't go out silently
			p.addErr(errNoMatch)
		}
		return nil, p.errs.err()
	}
	return val, p.errs.err()
}

func (p *parser) parseRule(rule *rule) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseRule " + rule.name))
	}

	if p.memoize {
		res, ok := p.getMemoized(rule)
		if ok {
			p.restore(res.end)
			return res.v, res.b
		}
	}

	start := p.pt
	p.rstack = append(p.rstack, rule)
	p.pushV()
	val, ok := p.parseExpr(rule.expr)
	p.popV()
	p.rstack = p.rstack[:len(p.rstack)-1]
	if ok && p.debug {
		p.print(strings.Repeat(" ", p.depth) + "MATCH", string(p.sliceFrom(start)))
	}

	if p.memoize {
		p.setMemoized(start, rule, resultTuple{val, ok, p.pt})
	}
	return val, ok
}

func (p *parser) parseExpr(expr interface{}) (interface{}, bool) {
	var pt savepoint
	var ok bool

	if p.memoize {
		res, ok := p.getMemoized(expr)
		if ok {
			p.restore(res.end)
			return res.v, res.b
		}
		pt = p.pt
	}

	p.exprCnt++
	var val interface{}
	switch expr := expr.(type) {
	case *actionExpr:
		val, ok = p.parseActionExpr(expr)
	case *andCodeExpr:
		val, ok = p.parseAndCodeExpr(expr)
	case *andExpr:
		val, ok = p.parseAndExpr(expr)
	case *anyMatcher:
		val, ok = p.parseAnyMatcher(expr)
	case *charClassMatcher:
		val, ok = p.parseCharClassMatcher(expr)
	case *choiceExpr:
		val, ok = p.parseChoiceExpr(expr)
	case *labeledExpr:
		val, ok = p.parseLabeledExpr(expr)
	case *litMatcher:
		val, ok = p.parseLitMatcher(expr)
	case *notCodeExpr:
		val, ok = p.parseNotCodeExpr(expr)
	case *notExpr:
		val, ok = p.parseNotExpr(expr)
	case *oneOrMoreExpr:
		val, ok = p.parseOneOrMoreExpr(expr)
	case *ruleRefExpr:
		val, ok = p.parseRuleRefExpr(expr)
	case *seqExpr:
		val, ok = p.parseSeqExpr(expr)
	case *zeroOrMoreExpr:
		val, ok = p.parseZeroOrMoreExpr(expr)
	case *zeroOrOneExpr:
		val, ok = p.parseZeroOrOneExpr(expr)
	default:
		panic(fmt.Sprintf("unknown expression type %T", expr))
	}
	if p.memoize {
		p.setMemoized(pt, expr, resultTuple{val, ok, p.pt})
	}
	return val, ok
}

func (p *parser) parseActionExpr(act *actionExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseActionExpr"))
	}

	start := p.pt
	val, ok := p.parseExpr(act.expr)
	if ok {
		p.cur.pos = start.position
		p.cur.text = p.sliceFrom(start)
		actVal, err := act.run(p)
		if err != nil {
			p.addErrAt(err, start.position)
		}
		val = actVal
	}
	if ok && p.debug {
		p.print(strings.Repeat(" ", p.depth) + "MATCH", string(p.sliceFrom(start)))
	}
	return val, ok
}

func (p *parser) parseAndCodeExpr(and *andCodeExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAndCodeExpr"))
	}

	ok, err := and.run(p)
	if err != nil {
		p.addErr(err)
	}
	return nil, ok
}

func (p *parser) parseAndExpr(and *andExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAndExpr"))
	}

	pt := p.pt
	p.pushV()
	_, ok := p.parseExpr(and.expr)
	p.popV()
	p.restore(pt)
	return nil, ok
}

func (p *parser) parseAnyMatcher(any *anyMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAnyMatcher"))
	}

	if p.pt.rn != utf8.RuneError {
		start := p.pt
		p.read()
		return p.sliceFrom(start), true
	}
	return nil, false
}

func (p *parser) parseCharClassMatcher(chr *charClassMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseCharClassMatcher"))
	}

	cur := p.pt.rn
	// can't match EOF
	if cur == utf8.RuneError {
		return nil, false
	}
	start := p.pt
	if chr.ignoreCase {
		cur = unicode.ToLower(cur)
	}

	// try to match in the list of available chars
	for _, rn := range chr.chars {
		if rn == cur {
			if chr.inverted {
				return nil, false
			}
			p.read()
			return p.sliceFrom(start), true
		}
	}

	// try to match in the list of ranges
	for i := 0; i < len(chr.ranges); i += 2 {
		if cur >= chr.ranges[i] && cur <= chr.ranges[i+1] {
			if chr.inverted {
				return nil, false
			}
			p.read()
			return p.sliceFrom(start), true
		}
	}

	// try to match in the list of Unicode classes
	for _, cl := range chr.classes {
		if unicode.Is(cl, cur) {
			if chr.inverted {
				return nil, false
			}
			p.read()
			return p.sliceFrom(start), true
		}
	}

	if chr.inverted {
		p.read()
		return p.sliceFrom(start), true
	}
	return nil, false
}

func (p *parser) parseChoiceExpr(ch *choiceExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseChoiceExpr"))
	}

	for _, alt := range ch.alternatives {
		p.pushV()
		val, ok := p.parseExpr(alt)
		p.popV()
		if ok {
			return val, ok
		}
	}
	return nil, false
}

func (p *parser) parseLabeledExpr(lab *labeledExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseLabeledExpr"))
	}

	p.pushV()
	val, ok := p.parseExpr(lab.expr)
	p.popV()
	if ok && lab.label != "" {
		m := p.vstack[len(p.vstack)-1]
		m[lab.label] = val
	}
	return val, ok
}

func (p *parser) parseLitMatcher(lit *litMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseLitMatcher"))
	}

	start := p.pt
	for _, want := range lit.val {
		cur := p.pt.rn
		if lit.ignoreCase {
			cur = unicode.ToLower(cur)
		}
		if cur != want {
			p.restore(start)
			return nil, false
		}
		p.read()
	}
	return p.sliceFrom(start), true
}

func (p *parser) parseNotCodeExpr(not *notCodeExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseNotCodeExpr"))
	}

	ok, err := not.run(p)
	if err != nil {
		p.addErr(err)
	}
	return nil, !ok
}

func (p *parser) parseNotExpr(not *notExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseNotExpr"))
	}

	pt := p.pt
	p.pushV()
	_, ok := p.parseExpr(not.expr)
	p.popV()
	p.restore(pt)
	return nil, !ok
}

func (p *parser) parseOneOrMoreExpr(expr *oneOrMoreExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseOneOrMoreExpr"))
	}

	var vals []interface{}

	for {
		p.pushV()
		val, ok := p.parseExpr(expr.expr)
		p.popV()
		if !ok {
			if len(vals) == 0 {
				// did not match once, no match
				return nil, false
			}
			return vals, true
		}
		vals = append(vals, val)
	}
}

func (p *parser) parseRuleRefExpr(ref *ruleRefExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseRuleRefExpr " + ref.name))
	}

	if ref.name == "" {
		panic(fmt.Sprintf("%s: invalid rule: missing name", ref.pos))
	}

	rule := p.rules[ref.name]
	if rule == nil {
		p.addErr(fmt.Errorf("undefined rule: %s", ref.name))
		return nil, false
	}
	return p.parseRule(rule)
}

func (p *parser) parseSeqExpr(seq *seqExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseSeqExpr"))
	}

	var vals []interface{}

	pt := p.pt
	for _, expr := range seq.exprs {
		val, ok := p.parseExpr(expr)
		if !ok {
			p.restore(pt)
			return nil, false
		}
		vals = append(vals, val)
	}
	return vals, true
}

func (p *parser) parseZeroOrMoreExpr(expr *zeroOrMoreExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseZeroOrMoreExpr"))
	}

	var vals []interface{}

	for {
		p.pushV()
		val, ok := p.parseExpr(expr.expr)
		p.popV()
		if !ok {
			return vals, true
		}
		vals = append(vals, val)
	}
}

func (p *parser) parseZeroOrOneExpr(expr *zeroOrOneExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseZeroOrOneExpr"))
	}

	p.pushV()
	val, _ := p.parseExpr(expr.expr)
	p.popV()
	// whether it matched or not, consider it a match
	return val, true
}

func rangeTable(class string) *unicode.RangeTable {
	if rt, ok := unicode.Categories[class]; ok {
		return rt
	}
	if rt, ok := unicode.Properties[class]; ok {
		return rt
	}
	if rt, ok := unicode.Scripts[class]; ok {
		return rt
	}

	// cannot happen
	panic(fmt.Sprintf("invalid Unicode class: %s", class))
}

