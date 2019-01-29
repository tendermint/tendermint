# Remote Testing

This folder will eventually contain a number of recipes for deploying remote
test networks for debugging and testing various different Tendermint network
configurations and scenarios. The primary tool for deploying these various
scenarios is [Ansible](https://docs.ansible.com/ansible/latest/).

## Folder layout
Following is a description of this folder's layout:

```
|_ common/           Common `make` and Ansible includes
|_ inventory/        Where to store all of your Ansible host inventory files
|_ networks/         The different remote network configurations
|_ scenarios/        The different testing scenarios from the client side
|_ Makefile          The primary Makefile for executing the different network deployments and scenarios
```

## Requirements
In order to execute the various deployments or scenarios, you will need:

* `make`
* Python 3.6+
* Tendermint [development
  requirements](https://github.com/tendermint/tendermint#minimum-requirements)
  (for building the Tendermint binary that we deploy to the test networks)

Target platform for execution of these deployments/scenarios is either
Linux/macOS.

## Deploying test networks
To deploy a particular test network to the relevant hosts, simply do the
following:

```bash
make deploy:001-reference
```

Each network deployment potentially has a different set of parameters that one
can supply via environment variables. See each network's folder for details.

## Executing test scenarios
To execute a particular testing scenario, simply:

```bash
make scenario:001-kvstore-test
```

This particular test scenario assumes you're running the `kvstore` proxy app in
your Tendermint network.
