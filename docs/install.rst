Install Tendermint
==================

From Binary
-----------

To download pre-built binaries, see the `Download page <https://tendermint.com/download>`__.

From Source
-----------

You'll need `go`, maybe `glide` and the tendermint source code.

Install Go
^^^^^^^^^^

Make sure you have `installed Go <https://golang.org/doc/install>`__ and
set the ``GOPATH``.

Get Source Code
^^^^^^^^^^^^^^^

You should be able to install the latest with a simple

::

    go get github.com/tendermint/tendermint/cmd/tendermint

Run ``tendermint --help`` and ``tendermint version`` to ensure your
installation worked.

If the installation failed, a dependency may have been updated and become
incompatible with the latest Tendermint master branch. We solve this
using the ``glide`` tool for dependency management.

First, install ``glide``:

::

    go get github.com/Masterminds/glide

Now we can fetch the correct versions of each dependency by running:

::

    cd $GOPATH/src/github.com/tendermint/tendermint
    glide install
    go install ./cmd/tendermint

Note that even though ``go get`` originally failed, the repository was
still cloned to the correct location in the ``$GOPATH``.

The latest Tendermint Core version is now installed.

Reinstall
---------

If you already have Tendermint installed, and you make updates, simply

::

    cd $GOPATH/src/github.com/tendermint/tendermint
    go install ./cmd/tendermint

To upgrade, there are a few options:

-  set a new ``$GOPATH`` and run
   ``go get github.com/tendermint/tendermint/cmd/tendermint``. This
   makes a fresh copy of everything for the new version.
-  run ``go get -u github.com/tendermint/tendermint/cmd/tendermint``,
   where the ``-u`` fetches the latest updates for the repository and
   its dependencies
-  fetch and checkout the latest master branch in
   ``$GOPATH/src/github.com/tendermint/tendermint``, and then run
   ``glide install && go install ./cmd/tendermint`` as above.

Note the first two options should usually work, but may fail. If they
do, use ``glide``, as above:

::

    cd $GOPATH/src/github.com/tendermint/tendermint
    glide install
    go install ./cmd/tendermint

Since the third option just uses ``glide`` right away, it should always
work.

Troubleshooting
---------------

If ``go get`` failing bothers you, fetch the code using ``git``:

::

    mkdir -p $GOPATH/src/github.com/tendermint
    git clone https://github.com/tendermint/tendermint $GOPATH/src/github.com/tendermint/tendermint
    cd $GOPATH/src/github.com/tendermint/tendermint
    glide install
    go install ./cmd/tendermint

Run
^^^

To start a one-node blockchain with a simple in-process application:

::

    tendermint init
    tendermint node --proxy_app=dummy
