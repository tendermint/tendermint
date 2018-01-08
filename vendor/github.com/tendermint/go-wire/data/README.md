

# data
`import "github.com/tendermint/go-wire/data"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Data is designed to provide a standard interface and helper functions to
easily allow serialization and deserialization of your data structures
in both binary and json representations.

This is commonly needed for interpreting transactions or stored data in the
abci app, as well as accepting json input in the light-client proxy. If we
can standardize how we pass data around the app, we can also allow more
extensions, like data storage that can interpret the meaning of the []byte
passed in, and use that to index multiple fields for example.

Serialization of data is pretty automatic using standard json and go-wire
encoders.  The main issue is deserialization, especially when using interfaces
where there are many possible concrete types.

go-wire handles this by registering the types and providing a custom
deserializer:


	var _ = wire.RegisterInterface(
	  struct{ PubKey }{},
	  wire.ConcreteType{PubKeyEd25519{}, PubKeyTypeEd25519},
	  wire.ConcreteType{PubKeySecp256k1{}, PubKeyTypeSecp256k1},
	)
	
	func PubKeyFromBytes(pubKeyBytes []byte) (pubKey PubKey, err error) {
	  err = wire.ReadBinaryBytes(pubKeyBytes, &pubKey)
	  return
	}
	
	func (pubKey PubKeyEd25519) Bytes() []byte {
	  return wire.BinaryBytes(struct{ PubKey }{pubKey})
	}

This prepends a type-byte to the binary representation upon serialization and
using that byte to switch between various representations on deserialization.
go-wire also supports something similar in json, but it leads to kind of ugly
mixed-types arrays, and requires using the go-wire json parser, which is
limited relative to the standard library encoding/json library.

In json, the typical idiom is to use a type string and message data:


	{
	  "type": "this part tells you how to interpret the message",
	  "data": ...the actual message is here, in some kind of json...
	}

I took inspiration from two blog posts, that demonstrate how to use this
to build (de)serialization in a go-wire like way.

* <a href="http://eagain.net/articles/go-dynamic-json/">http://eagain.net/articles/go-dynamic-json/</a>
* <a href="http://eagain.net/articles/go-json-kind/">http://eagain.net/articles/go-json-kind/</a>

This package unifies these two in a single Mapper.

You app needs to do three things to take full advantage of this:

1. For every interface you wish to serialize, define a holder struct with some helper methods, like FooerS wraps Fooer in common_test.go
2. In all structs that include this interface, include the wrapping struct instead.  Functionally, this also fulfills the interface, so except for setting it or casting it to a sub-type it works the same.
3. Register the interface implementations as in the last init of common_test.go. If you are currently using go-wire, you should be doing this already

The benefits here is you can now run any of the following methods, both for
efficient storage in our go app, and a common format for rpc / humans.


	orig := FooerS{foo}
	
	// read/write binary a la tendermint/go-wire
	bparsed := FooerS{}
	err := wire.ReadBinaryBytes(
	  wire.BinaryBytes(orig), &bparsed)
	
	// read/write json a la encoding/json
	jparsed := FooerS{}
	j, err := json.MarshalIndent(orig, "", "\t")
	err = json.Unmarshal(j, &jparsed)

See <a href="https://github.com/tendermint/go-wire/data/blob/master/common_test.go">https://github.com/tendermint/go-wire/data/blob/master/common_test.go</a> to see
how to set up your code to use this.




## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [type ByteEncoder](#ByteEncoder)
* [type Bytes](#Bytes)
  * [func (b Bytes) MarshalJSON() ([]byte, error)](#Bytes.MarshalJSON)
  * [func (b *Bytes) UnmarshalJSON(data []byte) error](#Bytes.UnmarshalJSON)
* [type JSONMapper](#JSONMapper)
  * [func (m *JSONMapper) FromJSON(data []byte) (interface{}, error)](#JSONMapper.FromJSON)
  * [func (m *JSONMapper) ToJSON(data interface{}) ([]byte, error)](#JSONMapper.ToJSON)
* [type Mapper](#Mapper)
  * [func NewMapper(base interface{}) Mapper](#NewMapper)
  * [func (m Mapper) RegisterInterface(kind string, b byte, data interface{}) Mapper](#Mapper.RegisterInterface)


#### <a name="pkg-files">Package files</a>
[binary.go](/src/github.com/tendermint/go-wire/data/binary.go) [bytes.go](/src/github.com/tendermint/go-wire/data/bytes.go) [docs.go](/src/github.com/tendermint/go-wire/data/docs.go) [json.go](/src/github.com/tendermint/go-wire/data/json.go) [wrapper.go](/src/github.com/tendermint/go-wire/data/wrapper.go) 



## <a name="pkg-variables">Variables</a>
``` go
var (
    Encoder       ByteEncoder = hexEncoder{}
    HexEncoder                = hexEncoder{}
    B64Encoder                = base64Encoder{base64.URLEncoding}
    RawB64Encoder             = base64Encoder{base64.RawURLEncoding}
)
```
Encoder is a global setting for all byte encoding
This is the default.  Please override in the main()/init()
of your program to change how byte slices are presented




## <a name="ByteEncoder">type</a> [ByteEncoder](/src/target/bytes.go?s=1436:1547#L44)
``` go
type ByteEncoder interface {
    Marshal(bytes []byte) ([]byte, error)
    Unmarshal(dst *[]byte, src []byte) error
}
```
ByteEncoder handles both the marshalling and unmarshalling of
an arbitrary byte slice.

All Bytes use the global Encoder set in this package.
If you want to use this encoding for byte arrays, you can just
implement a simple custom marshaller for your byte array


	type Dings [64]byte
	
	func (d Dings) MarshalJSON() ([]byte, error) {
	  return data.Encoder.Marshal(d[:])
	}
	
	func (d *Dings) UnmarshalJSON(data []byte) error {
	  ref := (*d)[:]
	  return data.Encoder.Unmarshal(&ref, data)
	}










## <a name="Bytes">type</a> [Bytes](/src/target/bytes.go?s=681:698#L16)
``` go
type Bytes []byte
```
Bytes is a special byte slice that allows us to control the
serialization format per app.

Thus, basecoin could use hex, another app base64, and a third
app base58...










### <a name="Bytes.MarshalJSON">func</a> (Bytes) [MarshalJSON](/src/target/bytes.go?s=700:744#L18)
``` go
func (b Bytes) MarshalJSON() ([]byte, error)
```



### <a name="Bytes.UnmarshalJSON">func</a> (\*Bytes) [UnmarshalJSON](/src/target/bytes.go?s=777:825#L22)
``` go
func (b *Bytes) UnmarshalJSON(data []byte) error
```



## <a name="JSONMapper">type</a> [JSONMapper](/src/target/json.go?s=80:178#L1)
``` go
type JSONMapper struct {
    // contains filtered or unexported fields
}
```









### <a name="JSONMapper.FromJSON">func</a> (\*JSONMapper) [FromJSON](/src/target/json.go?s=1202:1265#L41)
``` go
func (m *JSONMapper) FromJSON(data []byte) (interface{}, error)
```
FromJSON will deserialize the output of ToJSON for every registered
implementation of the interface




### <a name="JSONMapper.ToJSON">func</a> (\*JSONMapper) [ToJSON](/src/target/json.go?s=1814:1875#L67)
``` go
func (m *JSONMapper) ToJSON(data interface{}) ([]byte, error)
```
ToJson will serialize a registered implementation into a format like:


	{
	  "type": "foo",
	  "data": {
	    "name": "dings"
	  }
	}

this allows us to properly deserialize with FromJSON




## <a name="Mapper">type</a> [Mapper](/src/target/wrapper.go?s=485:535#L5)
``` go
type Mapper struct {
    *JSONMapper
    // contains filtered or unexported fields
}
```
Mapper is the main entry point in the package.

On init, you should call NewMapper() for each interface type you want
to support flexible de-serialization, and then
RegisterInterface() in the init() function for each implementation of these
interfaces.

Note that unlike go-wire, you can call RegisterInterface separately from
different locations with each implementation, not all in one place.
Just be careful not to use the same key or byte, of init will *panic*







### <a name="NewMapper">func</a> [NewMapper](/src/target/wrapper.go?s=747:786#L17)
``` go
func NewMapper(base interface{}) Mapper
```
NewMapper creates a Mapper.

If you have:


	type Foo interface {....}
	type FooS struct { Foo }

then you should pass in FooS{} in NewMapper, and implementations of Foo
in RegisterInterface





### <a name="Mapper.RegisterInterface">func</a> (Mapper) [RegisterInterface](/src/target/wrapper.go?s=1184:1263#L30)
``` go
func (m Mapper) RegisterInterface(kind string, b byte, data interface{}) Mapper
```
RegisterInterface should be called once for each implementation of the
interface that we wish to support.

kind is the type string used in the json representation, while b is the
type byte used in the go-wire representation. data is one instance of this
concrete type, like Bar{}








- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
