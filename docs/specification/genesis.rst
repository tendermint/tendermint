Genesis
=======

The genesis.json file in ``$TMROOT`` defines the initial TendermintCore
state upon genesis of the blockchain (`see
definition <https://github.com/tendermint/tendermint/blob/master/types/genesis.go>`__).

NOTE: This does not (yet) specify the application state (e.g. initial
distribution of tokens). Currently we leave it up to the application to
load the initial application genesis state. In the future, we may
include genesis SetOption messages that get passed from TendermintCore
to the app upon genesis.

Fields
~~~~~~

-  ``genesis_time``: Official time of blockchain start.
-  ``chain_id``: ID of the blockchain. This must be unique for every
   blockchain. If your testnet blockchains do not have unique chain IDs,
   you will have a bad time.
-  ``validators``:
-  ``pub_key``: The first element specifies the pub\_key type. 1 ==
   Ed25519. The second element are the pubkey bytes.
-  ``power``: The validator's voting power.
-  ``name``: Name of the validator (optional).
-  ``app_hash``: The expected application hash (as returned by the
   ``Commit`` ABCI message) upon genesis. If the app's hash does not
   match, a warning message is printed.

Sample genesis.json
~~~~~~~~~~~~~~~~~~~

.. code:: json

    {
      "genesis_time": "2016-02-05T06:02:31.526Z",
      "chain_id": "chain-tTH4mi",
      "validators": [
        {
          "pub_key": [
            1,
            "9BC5112CB9614D91CE423FA8744885126CD9D08D9FC9D1F42E552D662BAA411E"
          ],
          "power": 1,
          "name": "mach1"
        },
        {
          "pub_key": [
            1,
            "F46A5543D51F31660D9F59653B4F96061A740FF7433E0DC1ECBC30BE8494DE06"
          ],
          "power": 1,
          "name": "mach2"
        },
        {
          "pub_key": [
            1,
            "0E7B423C1635FD07C0FC3603B736D5D27953C1C6CA865BB9392CD79DE1A682BB"
          ],
          "power": 1,
          "name": "mach3"
        },
        {
          "pub_key": [
            1,
            "4F49237B9A32EB50682EDD83C48CE9CDB1D02A7CFDADCFF6EC8C1FAADB358879"
          ],
          "power": 1,
          "name": "mach4"
        }
      ],
      "app_hash": "15005165891224E721CB664D15CB972240F5703F"
    }
