var wire = require("js-wire");
var types = require("./types");

var readRequestInfo =      function(r) { return []; };
var readRequestSetOption = function(r) { return [r.readString(), r.readString()]; };
var readRequestAppendTx =  function(r) { return [r.readByteArray()]; };
var readRequestCheckTx =   function(r) { return [r.readByteArray()]; };
var readRequestGetHash =   function(r) { return []; };
var readRequestQuery =     function(r) { return [r.readByteArray()]; };

var runOnce = function(name, f) {
  var ran = false;
  return function() {
    if (ran) {
      console.log("Error: response was already written for "+name);
      return
    } else {
      ran = true;
    }
    return f.apply(this, arguments);
  };
};

var makeWriteResponseInfo = function(w, cb) { return runOnce("info", function(info) {
  w.writeUint8(types.ResponseTypeInfo);
  w.writeString(info);
  cb(w);
});};
var makeWriteResponseSetOption = function(w, cb) { return runOnce("set_option", function(log) {
  w.writeUint8(types.ResponseTypeSetOption);
  w.writeString(log);
  cb(w);
});};
var makeWriteResponseAppendTx = function(w, cb) { return runOnce("append_tx", function(code, result, log) {
  w.writeUint8(types.ResponseTypeAppendTx);
  w.writeUint8(code);
  w.writeByteArray(result);
  w.writeString(log);
  cb(w);
});};
var makeWriteResponseCheckTx = function(w, cb) { return runOnce("check_tx", function(code, result, log) {
  w.writeUint8(types.ResponseTypeCheckTx);
  w.writeUint8(code);
  w.writeByteArray(result);
  w.writeString(log);
  cb(w);
});};
var makeWriteResponseGetHash = function(w, cb) { return runOnce("get_hash", function(hash, log) {
  w.writeUint8(types.ResponseTypeGetHash);
  w.writeByteArray(hash);
  w.writeString(log);
  cb(w);
});};
var makeWriteResponseQuery = function(w, cb) { return runOnce("query", function(result, log) {
  w.writeUint8(types.ResponseTypeQuery);
  w.writeByteArray(result);
  w.writeString(log);
  cb(w);
});};

module.exports = {
  types : {
    0x01 : "echo",
    0x02 : "flush",
    0x03 : "info",
    0x04 : "set_option",
    0x21 : "append_tx",
    0x22 : "check_tx",
    0x23 : "get_hash",
    0x24 : "query",
  },
  readers : {
    "info":       readRequestInfo,
    "set_option": readRequestSetOption,
    "append_tx":  readRequestAppendTx,
    "check_tx":   readRequestCheckTx,
    "get_hash":   readRequestGetHash,
    "query":      readRequestQuery,
  },
  writerGenerators: {
    "info":       makeWriteResponseInfo,
    "set_option": makeWriteResponseSetOption,
    "append_tx":  makeWriteResponseAppendTx,
    "check_tx":   makeWriteResponseCheckTx,
    "get_hash":   makeWriteResponseGetHash,
    "query":      makeWriteResponseQuery,
  },
};
