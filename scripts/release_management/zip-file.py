#!/usr/bin/env python

# ZIP one file as "tendermint" into a ZIP like tendermint_VERSION_OS_ARCH.zip
# Use environment variables CIRCLE_TAG, GOOS and GOARCH for easy input parameters.
# Optimized for CircleCI

import os
import argparse
import zipfile
import hashlib


BLOCKSIZE = 65536


def zip_asset(file,destination,arcname,version,goos,goarch):
  filename = os.path.basename(file)
  output = "{0}/{1}_{2}_{3}_{4}.zip".format(destination,arcname,version,goos,goarch)

  with zipfile.ZipFile(output,'w') as f:
    f.write(filename=file,arcname=arcname)
    f.comment=filename
  return output


if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--file", default="build/tendermint_{0}_{1}".format(os.environ.get('GOOS'),os.environ.get('GOARCH')), help="File to zip")
  parser.add_argument("--destination", default="build", help="Destination folder for files")
  parser.add_argument("--version", default=os.environ.get('CIRCLE_TAG'), help="Version number for binary, e.g.: v1.0.0")
  parser.add_argument("--goos", default=os.environ.get('GOOS'), help="GOOS parameter")
  parser.add_argument("--goarch", default=os.environ.get('GOARCH'), help="GOARCH parameter")
  args = parser.parse_args()

  if args.version is None:
    raise parser.error("argument --version is required")
  if args.goos is None:
    raise parser.error("argument --goos is required")
  if args.goarch is None:
    raise parser.error("argument --goarch is required")

  file = zip_asset(args.file,args.destination,"tendermint",args.version,args.goos,args.goarch)
  print(file)

