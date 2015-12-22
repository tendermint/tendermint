wire = require("./wire")

module.exports = {
	types : {
		0x01 : "echo",
		0x02 : "flush",
		0x03 : "info",
		0x04 : "set_option",
		0x21 : "append_tx",
		0x22 : "get_hash",
		0x23 : "commit",
		0x24 : "rollback",
		0x25 : "add_listener",
		0x26 : "rm_listener",
	},

	decoder : RequestDecoder,

	buffer: BytesBuffer

}

function RequestDecoder(buf){
	this.buf= buf
}

var decode_string = wire.decode_string

// return nothing, one thing, or a list of things
RequestDecoder.prototype.echo = function(){ return decode_string(this.buf) };
RequestDecoder.prototype.flush = function(){};
RequestDecoder.prototype.info = function(){};
RequestDecoder.prototype.set_option = function(){ return [decode_string(this.buf), decode_string(this.buf)] };
RequestDecoder.prototype.append_tx = function(){ return decode_string(this.buf)};
RequestDecoder.prototype.get_hash = function(){ };
RequestDecoder.prototype.commit = function(){ };
RequestDecoder.prototype.rollback = function(){ };
RequestDecoder.prototype.add_listener = function(){ }; // TODO
RequestDecoder.prototype.rm_listener = function(){ }; // TODO

// buffered reader with read(n) method
function BytesBuffer(buf){
	this.buf = buf
}

BytesBuffer.prototype.read = function(n){
	b = this.buf.slice(0, n)
	this.buf = this.buf.slice(n)
	return b
};

BytesBuffer.prototype.write = function(buf){
	this.buf = Buffer.concat([this.buf, buf]);
};


BytesBuffer.prototype.size = function(){
	return this.buf.length
}

BytesBuffer.prototype.peek = function(){
	return this.buf[0]
}

