#!/bin/bash

# symbol prefixes:
# g_ -> global
# l_ - local variable
# f_ -> function

set -euo pipefail

GITIAN_CACHE_DIRNAME='.gitian-builder-cache'
GO_RELEASE='1.13.3'
GO_TARBALL="go${GO_RELEASE}.linux-amd64.tar.gz"
GO_TARBALL_URL="https://dl.google.com/go/${GO_TARBALL}"

# Defaults

DEFAULT_SIGN_COMMAND='gpg --detach-sign'
DEFAULT_TENDERMINT_SIGS=${TENDERMINT_SIGS:-'tendermint.sigs'}
DEFAULT_GITIAN_REPO='https://github.com/tendermint/gitian-builder'
DEFAULT_GBUILD_FLAGS=''
DEFAULT_SIGS_REPO='https://github.com/tendermint/tendermint.sigs'

# Overrides

SIGN_COMMAND=${SIGN_COMMAND:-${DEFAULT_SIGN_COMMAND}}
GITIAN_REPO=${GITIAN_REPO:-${DEFAULT_GITIAN_REPO}}
GBUILD_FLAGS=${GBUILD_FLAGS:-${DEFAULT_GBUILD_FLAGS}}

# Globals

g_workdir=''
g_gitian_cache=''
g_cached_gitian=''
g_cached_go_tarball=''
g_sign_identity=''
g_sigs_dir=''
g_flag_commit=''


f_help() {
  cat >&2 <<EOF
Usage: $(basename $0) [-h] PLATFORM
Launch a gitian build from the current source directory for the given PLATFORM.
The following platforms are supported:
  darwin
  linux
  windows
  all

  Options:
   -h               display this help and exit
   -c               clone the signatures repository and commit signatures;
                    ignored if sign identity is not supplied
   -s IDENTITY      sign build as IDENTITY

If a GPG identity is supplied via the -s flag, the build will be signed and verified.
The signature will be saved in '${DEFAULT_TENDERMINT_SIGS}/'. An alternative output directory
for signatures can be supplied via the environment variable \$TENDERMINT_SIGS.

The default signing command used to sign the build is '$DEFAULT_SIGN_COMMAND'.
An alternative signing command can be supplied via the environment
variable \$SIGN_COMMAND.
EOF
}


f_builddir() {
  printf '%s' "${g_workdir}/gitian-build-$1"
}

f_prep_build() {
  local l_platforms \
    l_os \
    l_builddir

  l_platforms="$1"

  if [ -n "${g_flag_commit}" -a ! -d "${g_sigs_dir}" ]; then
    git clone ${DEFAULT_SIGS_REPO} "${g_sigs_dir}"
  fi

  for l_os in ${l_platforms}; do
    l_builddir="$(f_builddir ${l_os})"

    f_echo_stderr "Preparing build directory $(basename ${l_builddir}), restoring files from cache"
    cp -ar "${g_cached_gitian}" "${l_builddir}" >&2
    mkdir "${l_builddir}/inputs/"
    cp -v "${g_cached_go_tarball}" "${l_builddir}/inputs/"
  done
}

f_build() {
  local l_descriptor

  l_descriptor=$1

  bin/gbuild --commit tendermint="$g_commit" ${GBUILD_FLAGS} "$l_descriptor"
  libexec/stop-target || f_echo_stderr "warning: couldn't stop target"
}

f_sign_verify() {
  local l_descriptor

  l_descriptor=$1

  bin/gsign -p "${SIGN_COMMAND}" -s "${g_sign_identity}" --destination="${g_sigs_dir}" --release=${g_release} ${l_descriptor}
  bin/gverify --destination="${g_sigs_dir}" --release="${g_release}" ${l_descriptor}
}

f_commit_sig() {
  local l_release_name

  l_release_name=$1

  pushd "${g_sigs_dir}"
  git add . || echo "git add failed" >&2
  git commit -m "Add ${l_release_name} reproducible build" || echo "git commit failed" >&2
  popd
}

f_prep_docker_image() {
  pushd $1
  bin/make-base-vm --docker --suite bionic --arch amd64
  popd
}

f_ensure_cache() {
  g_gitian_cache="${g_workdir}/${GITIAN_CACHE_DIRNAME}"
  [ -d "${g_gitian_cache}" ] || mkdir "${g_gitian_cache}"

  g_cached_go_tarball="${g_gitian_cache}/${GO_TARBALL}"
  if [ ! -f "${g_cached_go_tarball}" ]; then
  f_echo_stderr "${g_cached_go_tarball}: cache miss, caching..."
    curl -L "${GO_TARBALL_URL}" --output "${g_cached_go_tarball}"
  fi

  g_cached_gitian="${g_gitian_cache}/gitian-builder"
  if [ ! -d "${g_cached_gitian}" ]; then
  f_echo_stderr "${g_cached_gitian}: cache miss, caching..."
    git clone ${GITIAN_REPO} "${g_cached_gitian}"
  fi
}

f_demangle_platforms() {
  case "${1}" in
  all)
    printf '%s' 'darwin linux windows' ;;
  linux|darwin|windows)
    printf '%s' "${1}" ;;
  *)
    echo "invalid platform -- ${1}"
    exit 1
  esac
}

f_echo_stderr() {
  echo $@ >&2
}


while getopts ":cs:h" opt; do
  case "${opt}" in
    h)  f_help ; exit 0 ;;
    c)  g_flag_commit=y ;;
    s)  g_sign_identity="${OPTARG}" ;;
  esac
done

shift "$((OPTIND-1))"

g_platforms=$(f_demangle_platforms "${1}")
g_workdir="$(pwd)"
g_commit="$(git rev-parse HEAD)"
g_sigs_dir=${TENDERMINT_SIGS:-"${g_workdir}/${DEFAULT_TENDERMINT_SIGS}"}

f_ensure_cache

f_prep_docker_image "${g_cached_gitian}"

f_prep_build "${g_platforms}"

export USE_DOCKER=1
for g_os in ${g_platforms}; do
  g_release="$(git describe --tags --abbrev=9 | sed 's/^v//')-${g_os}"
  g_descriptor="${g_workdir}/scripts/gitian-descriptors/gitian-${g_os}.yml"
  [ -f ${g_descriptor} ]
  g_builddir="$(f_builddir ${g_os})"

  pushd "${g_builddir}"
  f_build "${g_descriptor}"
  if [ -n "${g_sign_identity}" ]; then
    f_sign_verify "${g_descriptor}"
  fi
  popd

  if [ -n "${g_sign_identity}" -a -n "${g_flag_commit}" ]; then
    [ -d "${g_sigs_dir}/.git/" ] && f_commit_sig ${g_release} || f_echo_stderr "couldn't commit, ${g_sigs_dir} is not a git clone"
  fi
done

exit 0
