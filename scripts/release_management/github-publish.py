#!/usr/bin/env python

# Publish an existing GitHub draft release. --id required.
# Optimized for CircleCI

import json
import os
import argparse
import httplib
from base64 import b64encode


def request(org, repo, id, data):
  user_and_pass = b64encode(b"{0}:{1}".format(os.environ['GITHUB_USERNAME'], os.environ['GITHUB_TOKEN'])).decode("ascii")
  headers = {
    'User-Agent': 'tenderbot',
    'Accept': 'application/vnd.github.v3+json',
    'Authorization': 'Basic %s' % user_and_pass
  }

  conn = httplib.HTTPSConnection('api.github.com', timeout=5)
  conn.request('POST', '/repos/{0}/{1}/releases/{2}'.format(org,repo,id), data, headers)
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
  parser.add_argument("--org", default="tendermint", help="GitHub organization")
  parser.add_argument("--repo", default="tendermint", help="GitHub repository")
  parser.add_argument("--id", help="GitHub release ID", required=True, type=int)
  parser.add_argument("--version", default=os.environ.get('CIRCLE_TAG'), help="Version number for the release, e.g.: v1.0.0")
  args = parser.parse_args()

  if not os.environ.has_key('GITHUB_USERNAME'):
    raise parser.error('GITHUB_USERNAME not set.')

  if not os.environ.has_key('GITHUB_TOKEN'):
    raise parser.error('GITHUB_TOKEN not set.')

  try:
    result = request(args.org, args.repo, args.id, data=json.dumps({'draft':False,'tag_name':"{0}".format(args.version)}))
  except IOError as e:
    print(e)
    result = request(args.org, args.repo, args.id, data=json.dumps({'draft':False,'tag_name':"{0}-autorelease".format(args.version)}))

  print(result['name'])
