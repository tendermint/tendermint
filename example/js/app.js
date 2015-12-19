server = require("./server")
wire = require("./wire")
util = require("util")

function CounterApp(){
	this.hashCount = 0;
	this.txCount = 0;
	this.commitCount = 0;
};

CounterApp.prototype.open = function(){
	return new CounterAppContext(this);
}

function CounterAppContext(app) {
	this.hashCount = app.hashCount;
	this.txCount = app.txCount;
	this.commitCount = app.commitCount;
	this.serial = false;
}

CounterAppContext.prototype.echo = function(msg){
	return {"response": msg, "ret_code":0}
}

CounterAppContext.prototype.info = function(){
	return {"response": [util.format("hash, tx, commit counts: %d, %d, %d", this.hashCount, this.txCount, this.commitCount)]}
}

CounterAppContext.prototype.set_option = function(key, value){
	if (key == "serial" && value == "on"){
		this.serial = true;
	}
	return {"ret_code":0}
}

CounterAppContext.prototype.append_tx = function(txBytes){
	if (this.serial) {
		txByteArray = txBytes
		if (txByte.length >= 2 && txBytes.slice(0, 2) == "0x") {
			txByteArray = wire.hex2bytes(txBytes.slice(2));
		}	
		r = new wire.BytesReader(txByteArray)
		txValue = decode_big_endian(r, txBytes.length)
		if (txValue != this.txcount){
			return {"ret_code":1}
		}
	}
	this.txCount += 1;
	return {"ret_code":0} // TODO: return events
}

CounterAppContext.prototype.get_hash = function(){
	this.hashCount += 1;
	if (this.txCount == 0){
		return {"response": "", "ret_code":0}
	}
	h = wire.encode_big_endian(this.txCount, 8);
	h = wire.reverse(h); // TODO
	return {"response": h.toString(), "ret_code":0}
}

CounterAppContext.prototype.commit = function(){
	this.commitCount += 1;
	return {"ret_code":0}
}

CounterAppContext.prototype.rollback = function(){
	return {"ret_code":0}
}

CounterAppContext.prototype.add_listener = function(){
	return {"ret_code":0}
}

CounterAppContext.prototype.rm_listener = function(){
	return {"ret_code":0}
}

CounterAppContext.prototype.event = function(){
}

console.log("Counter app in Javascript")

var app = new CounterApp();
var appServer = new server.AppServer(app);
appServer.server.listen(46658)
