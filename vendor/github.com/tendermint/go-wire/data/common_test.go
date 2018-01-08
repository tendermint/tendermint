package data_test

import (
	"strings"

	data "github.com/tendermint/go-wire/data"
)

/** These are some sample types to test parsing **/

type Fooer interface {
	Foo() string
}

type Bar struct {
	Name string `json:"name"`
}

func (b Bar) Foo() string {
	return "Bar " + b.Name
}

type Baz struct {
	Name string `json:"name"`
}

func (b Baz) Foo() string {
	return strings.Replace(b.Name, "r", "z", -1)
}

type Nested struct {
	Prefix string `json:"prefix"`
	Sub    FooerS `json:"sub"`
}

func (n Nested) Foo() string {
	return n.Prefix + ": " + n.Sub.Foo()
}

/** This is parse code: todo - autogenerate **/

var fooersParser data.Mapper

type FooerS struct {
	Fooer
}

func (f FooerS) MarshalJSON() ([]byte, error) {
	return fooersParser.ToJSON(f.Fooer)
}

func (f *FooerS) UnmarshalJSON(data []byte) (err error) {
	parsed, err := fooersParser.FromJSON(data)
	if err == nil {
		f.Fooer = parsed.(Fooer)
	}
	return
}

// Set is a helper to deal with wrapped interfaces
func (f *FooerS) Set(foo Fooer) {
	f.Fooer = foo
}

/** end TO-BE auto-generated code **/

/** This connects our code with the auto-generated helpers **/

// this init must come after the above init (which should be in a file from import)
func init() {
	fooersParser = data.NewMapper(FooerS{}).
		RegisterImplementation(Bar{}, "bar", 0x01).
		RegisterImplementation(Baz{}, "baz", 0x02).
		RegisterImplementation(Nested{}, "nest", 0x03)
}
