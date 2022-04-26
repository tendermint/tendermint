#!/bin/sh
set -euo pipefail

readonly root="$(dirname $0)"
cd "$root"

readonly oldvers=v0.34.19
readonly newvers=v0.35.x
readonly addr=localhost:26657
readonly tmhome="$PWD/tmhome"
readonly cfpath="$tmhome/config/config.toml"
readonly branch="$(git branch --show-current)"
readonly usevers="${1:-$newvers}"

mkdir -p bin

build_version() {
    local vers="$1"
    local wdir="build-${vers}"
    set -x
    if [[ "$vers" = ambient ]] ; then
        wdir=".."
        trap 'set +x' RETURN
    else
        git worktree add "$wdir" "$vers"
        trap "git worktree remove '$wdir'; set +x" RETURN
    fi
    (
        set -x
        cd "$wdir"
        make build

        if [[ -d ./scripts/confix ]] ; then
            go build -o ./build/confix ./scripts/confix
        fi
    )
    mv "${wdir}/build/tendermint" bin/tendermint-"$vers"
    mv "${wdir}/build/confix" bin/confix || true
}

call() {
    local method="$1"
    shift 1
    curl --fail-with-body -s "http://$addr/$method?$@"
}

put_transaction() {
    local key="$1"
    local val="$2"
    diag ":: adding transaction $key = $val"
    call broadcast_tx_commit "tx=\"${key}=${val}\"" | jq -r .result.hash
}

get_transaction() {
    local hash="$1"
    diag "Looking up transaction for hash $hash"
    call tx "hash=0x${hash}" | jq -r '(.result//.).tx|@base64d'
}

check_txn() {
    diag "Checking transactions..."
    for h in "$@" ; do
        diag ":: hash $h: " "$(get_transaction "$h")"
    done
}

diag() { echo "-- $@" 1>&2; }

diag "Installing Tendermint CLI"
for vers in "$oldvers" "$usevers" ; do
    diag ":: version $vers"
    build_version "$vers"
done

diag "Cleaning up TM settings"
rm -fr -- "$tmhome"

diag "Starting TM $oldvers"
./bin/tendermint-"$oldvers" --home="$tmhome" init
./bin/tendermint-"$oldvers" --home="$tmhome" start \
                 --proxy_app=kvstore \
		 2>/dev/null 1>&2 &
sleep 2

diag "Adding transactions..."
hash1="$(put_transaction t1 alpha)"
diag ":: transaction hash is $hash1"
hash2="$(put_transaction t2 bravo)"
diag ":: transaction hash is $hash2"
hash3="$(put_transaction t3 charlie)"
diag ":: transaction hash is $hash3"

check_txn "$hash1" "$hash2" "$hash3"

diag "Height now:" "$(call blockchain | jq -r .result.last_height)"

diag "Restarting TM $oldvers"
kill %1; wait
./bin/tendermint-"$oldvers" --home="$tmhome" start \
                 --proxy_app=kvstore \
		 2>/dev/null 1>&2 &
sleep 2

check_txn "$hash1" "$hash2" "$hash3"

diag "Stopping TM $oldvers"
kill %1; wait

diag "Updating configuration file ${cfpath}..."
./bin/confix -config "$cfpath" -out "$cfpath"

diag "Migrating databases with $usevers"
./bin/tendermint-"$usevers" --home="$tmhome" key-migrate

diag "Starting TM inspector for $usevers"
./bin/tendermint-"$usevers" --home="$tmhome" inspect &
sleep 2

diag "Height now:" "$(call blockchain | jq -r .result.last_height)"

check_txn "$hash1" "$hash2" "$hash3"

diag "Stopping TM inspector $usevers"
kill %1; wait

diag "Starting TM node $usevers"
./bin/tendermint-"$usevers" \
                 --home="$tmhome" start \
                 --proxy-app=kvstore &
sleep 10

check_txn "$hash1" "$hash2" "$hash3"

diag "Waiting for interrupt, address is http://${addr}/"
trap 'kill %1; wait' INT
wait
