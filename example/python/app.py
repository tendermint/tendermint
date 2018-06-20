import sys

from abci.wire import hex2bytes, decode_big_endian, encode_big_endian
from abci.server import ABCIServer
from abci.reader import BytesBuffer


class CounterApplication():

    def __init__(self):
        sys.exit("The python example is out of date.  Upgrading the Python examples is currently left as an exercise to you.")
        self.hashCount = 0
        self.txCount = 0
        self.serial = False

    def echo(self, msg):
        return msg, 0

    def info(self):
        return ["hashes:%d, txs:%d" % (self.hashCount, self.txCount)], 0

    def set_option(self, key, value):
        if key == "serial" and value == "on":
            self.serial = True
        return 0

    def deliver_tx(self, txBytes):
        if self.serial:
            txByteArray = bytearray(txBytes)
            if len(txBytes) >= 2 and txBytes[:2] == "0x":
                txByteArray = hex2bytes(txBytes[2:])
            txValue = decode_big_endian(
                BytesBuffer(txByteArray), len(txBytes))
            if txValue != self.txCount:
                return None, 6
        self.txCount += 1
        return None, 0

    def check_tx(self, txBytes):
        if self.serial:
            txByteArray = bytearray(txBytes)
            if len(txBytes) >= 2 and txBytes[:2] == "0x":
                txByteArray = hex2bytes(txBytes[2:])
            txValue = decode_big_endian(
                BytesBuffer(txByteArray), len(txBytes))
            if txValue < self.txCount:
                return 6
        return 0

    def commit(self):
        self.hashCount += 1
        if self.txCount == 0:
            return "", 0
        h = encode_big_endian(self.txCount, 8)
        h.reverse()
        return str(h), 0

    def add_listener(self):
        return 0

    def rm_listener(self):
        return 0

    def event(self):
        return


if __name__ == '__main__':
    l = len(sys.argv)
    if l == 1:
        port = 26658
    elif l == 2:
        port = int(sys.argv[1])
    else:
        print "too many arguments"
        quit()

    print 'ABCI Demo APP (Python)'

    app = CounterApplication()
    server = ABCIServer(app, port)
    server.main_loop()
