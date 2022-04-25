#!/usr/bin/env bash

set -e

function success {
    [ -t 1 ] && echo -e "\e[32mSUCCESS:\e[0m" "$@" || echo "SUCCESS:" "$@"
}

function debug {
    [ -t 1 ] && echo -e "\e[93mDEBUG:\e[0m" "$@" || echo "DEBUG:" "$@"
}

function error {
    debug Error: "$@"
    cleanup
    [ -t 1 ] && echo -e '\e[91mERROR:\e[0m' "$@" || echo "ERROR:" "$@"
    exit 1
}
function displayHelp {
    cat <<EOF
# $(basename "$0")

This script prepares new release of Tenderdash. It:

* generates changelog
* creates a release branch (release_VERSION)
* creates pull request with changelog

Once the pull request is accepted, you still need to tag and create a release manually.

To use, you need to checkout your current development branch (like 'v0.7-dev') first.

## Usage

    $0 [flags] <platform|tenderdash>

where flags can be one of:
    -r=<x.y.z-dev.n>, --release=<x.y.z-dev.n> - release number, like 0.7.0 (REQUIRED)
    --cleanup - clean up before releasing; it can remove your local changes
    -C=<path> - path to local Tenderdash repository
    -h, --help - display this help message

## Examples

### Full release of 0.7.4

git checkout v0.7-dev
$0  --release=0.7.4

### Prerelease of 0.8.0-dev.3

git checkout v0.8-dev
$0  --release=0.8.0-dev.3

EOF
}

function configureDefaults {
    debug Configuring default values
    REPO_DIR="$(realpath "$(dirname "${0}")/../..")"
}

function parseArgs {
    debug Parsing command line
    while [ "$#" -ge 1 ]; do
        # for arg in "$@"; do
        arg="$1"
        case $arg in
        --cleanup)
            CLEANUP=yes
            shift
            ;;
        -r=* | --release=*)
            NEW_PACKAGE_VERSION="${arg#*=}"
            shift
            ;;
        -r | --release)
            shift
            if [ -n "$1" ]; then
                NEW_PACKAGE_VERSION="${1#*=}"
            fi
            shift
            ;;
        -C=*)
            REPO_DIR="${arg#*=}"
            shift
            ;;
        -C)
            shift
            REPO_DIR="${1#*=}"
            shift
            ;;
        -h | --help)
            displayHelp
            shift
            exit 0
            ;;
        *)
            error "Unrecoginzed command line argument '$arg';  try '$0 --help'"
            ;;
        esac
    done
}

function configureFinal() {
    debug Finalizing configuration
    VERSION_WITHOUT_PRERELEASE=${NEW_PACKAGE_VERSION%-*}

    if [ "${VERSION_WITHOUT_PRERELEASE}" == "${NEW_PACKAGE_VERSION}" ]; then
        ## Full release
        RELEASE_TYPE=release
    else
        RELEASE_TYPE=prerelease
    fi

    CURRENT_BRANCH="$(git branch --show-current)"
    SOURCE_BRANCH="v${VERSION_WITHOUT_PRERELEASE%.*}-dev"
    RELEASE_BRANCH="release_${NEW_PACKAGE_VERSION}"
    MILESTONE="v${VERSION_WITHOUT_PRERELEASE}"

    if [[ $RELEASE_TYPE != "prerelease" ]]; then # full release
        TARGET_BRANCH="master"
    else # prerelease
        TARGET_BRANCH="v${VERSION_WITHOUT_PRERELEASE%.*}-dev"
    fi

    debug "Repository: ${REPO_DIR}"
    debug "Release type: ${RELEASE_TYPE}"
    debug "New version: ${NEW_PACKAGE_VERSION}"
    debug "Source branch: ${SOURCE_BRANCH}"
    debug "Target branch: ${TARGET_BRANCH}"
}

function validate {
    debug Validating configuration
    if [ -z "${NEW_PACKAGE_VERSION}" ]; then
        error "You must provide new release version with --release=x.y.z; see '$0 --help' for more details"
    fi

    if [[ "${CURRENT_BRANCH}" != "${SOURCE_BRANCH}" ]]; then
        error "you must run this script from the \"${SOURCE_BRANCH}\" branch"
    fi

    local UNCOMMITTED_FILES
    UNCOMMITTED_FILES="$(git status -su)"
    if [ -n "$UNCOMMITTED_FILES" ]; then
        error "Commit or stash your changes before running this script"
    fi

    # ensure github authentication
    if ! gh auth status &>/dev/null; then
        gh auth login
    fi
}

