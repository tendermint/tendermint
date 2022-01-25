// Package query implements the custom query format used to filter event
// subscriptions in Tendermint.
//
// Query expressions describe properties of events and their attributes, using
// strings like:
//
//    abci.invoice.number = 22 AND abci.invoice.owner = 'Ivan'
//
// Query expressions can handle attribute values encoding numbers, strings,
// dates, and timestamps.  The complete query grammar is described in the
// query/syntax package.
//
package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query/syntax"
)

// All is a query that matches all events.
var All *Query

// A Query is the compiled form of a query.
type Query struct {
	ast   syntax.Query
	conds []condition
}

// New parses and compiles the query expression into an executable query.
func New(query string) (*Query, error) {
	ast, err := syntax.Parse(query)
	if err != nil {
		return nil, err
	}
	return Compile(ast)
}

// MustCompile compiles the query expression into an executable query.
// In case of error, MustCompile will panic.
//
// This is intended for use in program initialization; use query.New if you
// need to check errors.
func MustCompile(query string) *Query {
	q, err := New(query)
	if err != nil {
		panic(err)
	}
	return q
}

// Compile compiles the given query AST so it can be used to match events.
func Compile(ast syntax.Query) (*Query, error) {
	conds := make([]condition, len(ast))
	for i, q := range ast {
		cond, err := compileCondition(q)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", q, err)
		}
		conds[i] = cond
	}
	return &Query{ast: ast, conds: conds}, nil
}

// Matches reports whether q matches the given events. If q == nil, the query
// matches any non-empty collection of events.
func (q *Query) Matches(events []types.Event) bool {
	if q == nil {
		return true
	}
	for _, cond := range q.conds {
		if !cond.matchesAny(events) {
			return false
		}
	}
	return len(events) != 0
}

// String matches part of the pubsub.Query interface.
func (q *Query) String() string {
	if q == nil {
		return "<empty>"
	}
	return q.ast.String()
}

// Syntax returns the syntax tree representation of q.
func (q *Query) Syntax() syntax.Query {
	if q == nil {
		return nil
	}
	return q.ast
}

// A condition is a compiled match condition.  A condition matches an event if
// the event has the designated type, contains an attribute with the given
// name, and the match function returns true for the attribute value.
type condition struct {
	tag   string // e.g., "tx.hash"
	match func(s string) bool
}

// findAttr returns a slice of attribute values from event matching the
// condition tag, and reports whether the event type strictly equals the
// condition tag.
func (c condition) findAttr(event types.Event) ([]string, bool) {
	if !strings.HasPrefix(c.tag, event.Type) {
		return nil, false // type does not match tag
	} else if len(c.tag) == len(event.Type) {
		return nil, true // type == tag
	}
	var vals []string
	for _, attr := range event.Attributes {
		fullName := event.Type + "." + attr.Key
		if fullName == c.tag {
			vals = append(vals, attr.Value)
		}
	}
	return vals, false
}

// matchesAny reports whether c matches at least one of the given events.
func (c condition) matchesAny(events []types.Event) bool {
	for _, event := range events {
		if c.matchesEvent(event) {
			return true
		}
	}
	return false
}

// matchesEvent reports whether c matches the given event.
func (c condition) matchesEvent(event types.Event) bool {
	vs, tagEqualsType := c.findAttr(event)
	if len(vs) == 0 {
		// As a special case, a condition tag that exactly matches the event type
		// is matched against an empty string. This allows existence checks to
		// work for type-only queries.
		if tagEqualsType {
			return c.match("")
		}
		return false
	}

	// At this point, we have candidate values.
	for _, v := range vs {
		if c.match(v) {
			return true
		}
	}
	return false
}

