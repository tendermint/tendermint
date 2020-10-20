# Produces the body for the release

version=$1

echo https://github.com/tendermint/tendermint/blob/${version}/CHANGELOG.md#${version} > release_notes.md
