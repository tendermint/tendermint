# Wire encoding for Golang

This software implements Go bindings for the Wire encoding protocol.
The goal of the Wire encoding protocol is to be a simple language-agnostic encoding protocol for rapid prototyping of blockchain applications.

This package also includes a compatible (and slower) JSON codec.

### Supported types

**Primary types**: `uvarint`, `varint`, `byte`, `uint[8,16,32,64]`, `int[8,16,32,64]`, `string`, and `time` types are supported

**Arrays**: Arrays can hold items of any arbitrary type.  For example, byte-arrays and byte-array-arrays are supported.

**Structs**: Struct fields are encoded by value (without the key name) in the order that they are declared in the struct.  In this way it is similar to Apache Avro.

**Interfaces**: Interfaces are like union types where the value can be any non-interface type. The actual value is preceded by a single "type byte" that shows which concrete is encoded.

**Pointers**: Pointers are like optional fields.  The first byte is 0x00 to denote a null pointer (e.g. no value), otherwise it is 0x01.

### Unsupported types

**Maps**: Maps are not supported because for most languages, key orders are nondeterministic.
If you need to encode/decode maps of arbitrary key-value pairs, encode an array of {key,value} structs instead.

**Floating points**: Floating point number types are discouraged because [of reasons](http://gafferongames.com/networking-for-game-programmers/floating-point-determinism/).  If you need to use them, use the field tag `wire:"unsafe"`.

**Enums**: Enum types are not supported in all languages, and they're simple enough to model as integers anyways.

### A struct example

Struct types can be automatically encoded with reflection.  Unlike json-encoding, no field
name or type information is encoded.  Field values are simply encoded in order.

```go
package main

import (
  "bytes"
  "fmt"
  "math"
  "github.com/tendermint/go-wire"
)

type Foo struct {
  MyString       string
  MyUint32       uint32
  myPrivateBytes []byte
}

func main() {

  foo := Foo{"my string", math.MaxUint32, []byte("my private bytes")}

  buf, n, err := new(bytes.Buffer), int(0), error(nil)
  wire.WriteBinary(foo, buf, &n, &err)

  fmt.Printf("%X\n", buf.Bytes())
}
```

The above example prints:

```
01096D7920737472696E67FFFFFFFF, where

0109                            is the varint encoding of the length of string "my string"
    6D7920737472696E67          is the bytes of string "my string"
                      FFFFFFFF  is the bytes for math.MaxUint32, a uint32
```

Note that the unexported "myPrivateBytes" isn't encoded.

### An interface example

Here's an example with interfaces.

```go
package main

import (
  "bytes"
  "fmt"
  "github.com/tendermint/go-wire"
)

type Animal interface{}
type Dog struct{ Name string }
type Cat struct{ Name string }
type Cow struct{ Name string }

var _ = wire.RegisterInterface(
  struct{ Animal }{},
  wire.ConcreteType{Dog{}, 0x01}, // type-byte of 0x01 for Dogs
  wire.ConcreteType{Cat{}, 0x02}, // type-byte of 0x02 for Cats
  wire.ConcreteType{Cow{}, 0x03}, // type-byte of 0x03 for Cows
)

func main() {

  animals := []Animal{
    Dog{"Snoopy"},
    Cow{"Daisy"},
  }

  buf, n, err := new(bytes.Buffer), int(0), error(nil)
  wire.WriteBinary(animals, buf, &n, &err)

  fmt.Printf("%X\n", buf.Bytes())
}
```

The above example prints:

```
0102010106536E6F6F70790301054461697379, where

0102                                    is the varint encoding of the length of the array
    01                                  is the type-byte for a Dog
      0106                              is the varint encoding of the length of the Dog's name
          536E6F6F7079                  is the Dog's name "Snoopy"
                      03                is the type-byte for a Cow
                        0105            is the varint encoding of the length of the Cow's name
                            4461697379  is the Cow's name "Daisy"
```

### A pointer example

Here's an example with pointers (and interfaces too).

```go
package main

import (
	"bytes"
	"fmt"
	"github.com/tendermint/go-wire"
)

type Animal interface{}
type Dog struct{ Name string }
type Cat struct{ Name string }
type Cow struct{ Name string }

var _ = wire.RegisterInterface(
	struct{ Animal }{},
	wire.ConcreteType{Dog{}, 0x01},  // type-byte of 0x01 for Dogs
	wire.ConcreteType{&Dog{}, 0x02}, // type-byte of 0x02 for Dog pointers
)

type MyStruct struct {
	Field1 Animal
	Field2 *Dog
	Field3 *Dog
}

func main() {

	myStruct := MyStruct{
		Field1: &Dog{"Snoopy"},
		Field2: &Dog{"Smappy"},
		Field3: (*Dog)(nil),
	}

	buf, n, err := new(bytes.Buffer), int(0), error(nil)
	wire.WriteBinary(myStruct, buf, &n, &err)

	fmt.Printf("%X\n", buf.Bytes())
}
```

The above example prints:

```
020106536E6F6F7079010106536D6170707900, where

02                                      is the type-byte for a Dog pointer for Field1
  0106                                  is the varint encoding of the length of the Dog's name
      536E6F6F7079                      is the Dog's name "Snoopy"
                  01                    is a byte indicating a non-null pointer for Field2
                    0106                is the varint encoding of the length of the Dog's name
                        536D61707079    is the Dog's name "Smappy"
                                    00  is a byte indicating a null pointer for Field3
```

Notice that in Field1, that the value is non-null is implied in the type-byte of 0x02.
While Golang lets you have nil-pointers as interface values, this is a Golang-specific feature that is absent in other OOP languages
such as Java.  So, Go-Wire does not support nil-pointers for interface values.  The following example would return an error:

```go
myStruct := MyStruct{
  Field1: (*Dog)(nil),    // Error!
  Field2: &Dog{"Smappy"}, // Ok!
  Field3: (*Dog)(nil),    // Ok!
}

buf, n, err := new(bytes.Buffer), int(0), error(nil)
wire.WriteBinary(myStruct, buf, &n, &err)
fmt.Println(err)

// Unexpected nil-pointer of type main.Dog for registered interface Animal.
// For compatibility with other languages, nil-pointer interface values are forbidden.
```
