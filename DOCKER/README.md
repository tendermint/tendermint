# Supported tags and respective `Dockerfile` links

- `0.9.1`, `0.9`, `latest` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/809e0e8c5933604ba8b2d096803ada7c5ec4dfd3/DOCKER/Dockerfile)
- `0.9.0` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/d474baeeea6c22b289e7402449572f7c89ee21da/DOCKER/Dockerfile)
- `0.8.0`, `0.8` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/bf64dd21fdb193e54d8addaaaa2ecf7ac371de8c/DOCKER/Dockerfile)
- `develop` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/master/DOCKER/Dockerfile.develop)

`develop` tag points to the [develop](https://github.com/tendermint/tendermint/tree/develop) branch.

# Tendermint

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state transition machine, written in any programming language, and securely replicates it on many machines.

For more background, see the [introduction](https://tendermint.com/intro).

To get started developing applications, see the [application developers guide](https://tendermint.com/docs/guides/app-development).

# How to use this image

## Start one instance of the Tendermint core with the `dummy` app

A very simple example of a built-in app and Tendermint core in one container.

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint
```

## mintnet-kubernetes

If you want to see many containers talking to each other, consider using [mintnet-kubernetes](https://github.com/tendermint/mintnet-kubernetes), which is a tool for running Tendermint-based applications on a Kubernetes cluster.

# Supported Docker versions

This image is officially supported on Docker version 1.13.1.

Support for older versions (down to 1.6) is provided on a best-effort basis.

Please see [the Docker installation documentation](https://docs.docker.com/installation/) for details on how to upgrade your Docker daemon.

# License

View [license information](https://raw.githubusercontent.com/tendermint/tendermint/master/LICENSE) for the software contained in this image.

# User Feedback

## Issues

If you have any problems with or questions about this image, please contact us through a [GitHub](https://github.com/tendermint/tendermint/issues) issue. If the issue is related to a CVE, please check for [a `cve-tracker` issue on the `official-images` repository](https://github.com/docker-library/official-images/issues?q=label%3Acve-tracker) first.

You can also reach the image maintainers via [Slack](http://forum.tendermint.com:3000/).

## Contributing

You are invited to contribute new features, fixes, or updates, large or small; we are always thrilled to receive pull requests, and do our best to process them as fast as we can.

Before you start to code, we recommend discussing your plans through a [GitHub](https://github.com/tendermint/tendermint/issues) issue, especially for more ambitious contributions. This gives other contributors a chance to point you in the right direction, give you feedback on your design, and help you find out if someone else is working on the same thing.
