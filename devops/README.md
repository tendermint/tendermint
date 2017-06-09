# DevOps tools

This folder contains tools that are used for automated testnet deployments at Tendermint. These are tools specialized for the Tendermint infrastructure. However there might be some added value for people that want to set up similar networks. Use them at your own risk.

## slacknotification.py

A small script that can send Slack messages.

Requirements: slackclient python library

Install slackclient by running as root:
```
pip install slackclient
```

## terraform-tendermint/

A few extensions to the terraform-digitalocean and terraform-aws configurations. This folder is not a complete Terraform configuration in itself, merely a few files that can be copied over to the other Terraform folders. It contains extensions specific to the Tendermint infrastructure, such as DNS configuration for testnet nodes.

