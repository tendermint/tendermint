--- 
order: 10
---

# State Sync

With fast sync a node is downloading all of the data of a application and verifying it. With state sync your node will download data related to the head of the chain. Then begin participating in consensus. This leads to significantly quicker times for joining networks.

## Using State Sync

> NOTE: Before trying to use state sync see if the application you are operating a node for supports it. 

To start using state sync you must first enable it in the `config.toml` file. Under the state sync section you will find multiple settings that need to be configured in order for your node to catch up.

Lets breakdown the settings:

- `enable`: Enable is to inform the node that you will be using state sync to bootstrap your node.
- `rpc_servers`: RPC servers are needed because the state sync server utilizes the light client for verification. 
  It is recommended to have greater than 2 servers. 
- `temp_dir`: Temporary directory is store the chunks in the machines local storage, If nothing is set it will create a directory in `/tmp`

The next information you will need to acquire it through publicly exposed RPC's or a block explorer which you trust. 

- `trust_height`: Trust height is needed to inform the light client of your trusted height. 
- `trust_hash` Trust hash is needed to inform the light client of your trusted hash. 
- `trust_period` Trust period is the period in which headers can be verified. 
  > :warning: This value should be significantly smaller than the unbonding period.
