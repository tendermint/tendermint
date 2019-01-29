#!/usr/bin/env python3
# Test script for interacting with the Tendermint kvstore application

import os
import sys
import binascii
import base64

import requests


def test_kvstore(put_host_addr, get_host_addr):
    """Executes a simple test against the key/value store running on the given
    Tendermint node."""

    url = "http://%s" % put_host_addr
    # generate a random hex value to send through to the kv store
    test_value = binascii.b2a_hex(os.urandom(15)).decode('utf-8')
    payload = {'tx': '"test_value=%s"' % test_value}
    r = requests.get(url+"/broadcast_tx_commit", params=payload)
    if r.status_code >= 400:
        print("Failed to send broadcast_tx_commit request. Got status: %d" % r.status_code)
        return 2

    # now try to fetch the value
    url = "http://%s" % get_host_addr
    payload = {'data': '"test_value"'}
    r = requests.get(url+'/abci_query', params=payload)
    if r.status_code >= 400:
        print("Failed to send abci_query request. Got status: %d" % r.status_code)
        return 3

    # check the response value
    stored_value = base64.b64decode(r.json()['result']['response']['value']).decode('utf-8')
    if stored_value != test_value:
        print("Expected %s, but got %s" % (test_value, stored_value))
        return 4

    print("kvstore test completed successfully!")
    return 0

def main():
    if len(sys.argv) > 2:
        sys.exit(test_kvstore(sys.argv[1], sys.argv[2]))
    else:
        print("Usage: test_kvstore.py <put_host> <get_host>")
        sys.exit(1)


if __name__ == "__main__":
    main()
