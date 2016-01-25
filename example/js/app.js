var server = require("./server");
var wire = require("js-wire");
var util = require("util");
var msg = require("./msgs");
var types = require("./types");


function CounterApp(){
	this.hashCount = 0;
	this.txCount = 0;
  this.serial = false;
};

CounterApp.prototype.info = function(cb) {
  return cb(util.format("hashes:%d, txs:%d", this.hashCount, this.txCount));
}

CounterApp.prototype.set_option = function(cb, key, value) {
	if (key == "serial" && value == "on") {
		this.serial = true;
	}
  return cb("");
}

CounterApp.prototype.append_tx = function(cb, txBytes) {
	if (this.serial) {
		if (txBytes.length >= 2 && txBytes.slice(0, 2) == "0x") {
      var hexString = txBytes.toString("ascii", 2);
      var hexBytes = new Buffer(hexString, "hex");
      txBytes = hexBytes;
		}	
    var txValue = txBytes.readIntBE(0, txBytes.length);
		if (txValue != this.txCount){
      return cb(types.RetCodeInvalidNonce, "", "Nonce is invalid");
		}
	}
	this.txCount += 1;
	return cb(types.RetCodeOK, "", "");
}

CounterApp.prototype.check_tx = function(cb, txBytes) {
	if (this.serial) {
		if (txBytes.length >= 2 && txBytes.slice(0, 2) == "0x") {
      var hexString = txBytes.toString("ascii", 2);
      var hexBytes = new Buffer(hexString, "hex");
      txBytes = hexBytes;
		}	
    var txValue = txBytes.readIntBE(0, txBytes.length);
		if (txValue < this.txCount){
      return cb(types.RetCodeInvalidNonce, "", "Nonce is too low");
		}
	}
	this.txCount += 1;
	return cb(types.RetCodeOK, "", "");
}

CounterApp.prototype.get_hash = function(cb) {
	this.hashCount += 1;
	if (this.txCount == 0){
    return cb("", "Zero tx count; hash is empth");
	}
  var buf = new Buffer(8);
  buf.writeIntBE(this.txCount, 0, 8);
  cb(buf, "");
}

CounterApp.prototype.query = function(cb) {
  return cb("", "Query not yet supporrted");
}

console.log("Counter app in Javascript");

var app = new CounterApp();
var appServer = new server.AppServer(app);
appServer.server.listen(46658);
