set -e

mkdir -p /test/p2p/

tendermint testnet \
  --config /test/docker/config-template.toml \ #TODO: take in laddr as a flag rather then a whole config
  --node-dir-prefix="mach" \
  --v=4 \
  --populate-persistent-peers=false \
  --o=/test/p2p/data
