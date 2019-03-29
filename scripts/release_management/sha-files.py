#!/usr/bin/env python

# Create SHA256 summaries from all ZIP files in a folder
# Optimized for CircleCI

import re
import os
import argparse
import zipfile
import hashlib


BLOCKSIZE = 65536


if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--folder", default="/tmp/workspace", help="Folder to look for, for ZIP files")
  parser.add_argument("--shafile", default="/tmp/workspace/SHA256SUMS", help="SHA256 summaries File")
  args = parser.parse_args()

  for filename in os.listdir(args.folder):
    if re.search('\.zip$',filename) is None:
      continue
    if not os.path.isfile(os.path.join(args.folder, filename)):
      continue
    with open(args.shafile,'a+') as shafile:
      hasher = hashlib.sha256()
      with open(os.path.join(args.folder, filename),'r') as f:
        buf = f.read(BLOCKSIZE)
        while len(buf) > 0:
          hasher.update(buf)
          buf = f.read(BLOCKSIZE)
      shafile.write("{0} {1}\n".format(hasher.hexdigest(),filename))  