function generateChangelog {
    debug Generating CHANGELOG

    CLIFF_CONFIG="${REPO_DIR}/scripts/release/cliff.toml"
    if [[ "${RELEASE_TYPE}" = "prerelease" ]]; then
        CLIFF_CONFIG="${REPO_DIR}/scripts/release/cliff-pre.toml"
    fi

    echo 2>"${REPO_DIR}/CHANGELOG.md"

    docker run --rm -ti -u "$(id -u)" \
        -v "${REPO_DIR}/.git":/app/:ro \
        -v "${CLIFF_CONFIG}":/cliff.toml:ro \
        -v "${REPO_DIR}/CHANGELOG.md":/CHANGELOG.md \
        orhunp/git-cliff:latest \
        --config /cliff.toml \
        --strip all \
        --tag "$NEW_PACKAGE_VERSION" \
        --output /CHANGELOG.md \
        --unreleased \
        "v0.7.0..HEAD"
}

function updateVersionGo {
    sed -i'' -e "s/TMVersionDefault = \"[^\"]*\"\s*\$/TMVersionDefault = \"${NEW_PACKAGE_VERSION}\"/g" "${REPO_DIR}/version/version.go"
}

function createReleasePR {
    debug "Creating release branch ${RELEASE_BRANCH}"
    git pull -q
    git checkout -q -b "${RELEASE_BRANCH}"

    # commit changes
    git commit -m "chore(release): update changelog and version to $NEW_PACKAGE_VERSION" \
        "$REPO_DIR/CHANGELOG.md" \
        "$REPO_DIR/version/version.go"

    # push changes
    git push --force -u origin "${RELEASE_BRANCH}"

    debug "Creating milestone $MILESTONE if it doesn't exist yet"
    gh api --silent --method POST 'repos/dashevo/tenderdash/milestones' --field "title=${MILESTONE}" || true

    if [[ -n "$(getPrURL)" ]]; then
        debug "PR for branch $TARGET_BRANCH already exists, skipping creation"
    else
        debug "Creating PR for branch $TARGET_BRANCH"
        gh pr create --base "$TARGET_BRANCH" \
            --fill \
            --title "chore(release): update changelog and bump version to $NEW_PACKAGE_VERSION" \
            --body-file "$REPO_DIR/scripts/release/pr_description.md" \
            --milestone "$MILESTONE"
    fi
}

function getPrURL() {
    gh pr list --json url --jq '.[0].url' -H "${RELEASE_BRANCH}" -B "$TARGET_BRANCH"
}

function getPrState() {
    gh pr list --json state --jq .[0].state -H "${RELEASE_BRANCH}" -B "$TARGET_BRANCH" --state all
}

function waitForMerge() {
    debug 'Waiting for the PR to be merged; use ^C to cancel'

    while [[ "$(getPrState)" != "MERGED" ]]; do
        sleep 5
    done
}

function createRelease() {
    gh_args=""
    if [[ "$RELEASE_TYPE" = "prerelease" ]]; then
        gh_args=--prerelease
    fi

    gh release create \
        --draft \
        --notes-file "${REPO_DIR}/build/CHANGELOG_CURRENT.md" \
        --title "v${NEW_PACKAGE_VERSION}" \
        $gh_args \
        "v${NEW_PACKAGE_VERSION}"
}

function deleteRelease() {
    if [[ "$(gh release view --json isDraft --jq .isDraft "v${NEW_PACKAGE_VERSION}")" == "true" ]]; then
        gh release delete "v${NEW_PACKAGE_VERSION}"
    fi
}

function getReleaseUrl() {
    gh release view --json url --jq .url "v${NEW_PACKAGE_VERSION}"
}

function cleanup() {
    debug Cleaning up

    git checkout --quiet -- "${REPO_DIR}/CHANGELOG.md"
    git checkout --quiet "${SOURCE_BRANCH}" || true
    git branch --quiet -D "${RELEASE_BRANCH}" || true

    # We need to re-detect current branch again
    CURRENT_BRANCH="$(git branch --show-current)"
}

configureDefaults
parseArgs "$@"
configureFinal

if [ -n "$CLEANUP" ]; then
    cleanup
    deleteRelease
fi

validate
generateChangelog
updateVersionGo
createReleasePR

PR_URL="$(getPrURL)"

success "New release branch ${RELEASE_BRANCH} for ${NEW_PACKAGE_VERSION} prepared successfully."
success "Release PR: ${PR_URL}"
success "Please review it, merge and create a release in Github."

waitForMerge

success "Release branch ${RELEASE_BRANCH} for ${NEW_PACKAGE_VERSION} is merged. Preparing the release."
createRelease

cleanup

sleep 5 # wait for the release to be finalized

success "Release ${NEW_PACKAGE_VERSION} created successfully."
success "Accept it at: $(getReleaseUrl)"
