math = require("math")

module.exports = {
	decode_string: decode_string,
	decode_varint: decode_varint,
	decode_big_endian: decode_big_endian,
	encode_big_endian: encode_big_endian,
	encode: encode,
	reverse: reverse,
}

function reverse(buf){
	for (var i = 0; i < buf.length/2; i++){
		a = buf[i];
		b = buf[buf.length-1 - i];
		buf[i] = b;
		buf[buf.length-1 - i] = a;
	}
	return buf
}

function uvarint_size(i){
	if (i == 0){
		return 0
	}

	for(var j = 1; j < 9; j++) {
		if ( i < 1<<j*8 ) {
			return j
		}
	}

	return 8
}

function encode_big_endian(i, size){
	if (size == 0){
		return new Buffer(0);
	}
	b = encode_big_endian(math.floor(i/256), size-1);
	return Buffer.concat([b, new Buffer([i%256])]);
}

function decode_big_endian(reader, size){
	if (size == 0){ return 0 }
	firstByte = reader.read(1)[0];
	return firstByte*(math.pow(256, size-1)) + decode_big_endian(reader, size-1)
}

function encode_string(s){
	size = encode_varint(s.length);
	return Buffer.concat([size, new Buffer(s)])
}

function decode_string(reader){
	length = decode_varint(reader);
	return reader.read(length).toString()
}

function encode_varint(i){
	var negate = false;
	if (i < 0){
		negate = true;
		i = -i;
	}
	size = uvarint_size(i);
	if (size == 0){
		return new Buffer([0])
	}

	big_end = encode_big_endian(i, size);
	if (negate){ size += 0xF0 }
	var buf = new Buffer([1]);
	return Buffer.concat([buf, big_end])
}

function decode_varint(reader){
	size = reader.read(1)[0];
	if (size == 0 ){
		return 0
	}
	var negate = false;
	if (size > 0xF0){ negate = true }
	if (negate) { size = size - 0xF0 }
	i = decode_big_endian(reader, size);
	if (negate) { i = i * -1}
	return i
}

function encode_list(l){
	var l2 = l.map(encode);
	var buf = new Buffer(encode_varint(l2.length));
	return Buffer.concat([buf, Buffer.concat(l2)]);
}

function encode(b){
	if (b == null){
		return Buffer(0)
	} else if (typeof b == "number"){
		return encode_varint(b)
	} else if (typeof b == "string"){
		return encode_string(b)
	} else if (Array.isArray(b)){
		return encode_list(b)
	} else{
		console.log("UNSUPPORTED TYPE!", typeof b, b)
	}
}





