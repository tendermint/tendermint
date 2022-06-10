# Genesis

The genesis file is the starting point of a chain. An application will populate the `app_state` field in the genesis with their required fields. Tendermint is not able to validate this section because it is unaware what application state consists of.

## Genesis Fields

- `genesis_time`: The time the blockchain started or will start. If nodes are started before this time they will sit idle until the time specified.
- `chain_id`: The chain identifier. Every chain should have a unique identifier. When conducting a fork based upgrade, we recommend changing the chainid to avoid network or consensus errors.
- `initial_height`: The starting height of the blockchain. When conducting a chain restart to avoid restarting at height 1, the network is able to start at a specified height.
- `consensus_params`
    - `block`
        - `max_bytes`: The max amount of bytes a block can be.
        - `max_gas`: The maximum amount of gas that a block can have.
    - `evidence`
        - `max_age_num_blocks`: After this preset amount of blocks has passed a single piece of evidence is considered invalid.
        - `max_age_duration`: After this preset amount of time has passed a single piece of evidence is considered invalid.
        - `max_bytes`: The max amount of bytes of all evidence included in a block.
    - `validator`
        - `pub_key_types`: Defines which curves are to be accepted as a valid validator consensus key. Tendermint supports ed25519, sr25519 and secp256k1.
    - `version`
        - `app_version`: The version of the application. This is set by the application and is used to identify which version of the app a user should be using in order to operate a node.
    - `synchrony`
        - `message_delay`: A bound on how long a proposal message may take to reach all validators on a network and still be considered valid.
        - `precision`: A bound on how skewed the proposer's clock may be from any validator on the network while still producing valid proposals.
    - `timeout`
        - `propose`: How long the Tendermint consensus engine will wait for a proposal block before prevoting nil.
        - `propose_delta`: How much the propose timeout increase with each round.
        - `vote`: How long the consensus engine will wait after receiving +2/3 votes in a round.
        - `vote_delta`: How much the vote timeout increases with each round.
        - `commit`: How long the consensus engine will wait after receiving +2/3 precommits before beginning the next height.
        - `bypass_commit_timeout`: Configures if the consensus engine will wait for the full commit timeout before proceeding to the next height. If this field is set to true, the conesnsus engine will proceed to the next height as soon as the node has gathered votes from all of the validators on the network.
- `validators`
    - This is an array of validators. This validator set is used as the starting validator set of the chain. This field can be empty, if the application sets the validator set in `InitChain`.
- `app_hash`: The applications state root hash. This field does not need to be populated at the start of the chain, the application may provide the needed information via `Initchain`.
- `app_state`: This section is filled in by the application and is unknown to Tendermint.
