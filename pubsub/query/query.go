// Package query provides a parser for a custom query format:
//
//		abci.invoice.number=22 AND abci.invoice.owner=Ivan
//
// See query.peg for the grammar, which is a https://en.wikipedia.org/wiki/Parsing_expression_grammar.
// More: https://github.com/PhilippeSigaud/Pegged/wiki/PEG-Basics
//
// It has a support for numbers (integer and floating point), dates and times.
package query

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Query holds the query string and the query parser.
type Query struct {
	str    string
	parser *QueryParser
}

// New parses the given string and returns a query or error if the string is
// invalid.
func New(s string) (*Query, error) {
	p := &QueryParser{Buffer: fmt.Sprintf(`"%s"`, s)}
	p.Init()
	if err := p.Parse(); err != nil {
		return nil, err
	}
	return &Query{str: s, parser: p}, nil
}

// MustParse turns the given string into a query or panics; for tests or others
// cases where you know the string is valid.
func MustParse(s string) *Query {
	q, err := New(s)
	if err != nil {
		panic(fmt.Sprintf("failed to parse %s: %v", s, err))
	}
	return q
}

// String returns the original string.
func (q *Query) String() string {
	return q.str
}

type operator uint8

const (
	opLessEqual operator = iota
	opGreaterEqual
	opLess
	opGreater
	opEqual
	opContains
)

// Matches returns true if the query matches the given set of tags, false otherwise.
//
// For example, query "name=John" matches tags = {"name": "John"}. More
// examples could be found in parser_test.go and query_test.go.
func (q *Query) Matches(tags map[string]interface{}) bool {
	if len(tags) == 0 {
		return false
	}

	buffer, begin, end := q.parser.Buffer, 0, 0

	var tag string
	var op operator

	// tokens must be in the following order: tag ("tx.gas") -> operator ("=") -> operand ("7")
	for _, token := range q.parser.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
		case ruletag:
			tag = buffer[begin:end]
		case rulele:
			op = opLessEqual
		case rulege:
			op = opGreaterEqual
		case rulel:
			op = opLess
		case ruleg:
			op = opGreater
		case ruleequal:
			op = opEqual
		case rulecontains:
			op = opContains
		case rulevalue:
			// strip single quotes from value (i.e. "'NewBlock'" -> "NewBlock")
			valueWithoutSingleQuotes := buffer[begin+1 : end-1]

			// see if the triplet (tag, operator, operand) matches any tag
			// "tx.gas", "=", "7", { "tx.gas": 7, "tx.ID": "4AE393495334" }
			if !match(tag, op, reflect.ValueOf(valueWithoutSingleQuotes), tags) {
				return false
			}
		case rulenumber:
			number := buffer[begin:end]
			if strings.Contains(number, ".") { // if it looks like a floating-point number
				value, err := strconv.ParseFloat(number, 64)
				if err != nil {
					panic(fmt.Sprintf("got %v while trying to parse %s as float64 (should never happen if the grammar is correct)", err, number))
				}
				if !match(tag, op, reflect.ValueOf(value), tags) {
					return false
				}
			} else {
				value, err := strconv.ParseInt(number, 10, 64)
				if err != nil {
					panic(fmt.Sprintf("got %v while trying to parse %s as int64 (should never happen if the grammar is correct)", err, number))
				}
				if !match(tag, op, reflect.ValueOf(value), tags) {
					return false
				}
			}
		case ruletime:
			value, err := time.Parse(time.RFC3339, buffer[begin:end])
			if err != nil {
				panic(fmt.Sprintf("got %v while trying to parse %s as time.Time / RFC3339 (should never happen if the grammar is correct)", err, buffer[begin:end]))
			}
			if !match(tag, op, reflect.ValueOf(value), tags) {
				return false
			}
		case ruledate:
			value, err := time.Parse("2006-01-02", buffer[begin:end])
			if err != nil {
				panic(fmt.Sprintf("got %v while trying to parse %s as time.Time / '2006-01-02' (should never happen if the grammar is correct)", err, buffer[begin:end]))
			}
			if !match(tag, op, reflect.ValueOf(value), tags) {
				return false
			}
		}
	}

	return true
}

// match returns true if the given triplet (tag, operator, operand) matches any tag.
//
// First, it looks up the tag in tags and if it finds one, tries to compare the
// value from it to the operand using the operator.
//
// "tx.gas", "=", "7", { "tx.gas": 7, "tx.ID": "4AE393495334" }
func match(tag string, op operator, operand reflect.Value, tags map[string]interface{}) bool {
	// look up the tag from the query in tags
	value, ok := tags[tag]
	if !ok {
		return false
	}
	switch operand.Kind() {
	case reflect.Struct: // time
		operandAsTime := operand.Interface().(time.Time)
		v, ok := value.(time.Time)
		if !ok { // if value from tags is not time.Time
			return false
		}
		switch op {
		case opLessEqual:
			return v.Before(operandAsTime) || v.Equal(operandAsTime)
		case opGreaterEqual:
			return v.Equal(operandAsTime) || v.After(operandAsTime)
		case opLess:
			return v.Before(operandAsTime)
		case opGreater:
			return v.After(operandAsTime)
		case opEqual:
			return v.Equal(operandAsTime)
		}
	case reflect.Float64:
		operandFloat64 := operand.Interface().(float64)
		var v float64
		// try our best to convert value from tags to float64
		switch vt := value.(type) {
		case float64:
			v = vt
		case float32:
			v = float64(vt)
		case int:
			v = float64(vt)
		case int8:
			v = float64(vt)
		case int16:
			v = float64(vt)
		case int32:
			v = float64(vt)
		case int64:
			v = float64(vt)
		default: // fail for all other types
			panic(fmt.Sprintf("Incomparable types: %T (%v) vs float64 (%v)", value, value, operandFloat64))
		}
		switch op {
		case opLessEqual:
			return v <= operandFloat64
		case opGreaterEqual:
			return v >= operandFloat64
		case opLess:
			return v < operandFloat64
		case opGreater:
			return v > operandFloat64
		case opEqual:
			return v == operandFloat64
		}
	case reflect.Int64:
		operandInt := operand.Interface().(int64)
		var v int64
		// try our best to convert value from tags to int64
		switch vt := value.(type) {
		case int64:
			v = vt
		case int8:
			v = int64(vt)
		case int16:
			v = int64(vt)
		case int32:
			v = int64(vt)
		case int:
			v = int64(vt)
		case float64:
			v = int64(vt)
		case float32:
			v = int64(vt)
		default: // fail for all other types
			panic(fmt.Sprintf("Incomparable types: %T (%v) vs int64 (%v)", value, value, operandInt))
		}
		switch op {
		case opLessEqual:
			return v <= operandInt
		case opGreaterEqual:
			return v >= operandInt
		case opLess:
			return v < operandInt
		case opGreater:
			return v > operandInt
		case opEqual:
			return v == operandInt
		}
	case reflect.String:
		v, ok := value.(string)
		if !ok { // if value from tags is not string
			return false
		}
		switch op {
		case opEqual:
			return v == operand.String()
		case opContains:
			return strings.Contains(v, operand.String())
		}
	default:
		panic(fmt.Sprintf("Unknown kind of operand %v", operand.Kind()))
	}

	return false
}
