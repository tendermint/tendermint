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

	"github.com/tendermint/tendermint/libs/pubsub"
)

// Query holds the query string and the query parser.
type Query struct {
	str    string
	parser *QueryParser
}

// Condition represents a single condition within a query and consists of tag
// (e.g. "tx.gas"), operator (e.g. "=") and operand (e.g. "7").
type Condition struct {
	Tag     string
	Op      Operator
	Operand interface{}
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

// Operator is an operator that defines some kind of relation between tag and
// operand (equality, etc.).
type Operator uint8

const (
	// "<="
	OpLessEqual Operator = iota
	// ">="
	OpGreaterEqual
	// "<"
	OpLess
	// ">"
	OpGreater
	// "="
	OpEqual
	// "CONTAINS"; used to check if a string contains a certain sub string.
	OpContains
)

const (
	// DateLayout defines a layout for all dates (`DATE date`)
	DateLayout = "2006-01-02"
	// TimeLayout defines a layout for all times (`TIME time`)
	TimeLayout = time.RFC3339
)

// Conditions returns a list of conditions.
func (q *Query) Conditions() []Condition {
	conditions := make([]Condition, 0)

	buffer, begin, end := q.parser.Buffer, 0, 0

	var tag string
	var op Operator

	// tokens must be in the following order: tag ("tx.gas") -> operator ("=") -> operand ("7")
	for _, token := range q.parser.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
		case ruletag:
			tag = buffer[begin:end]
		case rulele:
			op = OpLessEqual
		case rulege:
			op = OpGreaterEqual
		case rulel:
			op = OpLess
		case ruleg:
			op = OpGreater
		case ruleequal:
			op = OpEqual
		case rulecontains:
			op = OpContains
		case rulevalue:
			// strip single quotes from value (i.e. "'NewBlock'" -> "NewBlock")
			valueWithoutSingleQuotes := buffer[begin+1 : end-1]
			conditions = append(conditions, Condition{tag, op, valueWithoutSingleQuotes})
		case rulenumber:
			number := buffer[begin:end]
			if strings.ContainsAny(number, ".") { // if it looks like a floating-point number
				value, err := strconv.ParseFloat(number, 64)
				if err != nil {
					panic(fmt.Sprintf("got %v while trying to parse %s as float64 (should never happen if the grammar is correct)", err, number))
				}
				conditions = append(conditions, Condition{tag, op, value})
			} else {
				value, err := strconv.ParseInt(number, 10, 64)
				if err != nil {
					panic(fmt.Sprintf("got %v while trying to parse %s as int64 (should never happen if the grammar is correct)", err, number))
				}
				conditions = append(conditions, Condition{tag, op, value})
			}
		case ruletime:
			value, err := time.Parse(TimeLayout, buffer[begin:end])
			if err != nil {
				panic(fmt.Sprintf("got %v while trying to parse %s as time.Time / RFC3339 (should never happen if the grammar is correct)", err, buffer[begin:end]))
			}
			conditions = append(conditions, Condition{tag, op, value})
		case ruledate:
			value, err := time.Parse("2006-01-02", buffer[begin:end])
			if err != nil {
				panic(fmt.Sprintf("got %v while trying to parse %s as time.Time / '2006-01-02' (should never happen if the grammar is correct)", err, buffer[begin:end]))
			}
			conditions = append(conditions, Condition{tag, op, value})
		}
	}

	return conditions
}

// Matches returns true if the query matches the given set of tags, false otherwise.
//
// For example, query "name=John" matches tags = {"name": "John"}. More
// examples could be found in parser_test.go and query_test.go.
func (q *Query) Matches(tags pubsub.TagMap) bool {
	if tags.Len() == 0 {
		return false
	}

	buffer, begin, end := q.parser.Buffer, 0, 0

	var tag string
	var op Operator

	// tokens must be in the following order: tag ("tx.gas") -> operator ("=") -> operand ("7")
	for _, token := range q.parser.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
		case ruletag:
			tag = buffer[begin:end]
		case rulele:
			op = OpLessEqual
		case rulege:
			op = OpGreaterEqual
		case rulel:
			op = OpLess
		case ruleg:
			op = OpGreater
		case ruleequal:
			op = OpEqual
		case rulecontains:
			op = OpContains
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
			if strings.ContainsAny(number, ".") { // if it looks like a floating-point number
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
			value, err := time.Parse(TimeLayout, buffer[begin:end])
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
func match(tag string, op Operator, operand reflect.Value, tags pubsub.TagMap) bool {
	// look up the tag from the query in tags
	value, ok := tags.Get(tag)
	if !ok {
		return false
	}
	switch operand.Kind() {
	case reflect.Struct: // time
		operandAsTime := operand.Interface().(time.Time)
		// try our best to convert value from tags to time.Time
		var (
			v   time.Time
			err error
		)
		if strings.ContainsAny(value, "T") {
			v, err = time.Parse(TimeLayout, value)
		} else {
			v, err = time.Parse(DateLayout, value)
		}
		if err != nil {
			panic(fmt.Sprintf("Failed to convert value %v from tag to time.Time: %v", value, err))
		}
		switch op {
		case OpLessEqual:
			return v.Before(operandAsTime) || v.Equal(operandAsTime)
		case OpGreaterEqual:
			return v.Equal(operandAsTime) || v.After(operandAsTime)
		case OpLess:
			return v.Before(operandAsTime)
		case OpGreater:
			return v.After(operandAsTime)
		case OpEqual:
			return v.Equal(operandAsTime)
		}
	case reflect.Float64:
		operandFloat64 := operand.Interface().(float64)
		var v float64
		// try our best to convert value from tags to float64
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert value %v from tag to float64: %v", value, err))
		}
		switch op {
		case OpLessEqual:
			return v <= operandFloat64
		case OpGreaterEqual:
			return v >= operandFloat64
		case OpLess:
			return v < operandFloat64
		case OpGreater:
			return v > operandFloat64
		case OpEqual:
			return v == operandFloat64
		}
	case reflect.Int64:
		operandInt := operand.Interface().(int64)
		var v int64
		// if value looks like float, we try to parse it as float
		if strings.ContainsAny(value, ".") {
			v1, err := strconv.ParseFloat(value, 64)
			if err != nil {
				panic(fmt.Sprintf("Failed to convert value %v from tag to float64: %v", value, err))
			}
			v = int64(v1)
		} else {
			var err error
			// try our best to convert value from tags to int64
			v, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				panic(fmt.Sprintf("Failed to convert value %v from tag to int64: %v", value, err))
			}
		}
		switch op {
		case OpLessEqual:
			return v <= operandInt
		case OpGreaterEqual:
			return v >= operandInt
		case OpLess:
			return v < operandInt
		case OpGreater:
			return v > operandInt
		case OpEqual:
			return v == operandInt
		}
	case reflect.String:
		switch op {
		case OpEqual:
			return value == operand.String()
		case OpContains:
			return strings.Contains(value, operand.String())
		}
	default:
		panic(fmt.Sprintf("Unknown kind of operand %v", operand.Kind()))
	}

	return false
}
