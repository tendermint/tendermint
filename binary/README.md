### NOTICE

This documentation is out of date.
* 0x00 is reserved as a nil byte for RegisterInterface
* moved TypeByte() into RegisterInterface/ConcreteType
* Pointers that don't have a declared TypeByte() are
  encoded with a leading 0x00 (nil) or 0x01.

# `tendermint/binary`

The `binary` submodule encodes primary types and structs into bytes.

## Primary types

uint\*, int\*, string, time, byteslice and byteslice-slice types can be
encoded and decoded with the following methods:

The following writes `o uint64` to `w io.Writer`, and increments `n` and/or sets `err`
```go
WriteUint64(o uint64, w io.Writer, n *int64, err *error)

// Typical usage:
buf, n, err := new(bytes.Buffer), new(int64), new(error)
WriteUint64(uint64(x), buf, n, err)
if *err != nil {
    panic(err)
}

```

The following reads a `uint64` from `r io.Reader`, and increments `n` and/or sets `err`
```go
var o = ReadUint64(r io.Reader, n *int64, err *error)
```

Similar methods for `uint32`, `uint16`, `uint8`, `int64`, `int32`, `int16`, `int8` exist.
Protobuf variable length encoding is done with `uint` and `int` types:
```go
WriteUvarint(o uint, w io.Writer, n *int64, err *error)
var o = ReadUvarint(r io.Reader, n *int64, err *error)
```

Byteslices can be written with:
```go
WriteByteSlice(bz []byte, w io.Writer, n *int64, err *error)
```

Byteslices (and all slices such as byteslice-slices) are prepended with
`uvarint` encoded length, so `ReadByteSlice()` knows how many bytes to read.

Note that there is no type information encoded -- the caller is assumed to know what types
to decode.

## Struct Types

Struct types can be automatically encoded with reflection.  Unlike json-encoding, no field
name or type information is encoded.  Field values are simply encoded in order.

```go
type Foo struct {
    MyString        string
    MyUint32        uint32
    myPrivateBytes  []byte
}

foo := Foo{"my string", math.MaxUint32, []byte("my private bytes")}

buf, n, err := new(bytes.Buffer), new(int64), new(error)
WriteBinary(foo, buf, n, err)

// fmt.Printf("%X", buf.Bytes()) gives:
// 096D7920737472696E67FFFFFFFF
// 09:                           uvarint encoded length of string "my string"
//   6D7920737472696E67:         bytes of string "my string"
//                     FFFFFFFF: bytes for MaxUint32 
// Note that the unexported "myPrivateBytes" isn't encoded.

foo2 := ReadBinary(Foo{}, buf, n, err).(Foo)

// Or, to decode onto a pointer:
foo2 := ReadBinaryPtr(&Foo{}, buf, n, err).(*Foo)
```

WriteBinary and ReadBinary can encode/decode structs recursively. However, interface field
values are a bit more complicated.

```go
type Greeter interface {
	Greet() string
}

type Dog struct{}
func (d Dog) Greet() string { return "Woof!" }

type Cat struct{}
func (c Cat) Greet() string { return "Meow!" }

type Foo struct {
	Greeter
}

foo := Foo{Dog{}}

buf, n, err := new(bytes.Buffer), new(int64), new(error)
WriteBinary(foo, buf, n, err)

// This errors because we don't know whether to read a Dog or Cat.
foo2 := ReadBinary(Foo{}, buf, n, err)
```

In the above example, `ReadBinary()` fails because the `Greeter` field for `Foo{}`
is ambiguous -- it could be either a `Dog{}` or a `Cat{}`, like a union structure.
The solution is to declare the concrete implementation types for interfaces:

```go
type Dog struct{}
func (d Dog) TypeByte() byte { return GreeterTypeDog }
func (d Dog) Greet() string { return "Woof!" }

type Cat struct{}
func (c Cat) TypeByte() byte { return GreeterTypeCat }
func (c Cat) Greet() string { return "Meow!" }

var _ = RegisterInterface(
	struct{Greeter}{},
	ConcreteType{Dog{}},
	ConcreteType{Cat{}},
})
```

NOTE: The TypeByte() is written and expected to be read even when the struct
is encoded or decoded directly:

```go
WriteBinary(Dog{}, buf, n, err)        // Writes GreeterTypeDog byte
dog_ := ReadBinary(Dog{}, buf, n, err) // Expects to read GreeterTypeDog byte
dog := dog_.(Dog)                      // ok if *err != nil, otherwise dog_ == nil.
```
