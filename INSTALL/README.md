### Dependencies

Install the dependencies.
[install_env.sh](https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/install_env.sh):

    #!/bin/bash
    # Run this as super user
    # This part is for installing go language and setting up a user account
    apt-get update -y
    apt-get upgrade -y
    apt-get install -y make screen gcc git mercurial libc6-dev pkg-config libgmp-dev
    useradd tmuser -d /home/tmuser
    usermod -aG sudo tmuser
    mkdir /home/tmuser
    chown -R tmuser /home/tmuser
    su tmuser

### Install Go

Install Go separately, from here: <http://golang.org/doc/install> or 
Don't use `apt-get install golang`, it's still on an old version.
[install_golang.sh](https://raw.githubusercontent.com/tendermint/tendermint/master/INSTALL/install_golang.sh):

    #!/bin/bash
    # Run this as tmuser user
    # This part is for installing go

    if [ `whoami` != "root" ];
    then
      cd /home/tmuser
      mkdir gocode
      wget https://storage.googleapis.com/golang/go1.4.2.src.tar.gz
      tar -xzvf go*.tar.gz
      cd go/src
      ./make.bash
      cd /home/tmuser
      cp /etc/skel/.bashrc .
      mkdir -p /home/tmuser/go/src
      echo 'export GOROOT=/home/tmuser/go' >> /home/tmuser/.bashrc
      echo 'export GOPATH=/home/tmuser/gocode' >> /home/tmuser/.bashrc
      echo 'export PATH=$PATH:$GOROOT/bin' >> /home/tmuser/.bashrc
      source /home/tmuser/.bashrc
    else
      echo "should not be root to run install_golang.sh"
    fi

### Install/Update Tendermint

    #!/bin/bash
    # Run this as tmuser user
    # This part is for installing Tendermint

    if [ `whoami` != "root" ];
    then
      cd
      source /home/tmuser/.bashrc
      rm -rf $GOPATH/src/github.com/tendermint/tendermint
      go get github.com/tendermint/tendermint

      cd $GOPATH/src/github.com/tendermint/tendermint
      git checkout develop
      git pull 
      make
    else
      echo "should not be root to run update_tendermint.sh"
    fi

Finally, head back to [Developer Quick Start](https://github.com/tendermint/tendermint/wiki/Developer-Quick-Start) to compile and run Tendermint.
