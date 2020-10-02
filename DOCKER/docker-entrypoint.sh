#!/bin/bash
set -e

if [ ! -d "$TMHOME/config" ]; then
	echo "Running tendermint init to create (default) configuration for docker run."
	tendermint init

	sed -i \
		-e "s/^proxy_app\s*=.*/proxy_app = \"$PROXY_APP\"/" \
		-e "s/^moniker\s*=.*/moniker = \"$MONIKER\"/" \
		-e 's/^addr_book_strict\s*=.*/addr_book_strict = false/' \
		-e 's/^timeout_commit\s*=.*/timeout_commit = "500ms"/' \
		-e 's/^index_all_tags\s*=.*/index_all_tags = true/' \
		-e 's,^laddr = "tcp://127.0.0.1:26657",laddr = "tcp://0.0.0.0:26657",' \
		-e 's/^prometheus\s*=.*/prometheus = true/' \
		"$TMHOME/config/config.toml"

	jq ".chain_id = \"$CHAIN_ID\" | .consensus_params.block.time_iota_ms = \"500\"" \
		"$TMHOME/config/genesis.json" > "$TMHOME/config/genesis.json.new"
	mv "$TMHOME/config/genesis.json.new" "$TMHOME/config/genesis.json"
fi

exec tendermint "$@"
