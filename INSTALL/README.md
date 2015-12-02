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

    go get -u github.com/tendermint/tendermint/cmd/barak
    nohup barak -config="$GOPATH/src/github.com/tendermint/tendermint/cmd/barak/seed" &

### Install/Update MintDB

    go get -u github.com/tendermint/tendermint/cmd/tendermint
    mkdir -p ~/.tendermint
    cp $GOPATH/src/github.com/tendermint/tendermint/config/tendermint/genesis.json ~/.tendermint/
    tendermint node --seeds="goldenalchemist.chaintest.net:46656"
