"""
Test file for generating load on a Tendermint network running the `kvstore`
proxy application from a single machine.
"""

import base64
import binascii
import os

from locust import HttpLocust, TaskSequence, seq_task


MIN_WAIT = int(os.environ.get('MIN_WAIT', 100))
MAX_WAIT = int(os.environ.get('MAX_WAIT', 500))


class KVStoreBalancedRWTaskSequence(TaskSequence):
    """A balanced read/write task set for interacting with the nodes."""

    @seq_task(1)
    def put_value(self):
        self.key = binascii.hexlify(os.urandom(16)).decode('utf-8')
        self.value = binascii.hexlify(os.urandom(16)).decode('utf-8')
        self.client.get(
            '/broadcast_tx_sync',
            params={
                'tx': '"%s=%s"' % (self.key, self.value),
            },
            name='/broadcast_tx_sync?tx=[tx]',
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


def make_node_locust_class(host_url, node_id):
    """Class factory for creating locusts."""
    class NodeUserLocust(HttpLocust):
        host = host_url
        weight = 1
        task_set = KVStoreBalancedRWTaskSequence
        min_wait = MIN_WAIT
        max_wait = MAX_WAIT

        def setup(self):
            print("setup: %s with host = %s" % (self.__class__.__name__, self.host))

    NodeUserLocust.__name__ = "Node%dUserLocust" % node_id
    return NodeUserLocust


DEFAULT_HOSTS = [
    'http://tik0.sredev.co:26657',
    'http://tik1.sredev.co:26657',
    'http://tik2.sredev.co:26657',
    'http://tik3.sredev.co:26657',
]

# Parse the nodes we want to attack from the environment
HOST_URLS = os.environ.get('HOST_URLS', '::'.join(DEFAULT_HOSTS)).split('::')

for i in range(len(HOST_URLS)):
    node_cls = make_node_locust_class(HOST_URLS[i], i)
    # Inject the class into global space, so it will be picked up by Locust
    globals()[node_cls.__name__] = node_cls
    print("Created class %s" % node_cls.__name__)
