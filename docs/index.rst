.. Tendermint documentation master file, created by
   sphinx-quickstart on Mon Aug  7 04:55:09 2017.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

Welcome to Tendermint!
======================


.. image:: assets/tmint-logo-blue.png
   :height: 200px
   :width: 200px
   :align: center

Introduction
------------

.. toctree::
   :maxdepth: 1

   introduction.md
   install.md
   getting-started.md
   using-tendermint.md
   deploy-testnets.md
   ecosystem.md

Tendermint Tools
----------------

.. the tools/ files are pulled in from the tools repo
.. see the bottom of conf.py
.. toctree::
   :maxdepth: 1

   tools/docker.md
   terraform-and-ansible.md
   tools/benchmarking.md
   tools/monitoring.md

ABCI, Apps, Logging, Etc
------------------------

.. toctree::
   :maxdepth: 1

   abci-cli.md
   abci-spec.md
   app-architecture.md
   app-development.md
   subscribing-to-events-via-websocket.md
   indexing-transactions.md
   how-to-read-logs.md
   running-in-production.md
   metrics.md

Research & Specification
------------------------

.. toctree::
   :maxdepth: 1

   determinism.md
   transactional-semantics.md

.. specification.md ## keep this file for legacy purpose. needs to be fixed though

* For a deeper dive, see `this thesis <https://atrium.lib.uoguelph.ca/xmlui/handle/10214/9769>`__.
* There is also the `original whitepaper <https://tendermint.com/static/docs/tendermint.pdf>`__, though it is now quite outdated.
* Readers might also be interested in the `Cosmos Whitepaper <https://cosmos.network/whitepaper>`__ which describes Tendermint, ABCI, and how to build a scalable, heterogeneous, cryptocurrency network.
* For example applications and related software built by the Tendermint team and other, see the `software ecosystem <https://tendermint.com/ecosystem>`__.

Join the `community <https://cosmos.network/community>`__ to ask questions and discuss projects.
