#!/usr/bin/env python

# Bump the release number of a semantic version number and print it. --version is required.
# Version is
# - vA.B.C, in which case vA.B.C+1 will be returned
# - vA.B.C-devorwhatnot in which case vA.B.C will be returned
# - vA.B in which case vA.B.0 will be returned

import re
import argparse
import sys


def semver(ver):
  if re.match('v[0-9]+\.[0-9]+',ver) is None:
    ver="v0.0"
    #raise argparse.ArgumentTypeError('--version must be a semantic version number with major, minor and patch numbers')
  return ver


def get_tendermint_version():
  """Extracts the current Tendermint version from version/version.go"""
  pattern = re.compile(r"TMCoreSemVer = \"(?P<version>([0-9.]+)+)\"")
  with open("version/version.go", "rt") as version_file:
    for line in version_file:
      m = pattern.search(line)
      if m:
        return m.group('version')

  return None


if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--version", help="Version number to bump, e.g.: v1.0.0", required=True, type=semver)
  args = parser.parse_args()

  found = re.match('(v[0-9]+\.[0-9]+)(\.(.+))?', args.version)
  majorminorprefix = found.group(1)
  patch = found.group(3)
  if patch is None:
    patch = "0-new"

  if re.match('[0-9]+$',patch) is None:
    patchfound = re.match('([0-9]+)',patch)
    patch = int(patchfound.group(1))
  else:
    patch = int(patch) + 1

  expected_version = "{0}.{1}".format(majorminorprefix, patch)
  # if we're doing a release
  if expected_version != "v0.0.0":
    cur_version = get_tendermint_version()
    if not cur_version:
      print("Failed to obtain Tendermint version from version/version.go")
      sys.exit(1)
    expected_version_noprefix = expected_version.lstrip("v")
    if expected_version_noprefix != "0.0.0" and expected_version_noprefix != cur_version:
      print("Expected version/version.go#TMCoreSemVer to be {0}, but was {1}".format(expected_version_noprefix, cur_version))
      sys.exit(1)

  print(expected_version)
