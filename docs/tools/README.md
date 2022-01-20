---
order: 1
parent:
  title: Tooling
  order: 8
---

# Overview

Tendermint has some tools that are associated with it for:

- [Debugging](./debugging/pro.md)
- [Benchmarking](#benchmarking)
- [Testnets](#testnets)

## Benchmarking

- <https://github.com/informalsystems/tm-load-test>

`tm-load-test` is a distributed load testing tool (and framework) for load
testing Tendermint networks.

## Testnets

- <https://github.com/informalsystems/testnets>

This repository contains various different configurations of test networks for,
and relating to, Tendermint.

Use [Docker Compose](./docker-compose.md) to spin up Tendermint testnets on your
local machine.

Use [Terraform and Ansible](./terraform-and-ansible.md) to deploy Tendermint
testnets to the cloud.

See the `tendermint testnet --help` command for more help initializing testnets.
