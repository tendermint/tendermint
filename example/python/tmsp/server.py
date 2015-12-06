import socket
import select
import sys
import os


from wire import *
from reader import *
from msg import *

# TMSP server responds to messges by calling methods on the app
class TMSPServer():
    def __init__(self, app, port=5410):
	self.app = app
	self.appMap = {} # map conn file descriptors to (appContext, msgDecoder)

        self.port = port 
        self.listen_backlog = 10

        self.listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self.listener.setblocking(0)
        self.listener.bind(('', port))

        self.listener.listen(self.listen_backlog)

        self.shutdown = False

        self.read_list = [self.listener]
        self.write_list = []

    def handle_new_connection(self, r):
        new_fd, new_addr = r.accept()
        self.read_list.append(new_fd)
        self.write_list.append(new_fd)
        print 'new connection to', new_addr

	appContext = self.app.open()
	self.appMap[new_fd] = (appContext, RequestDecoder(ConnReader(new_fd)))

    def handle_conn_closed(self, r):
        self.read_list.remove(r)
        self.write_list.remove(r)
        r.close()
        print "connection closed"

    def handle_recv(self, r):
	appCtx, conn = self.appMap[r]
	response = bytearray()
	while True:
		try:
		    # first read the request type and get the msg decoder
		    typeByte = conn.reader.read(1)
		    typeByte = int(typeByte[0])
		    resTypeByte = typeByte+0x10
		    req_type = message_types[typeByte]

		    if req_type == "flush":
			    response += bytearray([resTypeByte]) 
			    sent = r.send(str(response))
			    return

		    decoder = getattr(conn, req_type)

		    req_args = decoder()
		    req_f = getattr(appCtx, req_type)
		    if req_args == None:
			res = req_f()
		    elif isinstance(req_args, tuple):
			res = req_f(*req_args)
		    else:
			res = req_f(req_args)

		    if isinstance(res, tuple):
	 	    	res, ret_code = res
		    else:
		    	ret_code = res
			res = None

		    if ret_code != 0:
			    print "non-zero retcode:", ret_code
			    return

		    if req_type in ("echo", "info"): # these dont return a ret code
		    	response += bytearray([resTypeByte]) + encode(res)
		    else:
		    	response += bytearray([resTypeByte]) + encode(ret_code) + encode(res)
		except TypeError as e: 
			print "TypeError on reading from connection:", e
			self.handle_conn_closed(r)
			return
		except ValueError as e: 
			print "ValueError on reading from connection:", e
			self.handle_conn_closed(r)
			return
		except IOError as e: 
			print "IOError on reading from connection:", e
			self.handle_conn_closed(r)
			return
		except:
			print "error reading from connection", sys.exc_info()[0] # TODO better
			self.handle_conn_closed(r)
			return

    def main_loop(self):
        while not self.shutdown:
            r_list, w_list, _ = select.select(self.read_list, self.write_list, [], 2.5)

            for r in r_list:
                if (r == self.listener):  
                    try:
                        self.handle_new_connection(r)

			# undo adding to read list ...
                    except NameError as e: 
			print "Could not connect due to NameError:", e
                    except TypeError as e: 
			print "Could not connect due to TypeError:", e
		    except:
			print "Could not connect due to unexpected error:", sys.exc_info()[0]
                else: 
                    self.handle_recv(r)



    def handle_shutdown(self):
        for r in self.read_list:
            r.close()
        for w in self.write_list:
            try:
                w.close()
            except: pass
        self.shutdown = True

