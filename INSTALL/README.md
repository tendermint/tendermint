NOTE: Only Ubuntu 14.04 64bit is supported at this time.

### Dependencies

Install the dependencies and create a new user `tmuser`

    curl -L https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/ssh_config.sh > ssh_config.sh
    curl -L https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/install_env.sh > install_env.sh
    source install_env.sh
    cd /home/tmuser

### Install Go

Don't use `apt-get install golang`, it's still on an old version.

    curl -L https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/install_golang.sh > install_golang.sh
    source install_golang.sh

### Install/Update Tendermint

    go get -u github.com/tendermint/tendermint/cmd/tendermint  # get+update
    go install github.com/tendermint/tendermint/cmd/tendermint # install
    tendermint node
