# go-base58

I copied this package from https://github.com/jbenet/go-base58
which in turn came from https://github.com/conformal/btcutil
to provide a simple base58 package that
- defaults to base58-check (btc)
- and allows using different alphabets.
- and returns an error on decoding problems to be
  compatible with the `encoding/*` packages in stdlib

## Usage

```go
package main

import (
  "fmt"
  b58 "github.com/tendermint/go-wire/data/base58"
)

func main() {
  buf := []byte{255, 254, 253, 252}
  fmt.Printf("buffer: %v\n", buf)

  str := b58.Encode(buf)
  fmt.Printf("encoded: %s\n", str)

  buf2, err := b58.Decode(str)
  if err != nil {
    panic(err)
  }
  fmt.Printf("decoded: %v\n", buf2)
}
```

### Another alphabet

```go
package main

import (
  "fmt"
  b58 "github.com/tendermint/go-wire/data/base58"
)

const BogusAlphabet = "ZYXWVUTSRQPNMLKJHGFEDCBAzyxwvutsrqponmkjihgfedcba987654321"


func encdec(alphabet string) {
  fmt.Printf("using: %s\n", alphabet)

  buf := []byte{255, 254, 253, 252}
  fmt.Printf("buffer: %v\n", buf)

  str := b58.EncodeAlphabet(buf, alphabet)
  fmt.Printf("encoded: %s\n", str)

  buf2, err := b58.DecodeAlphabet(str, alphabet)
  if err != nil {
    panic(err)
  }
  fmt.Printf("decoded: %v\n\n", buf2)
}


func main() {
  encdec(b58.BTCAlphabet)
  encdec(b58.FlickrAlphabet)
  encdec(BogusAlphabet)
}
```


## License

Package base58 (and the original btcutil) are licensed under the ISC License.
