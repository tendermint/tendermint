var net = require("net");
var wire = require("js-wire");
var msg = require("./msgs");
var types = require("./types");

var maxWriteBufferLength = 4096; // Any more and flush

// Takes an application and handles TMSP connection
// which invoke methods on the app
function AppServer(app){
  // set the app for the socket handler
  this.app = app;

  // create a server by providing callback for 
  // accepting new connection and callbacks for 
  // connection events ('data', 'end', etc.)
  this.createServer();
}

AppServer.prototype.createServer = function() {
  var app = this.app;

  // Define the socket handler
  this.server = net.createServer(function(socket) {
    socket.name = socket.remoteAddress + ":" + socket.remotePort;
    console.log("new connection from", socket.name);

    var conn = new Connection(socket, function(msgBytes, cb) {
      var r = new wire.Reader(msgBytes);

      // Now we can decode
      var typeByte = r.readByte();
      var reqType = msg.types[typeByte];

      // Special messages.
      // NOTE: msgs are length prefixed
      if (reqType == "flush") {
        var w = new wire.Writer();
        w.writeByte(types.ResponseTypeFlush);
        conn.writeMessage(w.getBuffer());
        conn.flush();
        return cb();
      } else if (reqType == "echo") {
        var message = r.readString();
        var w = new wire.Writer();
        w.writeByte(types.ResponseTypeEcho);
        w.writeString(message);
        conn.writeMessage(w.getBuffer());
        return cb();
      }

      // Make callback by wrapping cp
      var resCb = msg.writerGenerators[reqType](new wire.Writer(), function(w) {
        conn.writeMessage(w.getBuffer());
        return cb();
      });

      // Decode arguments
      var args = msg.readers[reqType](r);
      args.unshift(resCb);

      // Call function
      var res = app[reqType].apply(app, args);
      if (res != undefined) {
        console.log("Message handler shouldn't return anything!");
      }

    });
  });
}

//----------------------------------------

function Connection(socket, msgCb) {
  this.socket = socket;
  this.recvBuf = new Buffer(0);
  this.sendBuf = new Buffer(0);
  this.msgCb = msgCb;
  this.waitingResult = false;
  var conn = this;

  // Handle TMSP requests.
  socket.on('data', function(data) {
    conn.appendData(data);
  });
  socket.on('end', function() {
    console.log("connection ended");
  });
}

Connection.prototype.appendData = function(bytes) {
  var conn = this;
  if (bytes.length > 0) {
    this.recvBuf = Buffer.concat([this.recvBuf, new Buffer(bytes)]);
  }
  if (this.waitingResult) {
    return;
  }
  var r = new wire.Reader(this.recvBuf);
  var msg;
  try {
    msg = r.readByteArray();
  } catch(e) {
    return;
  }
  this.recvBuf = r.buf.slice(r.offset);
  this.waitingResult = true;
  this.socket.pause();
  //try {
    this.msgCb(msg, function() {
      // This gets called after msg handler is finished with response.
      conn.waitingResult = false;
      conn.socket.resume();
      if (conn.recvBuf.length > 0) {
        conn.appendData("");
      }
    });
  //} catch(e) {
  //  console.log("FATAL ERROR: ", e);
  //}
};

Connection.prototype.writeMessage = function(msgBytes) {
  var msgLength = wire.uvarintSize(msgBytes.length);
  var buf = new Buffer(1+msgLength+msgBytes.length);
  var w = new wire.Writer(buf);
  w.writeByteArray(msgBytes); // TODO technically should be writeVarint
  this.sendBuf = Buffer.concat([this.sendBuf, w.getBuffer()]);
  if (this.sendBuf.length >= maxWriteBufferLength) {
    this.flush();
  }
};

Connection.prototype.flush = function() {
  var n = this.socket.write(this.sendBuf);
  this.sendBuf = new Buffer(0);
}

//----------------------------------------

module.exports = { AppServer: AppServer };
