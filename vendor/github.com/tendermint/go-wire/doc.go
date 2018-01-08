/*
go-wire is our custom codec package for serializing and deserializing
data and structures as binary and JSON blobs.

In order to get started with go-wire we need to:

1) Choose the receiving structure for deserializing.
It MUST be an interface{} and during registration it MUST be
wrapped as a struct for example

    struct { Receiver }{}

2) Decide the IDs for the respective types that we'll be dealing with.
We shall call these the concrete types.

3) Register the receiving structure as well as each of the concrete types.
Typically do this in the init function so that it gets run before other functions are invoked
    

  func init() {
    wire.RegisterInterface(
	struct { Receiver }{},
	wire.ConcreteType{&bcMessage{}, 0x01},
	wire.ConcreteType{&bcResponse{}, 0x02},
	wire.ConcreteType{&bcStatus{}, 0x03},
    )
  }

  type bcMessage struct {
    Content string
    Height int
  }

  type bcResponse struct {
    Status int
    Message string 
  }

  type bcResponse struct {
    Status int
    Message string 
  }


Encoding to binary is performed by invoking wire.WriteBinary. You'll need to provide
the data to be encoded/serialized as well as where to store it and storage for
the number of bytes written as well as any error encountered

  var n int
  var err error
  buf := new(bytes.Buffer)
  bm := &bcMessage{Message: "Tendermint", Height: 100}
  wire.WriteBinary(bm, buf, &n, &err)

Decoding from binary is performed by invoking wire.ReadBinary. The data being decoded
has to be retrieved from the decoding receiver that we previously defined i.e. Receiver
for example

  recv := wire.ReadBinary(struct{ Receiver }{}, buf, 0, &n, &err).(struct{ Receiver }).Receiver
  decoded := recv.(*bcMessage)
  fmt.Printf("Decoded: %#v\n", decoded)

Note that in the decoding example we used

  struct { Receiver }{} --> .(struct{ Receiver }).Receiver

to receive the value. That correlates with the type that we registered in wire.RegisterInterface
*/
package wire