func compileCondition(cond syntax.Condition) (condition, error) {
	out := condition{tag: cond.Tag}

	// Handle existence checks separately to simplify the logic below for
	// comparisons that take arguments.
	if cond.Op == syntax.TExists {
		out.match = func(string) bool { return true }
		return out, nil
	}

	// All the other operators require an argument.
	if cond.Arg == nil {
		return condition{}, fmt.Errorf("missing argument for %v", cond.Op)
	}

	// Precompile the argument value matcher.
	argType := cond.Arg.Type
	var argValue interface{}

	switch argType {
	case syntax.TString:
		argValue = cond.Arg.Value()
	case syntax.TNumber:
		argValue = cond.Arg.Number()
	case syntax.TTime, syntax.TDate:
		argValue = cond.Arg.Time()
	default:
		return condition{}, fmt.Errorf("unknown argument type %v", argType)
	}

	mcons := opTypeMap[cond.Op][argType]
	if mcons == nil {
		return condition{}, fmt.Errorf("invalid op/arg combination (%v, %v)", cond.Op, argType)
	}
	out.match = mcons(argValue)
	return out, nil
}

// TODO(creachadair): The existing implementation allows anything number shaped
// to be treated as a number. This preserves the parts of that behavior we had
// tests for, but we should probably get rid of that.
var extractNum = regexp.MustCompile(`^\d+(\.\d+)?`)

func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(extractNum.FindString(s), 64)
}

// A map of operator ⇒ argtype ⇒ match-constructor.
// An entry does not exist if the combination is not valid.
//
// Disable the dupl lint for this map. The result isn't even correct.
//nolint:dupl
var opTypeMap = map[syntax.Token]map[syntax.Token]func(interface{}) func(string) bool{
	syntax.TContains: {
		syntax.TString: func(v interface{}) func(string) bool {
			return func(s string) bool {
				return strings.Contains(s, v.(string))
			}
		},
	},
	syntax.TEq: {
		syntax.TString: func(v interface{}) func(string) bool {
			return func(s string) bool { return s == v.(string) }
		},
		syntax.TNumber: func(v interface{}) func(string) bool {
			return func(s string) bool {
				w, err := parseNumber(s)
				return err == nil && w == v.(float64)
			}
		},
		syntax.TDate: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseDate(s)
				return err == nil && ts.Equal(v.(time.Time))
			}
		},
		syntax.TTime: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseTime(s)
				return err == nil && ts.Equal(v.(time.Time))
			}
		},
	},
	syntax.TLt: {
		syntax.TNumber: func(v interface{}) func(string) bool {
			return func(s string) bool {
				w, err := parseNumber(s)
				return err == nil && w < v.(float64)
			}
		},
		syntax.TDate: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseDate(s)
				return err == nil && ts.Before(v.(time.Time))
			}
		},
		syntax.TTime: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseTime(s)
				return err == nil && ts.Before(v.(time.Time))
			}
		},
	},
	syntax.TLeq: {
		syntax.TNumber: func(v interface{}) func(string) bool {
			return func(s string) bool {
				w, err := parseNumber(s)
				return err == nil && w <= v.(float64)
			}
		},
		syntax.TDate: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseDate(s)
				return err == nil && !ts.After(v.(time.Time))
			}
		},
		syntax.TTime: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseTime(s)
				return err == nil && !ts.After(v.(time.Time))
			}
		},
	},
	syntax.TGt: {
		syntax.TNumber: func(v interface{}) func(string) bool {
			return func(s string) bool {
				w, err := parseNumber(s)
				return err == nil && w > v.(float64)
			}
		},
		syntax.TDate: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseDate(s)
				return err == nil && ts.After(v.(time.Time))
			}
		},
		syntax.TTime: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseTime(s)
				return err == nil && ts.After(v.(time.Time))
			}
		},
	},
	syntax.TGeq: {
		syntax.TNumber: func(v interface{}) func(string) bool {
			return func(s string) bool {
				w, err := parseNumber(s)
				return err == nil && w >= v.(float64)
			}
		},
		syntax.TDate: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseDate(s)
				return err == nil && !ts.Before(v.(time.Time))
			}
		},
		syntax.TTime: func(v interface{}) func(string) bool {
			return func(s string) bool {
				ts, err := syntax.ParseTime(s)
				return err == nil && !ts.Before(v.(time.Time))
			}
		},
	},
}
