This page helps you set your GOPATH environment variable after [installing go](https://golang.org/doc/install).

<hr/>

The GOPATH environment variable is a directory where all Go projects live.

First, make your GOPATH directory

```bash
> mkdir ~/go
```

Then, set environment variables in your shell startup script.  On OS X, this is `~/.bash_profile`.  For different operating systems or shells, the path may be different.  For Ubuntu, it may be `~/.profile` or `~/.bashrc`.

```bash
echo export GOPATH=\"\$HOME/go\" >> ~/.bash_profile
echo export PATH=\"\$PATH:\$GOPATH/bin\" >> ~/.bash_profile
```