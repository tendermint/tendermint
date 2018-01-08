/*
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

* http://eagain.net/articles/go-dynamic-json/
* http://eagain.net/articles/go-json-kind/

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

See https://github.com/tendermint/go-wire/data/blob/master/common_test.go to see
how to set up your code to use this.
*/
package data
