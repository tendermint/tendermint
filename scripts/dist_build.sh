#!/usr/bin/env bash
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that.
cd "$DIR"

# Make sure build tools are available, get VENDORED dependencies and build
make get_tools get_vendor_deps build_cc

# Zip all the files.
echo "==> Packaging..."
for PLATFORM in $(find ./build/pkg -mindepth 1 -maxdepth 1 -type d); do
		OSARCH=$(basename "${PLATFORM}")
		echo "--> ${OSARCH}"

		pushd "$PLATFORM" >/dev/null 2>&1
		zip "../${OSARCH}.zip" ./*
		popd >/dev/null 2>&1
done

exit 0
