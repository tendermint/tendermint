#!/usr/bin/env python

# Bump the release number of a semantic version number and print it. --version is required.
# Version is
# - vA.B.C, in which case vA.B.C+1 will be returned
# - vA.B.C-devorwhatnot in which case vA.B.C will be returned
# - vA.B in which case vA.B.0 will be returned

import re
import argparse


def semver(ver):
  if re.match('v[0-9]+\.[0-9]+',ver) is None:
    raise argparse.ArgumentTypeError('--version must be a semantic version number with major, minor and patch numbers')
  return ver


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

  print("{0}.{1}".format(majorminorprefix, patch))
