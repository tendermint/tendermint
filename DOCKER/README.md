# Supported tags and respective `Dockerfile` links

- `0.11.0`, `latest` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/9177cc1f64ca88a4a0243c5d1773d10fba67e201/DOCKER/Dockerfile)
- `0.10.0` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/e5342f4054ab784b2cd6150e14f01053d7c8deb2/DOCKER/Dockerfile)
- `0.9.1`, `0.9`, [(Dockerfile)](https://github.com/tendermint/tendermint/blob/809e0e8c5933604ba8b2d096803ada7c5ec4dfd3/DOCKER/Dockerfile)
- `0.9.0` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/d474baeeea6c22b289e7402449572f7c89ee21da/DOCKER/Dockerfile)
- `0.8.0`, `0.8` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/bf64dd21fdb193e54d8addaaaa2ecf7ac371de8c/DOCKER/Dockerfile)
- `develop` [(Dockerfile)](https://github.com/tendermint/tendermint/blob/master/DOCKER/Dockerfile.develop)

`develop` tag points to the [develop](https://github.com/tendermint/tendermint/tree/develop) branch.

# Quick reference

* **Where to get help:**
  [Chat on Rocket](https://cosmos.rocket.chat/)

* **Where to file issues:**
  https://github.com/tendermint/tendermint/issues

* **Supported Docker versions:**
  [the latest release](https://github.com/moby/moby/releases) (down to 1.6 on a best-effort basis)

# Tendermint

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state transition machine, written in any programming language, and securely replicates it on many machines.

For more background, see the [introduction](https://tendermint.readthedocs.io/en/master/introduction.html).

To get started developing applications, see the [application developers guide](https://tendermint.readthedocs.io/en/master/getting-started.html).

# How to use this image

## Start one instance of the Tendermint core with the `dummy` app

A very simple example of a built-in app and Tendermint core in one container.

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint node --proxy_app=dummy
```

## mintnet-kubernetes

If you want to see many containers talking to each other, consider using [mintnet-kubernetes](https://github.com/tendermint/tools/tree/master/mintnet-kubernetes), which is a tool for running Tendermint-based applications on a Kubernetes cluster.

# License

View [license information](https://raw.githubusercontent.com/tendermint/tendermint/master/LICENSE) for the software contained in this image.

# User Feedback

## Contributing

You are invited to contribute new features, fixes, or updates, large or small; we are always thrilled to receive pull requests, and do our best to process them as fast as we can.

Before you start to code, we recommend discussing your plans through a [GitHub](https://github.com/tendermint/tendermint/issues) issue, especially for more ambitious contributions. This gives other contributors a chance to point you in the right direction, give you feedback on your design, and help you find out if someone else is working on the same thing.
