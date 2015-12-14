
# Simple read() method around a bytearray
class BytesReader():
	def __init__(self, b):
		self.buf = b

	def read(self, n):
		if len(self.buf) < n:
			print "reader err: buf less than n"
			# TODO: exception
			return
		r = self.buf[:n]
		self.buf = self.buf[n:]
		return r

# Buffer bytes off a tcp connection and read them off in chunks
class ConnReader():
	def __init__(self, conn):
		self.conn = conn
		self.buf = bytearray()

	# blocking 
	def read(self, n):
		while n > len(self.buf):
			moreBuf = self.conn.recv(1024)
			if not moreBuf:
				raise IOError("dead connection")
			self.buf = self.buf + bytearray(moreBuf)

		r = self.buf[:n]
		self.buf = self.buf[n:]
		return r
