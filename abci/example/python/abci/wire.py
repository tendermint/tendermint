
# the decoder works off a reader
# the encoder returns bytearray


def hex2bytes(h):
    return bytearray(h.decode('hex'))


def bytes2hex(b):
    if type(b) in (str, unicode):
        return "".join([hex(ord(c))[2:].zfill(2) for c in b])
    else:
        return bytes2hex(b.decode())


# expects uvarint64 (no crazy big nums!)
def uvarint_size(i):
    if i == 0:
        return 0
    for j in xrange(1, 8):
        if i < 1 << j * 8:
            return j
    return 8

# expects i < 2**size


def encode_big_endian(i, size):
    if size == 0:
        return bytearray()
    return encode_big_endian(i / 256, size - 1) + bytearray([i % 256])


def decode_big_endian(reader, size):
    if size == 0:
        return 0
    firstByte = reader.read(1)[0]
    return firstByte * (256 ** (size - 1)) + decode_big_endian(reader, size - 1)

# ints are max 16 bytes long


def encode_varint(i):
    negate = False
    if i < 0:
        negate = True
        i = -i
    size = uvarint_size(i)
    if size == 0:
        return bytearray([0])
    big_end = encode_big_endian(i, size)
    if negate:
        size += 0xF0
    return bytearray([size]) + big_end

# returns the int and whats left of the byte array


def decode_varint(reader):
    size = reader.read(1)[0]
    if size == 0:
        return 0

    negate = True if size > int(0xF0) else False
    if negate:
        size = size - 0xF0
    i = decode_big_endian(reader, size)
    if negate:
        i = i * (-1)
    return i


def encode_string(s):
    size = encode_varint(len(s))
    return size + bytearray(s)


def decode_string(reader):
    length = decode_varint(reader)
    return str(reader.read(length))


def encode_list(s):
    b = bytearray()
    map(b.extend, map(encode, s))
    return encode_varint(len(s)) + b


def encode(s):
    if s is None:
        return bytearray()
    if isinstance(s, int):
        return encode_varint(s)
    elif isinstance(s, str):
        return encode_string(s)
    elif isinstance(s, list):
        return encode_list(s)
    else:
        print "UNSUPPORTED TYPE!", type(s), s


if __name__ == '__main__':
    ns = [100, 100, 1000, 256]
    ss = [2, 5, 5, 2]
    bs = map(encode_big_endian, ns, ss)
    ds = map(decode_big_endian, bs, ss)
    print ns
    print [i[0] for i in ds]

    ss = ["abc", "hi there jim", "ok now what"]
    e = map(encode_string, ss)
    d = map(decode_string, e)
    print ss
    print [i[0] for i in d]
