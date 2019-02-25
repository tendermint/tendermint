# Release management scripts

## Overview
The scripts in this folder are used for release management in CircleCI. Although the scripts are fully configurable using input parameters,
the default settings were modified to accommodate CircleCI execution.

# Build scripts
These scripts help during the build process. They prepare the release files.

## bump-semver.py
Bumps the semantic version of the input `--version`. Versions are expected in vMAJOR.MINOR.PATCH format or vMAJOR.MINOR format.

In vMAJOR.MINOR format, the result will be patch version 0 of that version, for example `v1.2 -> v1.2.0`.

In vMAJOR.MINOR.PATCH format, the result will be a bumped PATCH version, for example `v1.2.3 -> v1.2.4`.

If the PATCH number contains letters, it is considered a development version, in which case, the result is the non-development version of that number.
The patch number will not be bumped, only the "-dev" or similar additional text will be removed. For example: `v1.2.6-rc1 -> v1.2.6`.

## zip-file.py
Specialized ZIP command for release management. Special features:
1. Uses Python ZIP libaries, so the `zip` command does not need to be installed.
1. Can only zip one file.
1. Optionally gets file version, Go OS and architecture.
1. By default all inputs and output is formatted exactly how CircleCI needs it.

By default, the command will try to ZIP the file at `build/tendermint_${GOOS}_${GOARCH}`.
This can be changed with the `--file` input parameter.

By default, the command will output the ZIP file to `build/tendermint_${CIRCLE_TAG}_${GOOS}_${GOARCH}.zip`.
This can be changed with the `--destination` (folder), `--version`, `--goos` and `--goarch` input parameters respectively.

## sha-files.py
Specialized `shasum` command for release management. Special features:
1. Reads all ZIP files in the given folder.
1. By default all inputs and output is formatted exactly how CircleCI needs it.

By default, the command will look up all ZIP files in the `build/` folder.

By default, the command will output results into the `build/SHA256SUMS` file.

# GitHub management
Uploading build results to GitHub requires at least these steps:
1. Create a new release on GitHub with content
2. Upload all binaries to the release
3. Publish the release
The below scripts help with these steps.

## github-draft.py
Creates a GitHub release and fills the content with the CHANGELOG.md link. The version number can be changed by the `--version` parameter.

By default, the command will use the tendermint/tendermint organization/repo, which can be changed using the `--org` and `--repo` parameters.

By default, the command will get the version number from the `${CIRCLE_TAG}` variable.

Returns the GitHub release ID.

## github-upload.py
Upload a file to a GitHub release. The release is defined by the mandatory `--id` (release ID) input parameter.

By default, the command will upload the file `/tmp/workspace/tendermint_${CIRCLE_TAG}_${GOOS}_${GOARCH}.zip`. This can be changed by the `--file` input parameter.

## github-publish.py
Publish a GitHub release. The release is defined by the mandatory `--id` (release ID) input parameter.

