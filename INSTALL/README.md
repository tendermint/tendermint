NOTE: Only Ubuntu 14.04 64bit is supported at this time.

### Server setup / create `tmuser`

Secure the server, install dependencies, and create a new user `tmuser`

    curl -L https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/install_env.sh > install_env.sh
    source install_env.sh
    cd /home/tmuser

### Install Go as `tmuser`

Don't use `apt-get install golang`, it's still on an old version.

    curl -L https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/install_golang.sh > install_golang.sh
    source install_golang.sh

### Run Barak

WARNING: THIS STEP WILL GIVE CONTROL OF THE CURRENT USER TO THE DEV TEAM.

    go install github.com/tendermint/tendermint/cmd/barak
    cat $GOPATH/src/github.com/tendermint/tendermint/cmd/barak/seed0 | barak

### Install/Update Tendermint

    go get -u github.com/tendermint/tendermint/cmd/tendermint  # get+update
    go install github.com/tendermint/tendermint/cmd/tendermint # install
    tendermint node
