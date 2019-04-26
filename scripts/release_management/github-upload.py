#!/usr/bin/env python

# Upload a file to a GitHub draft release. --id and --file are required.
# Optimized for CircleCI

import json
import os
import re
import argparse
import mimetypes
import httplib
from base64 import b64encode


def request(baseurl, path, mimetype, mimeencoding, data):
  user_and_pass = b64encode(b"{0}:{1}".format(os.environ['GITHUB_USERNAME'], os.environ['GITHUB_TOKEN'])).decode("ascii")
  
  headers = {
    'User-Agent': 'tenderbot',
    'Accept': 'application/vnd.github.v3.raw+json',
    'Authorization': 'Basic %s' % user_and_pass,
    'Content-Type': mimetype,
    'Content-Encoding': mimeencoding
  }

  conn = httplib.HTTPSConnection(baseurl, timeout=5)
  conn.request('POST', path, data, headers)
  response = conn.getresponse()
  if response.status < 200 or response.status > 299:
    print(response)
    conn.close()
    raise IOError(response.reason)
  responsedata = response.read()
  conn.close()
  return json.loads(responsedata)


if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--id", help="GitHub release ID", required=True, type=int)
  parser.add_argument("--file", default="/tmp/workspace/tendermint_{0}_{1}_{2}.zip".format(os.environ.get('CIRCLE_TAG'),os.environ.get('GOOS'),os.environ.get('GOARCH')), help="File to upload")
  parser.add_argument("--return-id-only", help="Return only the release ID after upload to GitHub.", action='store_true')
  args = parser.parse_args()

  if not os.environ.has_key('GITHUB_USERNAME'):
    raise parser.error('GITHUB_USERNAME not set.')

  if not os.environ.has_key('GITHUB_TOKEN'):
    raise parser.error('GITHUB_TOKEN not set.')

  mimetypes.init()
  filename = os.path.basename(args.file)
  mimetype,mimeencoding = mimetypes.guess_type(filename, strict=False)
  if mimetype is None:
    mimetype = 'application/zip'
  if mimeencoding is None:
    mimeencoding = 'utf8'

  with open(args.file,'rb') as f:
    asset = f.read()

  result = request('uploads.github.com', '/repos/tendermint/tendermint/releases/{0}/assets?name={1}'.format(args.id, filename), mimetype, mimeencoding, asset)

  if args.return_id_only:
    print(result['id'])
  else:
    print(result['browser_download_url'])

