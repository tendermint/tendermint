GOPATH=$(go env GOPATH)
BLS_REPO_PATH="${GOPATH}"/src/github.com/quantumexplorer/bls-signatures

# Install packages for bindings
GO111MODULE=off go get golang.org/x/tools/cmd/goimports
GO111MODULE=off go get golang.org/x/lint/golint

# Cleaning previous build
rm -rf "${BLS_REPO_PATH}"
mkdir -pv "${GOPATH}"/github.com/quantumexplorer/

# Cloning bls repo and fetching dependencies
git clone https://github.com/quantumexplorer/bls-signatures.git "$BLS_REPO_PATH"
cd "$BLS_REPO_PATH"
git submodule update --init --recursive

# Build the bindings
mkdir build
cd build
cmake ../
cmake --build . -- -j 6
cd "${BLS_REPO_PATH}"/go-bindings
make
