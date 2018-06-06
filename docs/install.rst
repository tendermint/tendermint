Install Tendermint
==================

The fastest and easiest way to install the ``tendermint`` binary
is to run `this script <https://github.com/tendermint/tendermint/blob/develop/scripts/install/install_tendermint_ubuntu.sh>`__ on
a fresh Ubuntu instance,
or `this script <https://github.com/tendermint/tendermint/blob/develop/scripts/install/install_tendermint_bsd.sh>`__
on a fresh FreeBSD instance. Read the comments / instructions carefully (i.e., reset your terminal after running the script,
make sure your okay with the network connections being made).

From Binary
-----------

To download pre-built binaries, see the `releases page <https://github.com/tendermint/tendermint/releases>`__.

From Source
-----------

You'll need ``go`` `installed <https://golang.org/doc/install>`__ and the required
`environment variables set <https://github.com/tendermint/tendermint/wiki/Setting-GOPATH>`__

Get Source Code
^^^^^^^^^^^^^^^

::

    mkdir -p $GOPATH/src/github.com/tendermint
    cd $GOPATH/src/github.com/tendermint
    git clone https://github.com/tendermint/tendermint.git
    cd tendermint

Get Tools & Dependencies
^^^^^^^^^^^^^^^^^^^^^^^^

::

    make get_tools
    make get_vendor_deps

Compile
^^^^^^^

::

    make install

to put the binary in ``$GOPATH/bin`` or use:

::

    make build

to put the binary in ``./build``.

The latest ``tendermint version`` is now installed.

Reinstall
---------

If you already have Tendermint installed, and you make updates, simply

::

    cd $GOPATH/src/github.com/tendermint/tendermint
    make install

To upgrade, there are a few options:

-  set a new ``$GOPATH`` and run
   ``go get github.com/tendermint/tendermint/cmd/tendermint``. This
   makes a fresh copy of everything for the new version.
-  run ``go get -u github.com/tendermint/tendermint/cmd/tendermint``,
   where the ``-u`` fetches the latest updates for the repository and
   its dependencies
-  fetch and checkout the latest master branch in
   ``$GOPATH/src/github.com/tendermint/tendermint``, and then run
   ``make get_vendor_deps && make install`` as above.

Note the first two options should usually work, but may fail. If they
do, use ``dep``, as above:

::

    cd $GOPATH/src/github.com/tendermint/tendermint
    make get_vendor_deps
    make install

Since the third option just uses ``dep`` right away, it should always
work.

Run
^^^

To start a one-node blockchain with a simple in-process application:

::

    tendermint init
    tendermint node --proxy_app=kvstore
