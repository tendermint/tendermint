"""
Test file for generating load on a Tendermint network running the `kvstore`
proxy application from a single machine.
"""

import base64
import binascii
import os

from locust import HttpLocust, TaskSequence, seq_task


class KVStoreBalancedRWTaskSequence(TaskSequence):
    """A balanced read/write task set for interacting with the nodes."""

    @seq_task(1)
    def put_value(self):
        self.key = binascii.hexlify(os.urandom(16)).decode('utf-8')
        self.value = binascii.hexlify(os.urandom(16)).decode('utf-8')
        self.client.get(
            '/broadcast_tx_commit',
            params={
                'tx': '"%s=%s"' % (self.key, self.value),
            },
            name='/broadcast_tx_commit?tx=[tx]',
        )

    @seq_task(2)
    def get_value(self):
        with self.client.get('/abci_query', params={'data': '"%s"' % self.key}, name='/abci_query?data=[data]',) as response:
            if response and hasattr(response, 'failure'):
                response_json = response.json()
                stored_encoded = response_json.get('result', {}).get('response', {}).get('value', None)
                if not stored_encoded:
                    response.failure("Missing result in response")
                else:
                    stored = base64.b64decode(stored_encoded).decode('utf-8')
                    if stored != self.value:
                        response.failure("Result does not match original value!")


class Node0UserLocust(HttpLocust):
    host = "http://tik0.sredev.co:26657"
    weight = 1
    task_set = KVStoreBalancedRWTaskSequence
    min_wait = 100
    max_wait = 500


class Node1UserLocust(HttpLocust):
    host = "http://tik1.sredev.co:26657"
    weight = 1
    task_set = KVStoreBalancedRWTaskSequence
    min_wait = 100
    max_wait = 500


class Node2UserLocust(HttpLocust):
    host = "http://tik2.sredev.co:26657"
    weight = 1
    task_set = KVStoreBalancedRWTaskSequence
    min_wait = 100
    max_wait = 500


class Node3UserLocust(HttpLocust):
    host = "http://tik3.sredev.co:26657"
    weight = 1
    task_set = KVStoreBalancedRWTaskSequence
    min_wait = 100
    max_wait = 500
