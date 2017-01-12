import socket
import select
import sys

from wire import decode_varint, encode
from reader import BytesBuffer
from msg import RequestDecoder, message_types

# hold the asyncronous state of a connection
# ie. we may not get enough bytes on one read to decode the message

class Connection():

    def __init__(self, fd, app):
        self.fd = fd
        self.app = app
        self.recBuf = BytesBuffer(bytearray())
        self.resBuf = BytesBuffer(bytearray())
        self.msgLength = 0
        self.decoder = RequestDecoder(self.recBuf)
        self.inProgress = False  # are we in the middle of a message

    def recv(this):
        data = this.fd.recv(1024)
        if not data:  # what about len(data) == 0
            raise IOError("dead connection")
        this.recBuf.write(data)

# ABCI server responds to messges by calling methods on the app

class ABCIServer():

    def __init__(self, app, port=5410):
        self.app = app
        # map conn file descriptors to (app, reqBuf, resBuf, msgDecoder)
        self.appMap = {}

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
        new_fd.setblocking(0)  # non-blocking
        self.read_list.append(new_fd)
        self.write_list.append(new_fd)
        print 'new connection to', new_addr

        self.appMap[new_fd] = Connection(new_fd, self.app)

    def handle_conn_closed(self, r):
        self.read_list.remove(r)
        self.write_list.remove(r)
        r.close()
        print "connection closed"

    def handle_recv(self, r):
        #  app, recBuf, resBuf, conn
        conn = self.appMap[r]
        while True:
            try:
                print "recv loop"
                # check if we need more data first
                if conn.inProgress:
                    if (conn.msgLength == 0 or conn.recBuf.size() < conn.msgLength):
                        conn.recv()
                else:
                    if conn.recBuf.size() == 0:
                        conn.recv()

                conn.inProgress = True

                # see if we have enough to get the message length
                if conn.msgLength == 0:
                    ll = conn.recBuf.peek()
                    if conn.recBuf.size() < 1 + ll:
                        # we don't have enough bytes to read the length yet
                        return
                    print "decoding msg length"
                    conn.msgLength = decode_varint(conn.recBuf)

                # see if we have enough to decode the message
                if conn.recBuf.size() < conn.msgLength:
                    return

                # now we can decode the message

                # first read the request type and get the particular msg
                # decoder
                typeByte = conn.recBuf.read(1)
                typeByte = int(typeByte[0])
                resTypeByte = typeByte + 0x10
                req_type = message_types[typeByte]

                if req_type == "flush":
                    # messages are length prefixed
                    conn.resBuf.write(encode(1))
                    conn.resBuf.write([resTypeByte])
                    conn.fd.send(str(conn.resBuf.buf))
                    conn.msgLength = 0
                    conn.inProgress = False
                    conn.resBuf = BytesBuffer(bytearray())
                    return

                decoder = getattr(conn.decoder, req_type)

                print "decoding args"
                req_args = decoder()
                print "got args", req_args

                # done decoding message
                conn.msgLength = 0
                conn.inProgress = False

                req_f = getattr(conn.app, req_type)
                if req_args is None:
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

                print "called", req_type, "ret code:", ret_code
                if ret_code != 0:
                    print "non-zero retcode:", ret_code

                if req_type in ("echo", "info"):  # these dont return a ret code
                    enc = encode(res)
                    # messages are length prefixed
                    conn.resBuf.write(encode(len(enc) + 1))
                    conn.resBuf.write([resTypeByte])
                    conn.resBuf.write(enc)
                else:
                    enc, encRet = encode(res), encode(ret_code)
                    # messages are length prefixed
                    conn.resBuf.write(encode(len(enc) + len(encRet) + 1))
                    conn.resBuf.write([resTypeByte])
                    conn.resBuf.write(encRet)
                    conn.resBuf.write(enc)
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
            except Exception as e:
                # sys.exc_info()[0] # TODO better
                print "error reading from connection", str(e)
                self.handle_conn_closed(r)
                return

    def main_loop(self):
        while not self.shutdown:
            r_list, w_list, _ = select.select(
                self.read_list, self.write_list, [], 2.5)

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
            except Exception as e:
                print(e)  # TODO: add logging
        self.shutdown = True
