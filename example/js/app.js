server = require("./server")
wire = require("./wire")
util = require("util")

function CounterApp(){
	this.hashCount = 0;
	this.txCount = 0;
  this.serial = false;
};

CounterApp.prototype.echo = function(msg){
	return {"response": msg, "ret_code":0}
}

CounterApp.prototype.info = function(){
	return {"response": [util.format("hashes:%d, txs:%d", this.hashCount, this.txCount)]}
}

CounterApp.prototype.set_option = function(key, value){
	if (key == "serial" && value == "on"){
		this.serial = true;
	}
	return {"ret_code":0}
}

CounterApp.prototype.append_tx = function(txBytes){
	if (this.serial) {
		txByteArray = new Buffer(txBytes)
		if (txBytes.length >= 2 && txBytes.slice(0, 2) == "0x") {
			txByteArray = wire.hex2bytes(txBytes.slice(2));
		}	
		r = new msg.buffer(txByteArray)
		txValue = wire.decode_big_endian(r, txBytes.length)
		if (txValue != this.txCount){
			return {"ret_code":6}
		}
	}
	this.txCount += 1;
	return {"ret_code":0} // TODO: return events
}

CounterApp.prototype.check_tx = function(txBytes){
	if (this.serial) {
		txByteArray = new Buffer(txBytes)
		if (txBytes.length >= 2 && txBytes.slice(0, 2) == "0x") {
			txByteArray = wire.hex2bytes(txBytes.slice(2));
		}	
		r = new msg.buffer(txByteArray)
		txValue = wire.decode_big_endian(r, txBytes.length)
		if (txValue < this.txCount){
			return {"ret_code":6}
		}
	}
	return {"ret_code":0}
}

CounterApp.prototype.get_hash = function(){
	this.hashCount += 1;
	if (this.txCount == 0){
		return {"response": "", "ret_code":0}
	}
	h = wire.encode_big_endian(this.txCount, 8);
	h = wire.reverse(h); // TODO
	return {"response": h.toString(), "ret_code":0}
}

CounterApp.prototype.add_listener = function(){
	return {"ret_code":0}
}

CounterApp.prototype.rm_listener = function(){
	return {"ret_code":0}
}

CounterApp.prototype.event = function(){
}

console.log("Counter app in Javascript")

var app = new CounterApp();
var appServer = new server.AppServer(app);
appServer.server.listen(46658)
