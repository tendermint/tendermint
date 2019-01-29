# Remote Testing

This folder will eventually contain a number of recipes for deploying remote
test networks for debugging and testing various different Tendermint network
configurations and scenarios. The primary tool for deploying these various
scenarios is [Ansible](https://docs.ansible.com/ansible/latest/).

## Folder layout
Following is a description of this folder's layout:

```
|_ networks/         The different remote network configurations
|_ scenarios/        The different testing scenarios from the client side
|_ inventory/        Where to store all of your host inventory files
|_ Makefile          The primary Makefile for executing the different scenarios
```

## Requirements
TODO

## Deploying test networks
To deploy a particular test network to the relevant hosts, simply do the
following:

```bash
make network:001-reference
```

To override the specific set of hosts we will use for deployment of the test
network, simply do the following:

```bash
INVENTORY=inventory/aws-network-01 \
    make network:001-reference
```

## Executing test scenarios
To execute a particular testing scenario, simply:

```bash
make scenario:001-simple-kvstore-interaction
```
