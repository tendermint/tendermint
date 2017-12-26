# Tendermint Encoding

## Serialization

Tendermint aims to encode data structures in a manner similar to how the corresponding Go structs are laid out in memory.
Variable length items are length-prefixed.
While the encoding was inspired by Go, it is easily implemented in other languages as well given its intuitive design.

### Fixed Length Integers

Fixed length integers are encoded in Big-Endian using the specified number of bytes.
So `uint8` and `int8` use one byte, `uint16` and `int16` use two bytes,
`uint32` and `int32` use 3 bytes, and `uint64` and `int64` use 4 bytes.

Negative integers are encoded via twos-complement.

Examples:

```
encode(uint8(6))    == [0x06]
encode(uint32(6))   == [0x00, 0x00, 0x00, 0x06]

encode(int8(-6))    == [0xFA]
encode(int32(-6))   == [0xFF, 0xFF, 0xFF, 0xFA]
```

### Variable Length Integers

Variable length integers are encoded as length-prefixed Big-Endian integers.
The length-prefix consists of a single byte and corresponds to the length of the encoded integer.

Negative integers are encoded by flipping the leading bit of the length-prefix to a `1`.


Examples:

```
encode(uint(6))     == [0x01, 0x06]
encode(uint(70000)) == [0x03, 0x01, 0x11, 0x70]

encode(int(-6))     == [0xF1, 0x06]
encode(int(-70000)) == [0xF3, 0x01, 0x11, 0x70]
```

### Strings

An encoded string is a length prefix followed by the underlying bytes of the string.
The length-prefix is itself encoded as an `int`.

Examples:

```
encode("a")     == [0x01, 0x01, 0x61]
encode("hello") == [0x01, 0x05, 0x68, 0x65, 0x6C, 0x6C, 0x6F]
encode("¥")     == [0x01, 0x02, 0xC2, 0xA5]
```

### Arrays (fixed length)

An encoded fix-lengthed array is the concatenation of the encoding of its elements.
There is no length-prefix.

Examples:

```
encode([4]int8{1, 2, 3, 4})     == [0x01, 0x02, 0x03, 0x04]
encode([4]int16{1, 2, 3, 4})    == [0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04]
encode([4]int{1, 2, 3, 4})      == [0x01, 0x01, 0x01, 0x02, 0x01, 0x03, 0x01, 0x04]
encode([2]string{"abc", "efg"}) == [0x01, 0x03, 0x61, 0x62, 0x63, 0x01, 0x03, 0x65, 0x66, 0x67]
```

### Slices (variable length)

An encoded variable-length array is a length prefix followed by the concatenation of the encoding of its elements.
The length-prefix is itself encoded as an `int`.

Examples:

```
encode([]int8{1, 2, 3, 4})      == [0x01, 0x04, 0x01, 0x02, 0x03, 0x04]
encode([]int16{1, 2, 3, 4})     == [0x01, 0x04, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04]
encode([]int{1, 2, 3, 4})       == [0x01, 0x04, 0x01, 0x01, 0x01, 0x02, 0x01, 0x03, 0x01, 0x4]
encode([]string{"abc", "efg"})  == [0x01, 0x02, 0x01, 0x03, 0x61, 0x62, 0x63, 0x01, 0x03, 0x65, 0x66, 0x67]
```

### Time

Time is encoded as an `int64` of the number of nanoseconds since January 1, 1970,
rounded to the nearest millisecond.

Times before then are invalid.

Examples:

```
encode(time.Time("Jan 1 00:00:00 UTC 1970"))            == [0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]
encode(time.Time("Jan 1 00:00:01 UTC 1970"))            == [0x00, 0x00, 0x00, 0x00, 0x3B, 0x9A, 0xCA, 0x00] // 1,000,000,000 ns
encode(time.Time("Mon Jan 2 15:04:05 -0700 MST 2006"))  == [0x0F, 0xC4, 0xBB, 0xC1, 0x53, 0x03, 0x12, 0x00]
```

### Structs

An encoded struct is the concatenation of the encoding of its elements.
There is no length-prefix.

Examples:

```
type MyStruct struct{
    A int
    B string
    C time.Time
}
encode(MyStruct{4, "hello", time.Time("Mon Jan 2 15:04:05 -0700 MST 2006")}) ==
    [0x01, 0x04, 0x01, 0x05, 0x68, 0x65, 0x6C, 0x6C, 0x6F, 0x0F, 0xC4, 0xBB, 0xC1, 0x53, 0x03, 0x12, 0x00]
```


## Merkle Trees

SimpleMerkleRoot

MakeBlockParts
