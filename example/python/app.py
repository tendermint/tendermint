
import sys
sys.path.insert(0, './tmsp')

from wire import *
from server import *


# tmsp application interface

class CounterApplication():
    def __init__(self):
        self.hashCount = 0
	self.txCount = 0
	self.commitCount = 0

    def open(self):
	    return CounterAppContext(self)

class CounterAppContext():
	def __init__(self, app):
	    self.app = app
	    self.hashCount = app.hashCount
	    self.txCount = app.txCount
	    self.commitCount = app.commitCount
	    self.serial = False
	    
	def echo(self, msg):
		return msg, 0

	def info(self):
		return ["hash, tx, commit counts:%d, %d, %d"%(self.hashCount, self.txCount, self.commitCount)], 0

	def set_option(self, key, value):
		if key == "serial" and value == "on":
			self.serial = True
		return 0

	def append_tx(self, txBytes):
		if self.serial:
			txByteArray = bytearray(txBytes)
			if len(txBytes) >= 2 and txBytes[:2] == "0x":
				txByteArray = hex2bytes(txBytes[2:])
			txValue = decode_big_endian(BytesBuffer(txByteArray), len(txBytes))
			if txValue != self.txCount:
				return None, 1
		self.txCount += 1
		return None, 0

	def get_hash(self):
		self.hashCount += 1
		if self.txCount == 0:
			return "", 0
		h = encode_big_endian(self.txCount, 8)
		h.reverse()
		return str(h), 0

	def commit(self):
		return 0 

	def rollback(self):
		return 0

	def add_listener(self):
		return 0

	def rm_listener(self):
		return 0

	def event(self):
		return 

 
if __name__ == '__main__':
    l = len(sys.argv)
    if l == 1:
       port = 46658
    elif l == 2: 
        port = int(sys.argv[1])
    else:
        print "too many arguments"
        quit()

    print 'TMSP Demo APP (Python)'

    app = CounterApplication()	
    server = TMSPServer(app, port)
    server.main_loop()
