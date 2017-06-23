# Installing Go

All Tendermint related materials assume you have done everything described here.

## Download and Install

Download and install Go [directly from the website](https://golang.org/doc/install).

## Set GOPATH

To use Go, you must set the `GOPATH` environment variable.
The `GOPATH` environment variable refers to a directory where all Go projects live.

First, make your GOPATH directory

```
> mkdir ~/gocode
```

Then, set environment variables in your shell startup script. On OS X, this is `~/.bash_profile`. For different operating systems or shells, the path may be different. For Ubuntu, it may be `~/.profile` or `~/.bashrc`.

```
echo export GOPATH=\"\$HOME/gocode\" >> ~/.bash_profile
echo export PATH=\"\$PATH:\$GOPATH/bin\" >> ~/.bash_profile
```

This last step adds `$GOPATH/bin` to your `$PATH` so newly compiled Go binaries can be easily executed.

See the [Go docs](https://golang.org/doc/code.html#GOPATH) for more.
