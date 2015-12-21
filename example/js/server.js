
// Load the TCP Library
net = require('net');
msg = require('./msgs');
wire = require("./wire")

// Takes an application and handles tmsp connection
// which invoke methods on the app
function AppServer(app){
	// set the app for the socket handler
	this.app = app;

	// create a server by providing callback for 
	// accepting new connection and callbacks for 
	// connection events ('data', 'end', etc.)
	this.createServer()
}

module.exports = { AppServer: AppServer };

AppServer.prototype.createServer = function(){
	app = this.app
	conns = {} // map sockets to their state

	// define the socket handler
	this.server = net.createServer(function(socket){
		  socket.name = socket.remoteAddress + ":" + socket.remotePort 
		  console.log("new connection from", socket.name)

		  appCtx = app.open()

		  var conn = { 
			  recBuf: new msg.buffer(new Buffer(0)),
			  resBuf: new msg.buffer(new Buffer(0)),
			  msgLength: 0,
			  inProgress: false
		  }
		  conns[socket] = conn

		  // Handle tmsp requests.
		  socket.on('data', function (data) {

		      if (data.length == 0){
		      	// TODO err
		      	console.log("empty data!")
		      	return
		      }
		      conn = conns[socket]

		      // we received data. append it
		      conn.recBuf.write(data)

		      while ( conn.recBuf.size() > 0 ){

			if (conn.msgLength == 0){
				ll = conn.recBuf.peek();
				if (conn.recBuf.size() < 1 + ll){
					// don't have enough bytes to read length yet
					return
				}
				conn.msgLength = wire.decode_varint(conn.recBuf)
			}

			if (conn.recBuf.size() < conn.msgLength) {
				// don't have enough to decode the message
				return
			}

			// now we can decode
			typeByte = conn.recBuf.read(1);
			resTypeByte = typeByte[0] + 0x10 
			reqType = msg.types[typeByte[0]];

			if (reqType == "flush"){
				// msgs are length prefixed
				conn.resBuf.write(wire.encode(1));
				conn.resBuf.write(new Buffer([resTypeByte]))
				n = socket.write(conn.resBuf.buf);
				conn.msgLength = 0;
				conn.resBuf = new msg.buffer(new Buffer(0));
				return
			}

			// decode args
			decoder = new msg.decoder(conn.recBuf);
			args = decoder[reqType]();

			// done decoding
			conn.msgLength = 0

			var res = function(){
				if (args == null){
					return appCtx[reqType]();
				} else if (Array.isArray(args)){
					return appCtx[reqType].apply(appCtx, args);
				} else {
					return appCtx[reqType](args)
				}
			}()


			var retCode = res["ret_code"]
			var res = res["response"]

			if (retCode != null && retCode != 0){
				console.log("non-zero ret code", retCode)
			}


			if (reqType == "echo" || reqType == "info"){
				enc = Buffer.concat([new Buffer([resTypeByte]), wire.encode(res)]);
				// length prefixed
				conn.resBuf.write(wire.encode(enc.length));
				conn.resBuf.write(enc);
			} else {
				enc = Buffer.concat([new Buffer([resTypeByte]), wire.encode(retCode), wire.encode(res)]);
				conn.resBuf.write(wire.encode(enc.length));
				conn.resBuf.write(enc);
			}
		    }
		  });

		  socket.on('end', function () {
		    console.log("connection ended")
		  });
		})
}

