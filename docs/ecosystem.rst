Tendermint Ecosystem
====================

Below are the many applications built using various pieces of the Tendermint stack. We thank the community for their contributions thus far  and welcome the addition of new projects. Feel free to submit a pull request to add your project!

ABCI Applications
-----------------

Burrow
^^^^^^

Ethereum Virtual Machine augmented with native permissioning scheme and global key-value store, written in Go, authored by Monax Industries, and incubated `by Hyperledger <https://github.com/hyperledger/burrow>`__.

cb-ledger
^^^^^^^^^

Custodian Bank Ledger, integrating central banking with the blockchains of tomorrow, written in C++, and `authored by Block Finance <https://github.com/block-finance/cpp-abci>`__.
      
Clearchain
^^^^^^^^^^

Application to manage a distributed ledger for money transfers that support multi-currency accounts, written in Go, and `authored by Allession Treglia <https://github.com/tendermint/clearchain>`__.

Comit
^^^^^

Public service reporting and tracking, written in Go, and `authored by Zach Balder <https://github.com/zbo14/comit>`__.
     
Cosmos SDK
^^^^^^^^^^

A prototypical account based crypto currency state machine supporting plugins, written in Go, and `authored by Cosmos <https://github.com/cosmos/cosmos-sdk>`__.

Ethermint
^^^^^^^^^

The go-ethereum state machine run as a ABCI app, written in Go, `authored by Tendermint <https://github.com/tendermint/ethermint>`__.

IAVL
^^^^

Immutable AVL+ tree with Merkle proofs, Written in Go, `authored by Tendermint <https://github.com/tendermint/iavl>`__.

Lotion
^^^^^^

A Javascript microframework for building blockchain applications with Tendermint, written in Javascript, `authored by Judd Keppel of Tendermint <https://github.com/keppel/lotion>`__. See also `lotion-chat <https://github.com/keppel/lotion-chat>`__ and `lotion-coin <https://github.com/keppel/lotion-coin>`__ apps written using Lotion.

MerkleTree
^^^^^^^^^^

Immutable AVL+ tree with Merkle proofs, Written in Java, `authored by jTendermint <https://github.com/jTendermint/MerkleTree>`__.

Passchain
^^^^^^^^^

Passchain is a tool to securely store and share passwords, tokens and other short secrets, `authored by trusch <https://github.com/trusch/passchain>`__.

Passwerk
^^^^^^^^

Encrypted storage web-utility backed by Tendermint, written in Go, `authored by Rigel Rozanski <https://github.com/rigelrozanski/passwerk>`__.

Py-Tendermint
^^^^^^^^^^^^^

A Python microframework for building blockchain applications with Tendermint, written in Python, `authored by Dave Bryson <https://github.com/davebryson/py-tendermint>`__.

Stratumn
^^^^^^^^

SDK for "Proof-of-Process" networks, written in Go, `authored by the Stratumn team <https://github.com/stratumn/sdk>`__.

TMChat
^^^^^^

P2P chat using Tendermint, written in Java, `authored by wolfposd <https://github.com/wolfposd/TMChat>`__.
      

ABCI Servers
------------

+------------------------------------------------------------------+--------------------+--------------+
| **Name**                                                         | **Author**         | **Language** |
|                                                                  |                    |              |
+------------------------------------------------------------------+--------------------+--------------+
| `abci <https://github.com/tendermint/abci>`__                    | Tendermint         | Go           |
+------------------------------------------------------------------+--------------------+--------------+
| `js abci <https://github.com/tendermint/js-abci>`__              | Tendermint         | Javascript   |
+------------------------------------------------------------------+--------------------+--------------+
| `cpp-tmsp <https://github.com/block-finance/cpp-abci>`__         | Martin Dyring      | C++          |
+------------------------------------------------------------------+--------------------+--------------+
| `c-abci <https://github.com/chainx-org/c-abci>`__                | ChainX             | C            |
+------------------------------------------------------------------+--------------------+--------------+
| `jabci <https://github.com/jTendermint/jabci>`__                 | jTendermint        | Java         |
+------------------------------------------------------------------+--------------------+--------------+
| `ocaml-tmsp <https://github.com/zbo14/ocaml-tmsp>`__             | Zach Balder        | Ocaml        |
+------------------------------------------------------------------+--------------------+--------------+
| `abci_server <https://github.com/KrzysiekJ/abci_server>`__       | Krzysztof Jurewicz | Erlang       |
+------------------------------------------------------------------+--------------------+--------------+
| `rust-tsp <https://github.com/tendermint/rust-tsp>`__            | Adrian Brink       | Rust         |
+------------------------------------------------------------------+--------------------+--------------+
| `hs-abci <https://github.com/albertov/hs-abci>`__                | Alberto Gonzalez   | Haskell      |
+------------------------------------------------------------------+--------------------+--------------+
| `haskell-abci <https://github.com/cwgoes/haskell-abci>`__        | Christoper Goes    | Haskell      |
+------------------------------------------------------------------+--------------------+--------------+
| `Spearmint <https://github.com/dennismckinnon/spearmint>`__      | Dennis Mckinnon    | Javascript   |
+------------------------------------------------------------------+--------------------+--------------+
| `py-tendermint <https://github.com/davebryson/py-tendermint>`__  | Dave Bryson        | Python       |
+------------------------------------------------------------------+--------------------+--------------+

Deployment Tools
----------------

See `deploy testnets <./deploy-testnets.html>`__ for information about all the tools built by Tendermint. We have Kubernetes, Ansible, and Terraform integrations.

Cloudsoft built `brooklyn-tendermint <https://github.com/cloudsoft/brooklyn-tendermint>`__ for deploying a tendermint testnet in docker continers. It uses Clocker for Apache Brooklyn.
