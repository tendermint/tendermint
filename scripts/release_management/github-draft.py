#!/usr/bin/env python

# Create a draft release on GitHub. By default in the tendermint/tendermint repo.
# Optimized for CircleCI

import argparse
import httplib
import json
import os
from base64 import b64encode

def request(org, repo, data):
  user_and_pass = b64encode(b"{0}:{1}".format(os.environ['GITHUB_USERNAME'], os.environ['GITHUB_TOKEN'])).decode("ascii")
  headers = {
    'User-Agent': 'tenderbot',
    'Accept': 'application/vnd.github.v3+json',
    'Authorization': 'Basic %s' % user_and_pass
  }
  
  conn = httplib.HTTPSConnection('api.github.com', timeout=5)
  conn.request('POST', '/repos/{0}/{1}/releases'.format(org,repo), data, headers)
  response = conn.getresponse()
  if response.status < 200 or response.status > 299:
    print("{0}: {1}".format(response.status, response.reason))
    conn.close()
    raise IOError(response.reason)
  responsedata = response.read()
  conn.close()
  return json.loads(responsedata)


def create_draft(org,repo,branch,version):
  draft = {
    'tag_name': version,
    'target_commitish': '{0}'.format(branch),
    'name': '{0} (WARNING: ALPHA SOFTWARE)'.format(version),
    'body': '<a href=https://github.com/{0}/{1}/blob/{2}/CHANGELOG.md#{3}>https://github.com/{0}/{1}/blob/{2}/CHANGELOG.md#{3}</a>'.format(org,repo,branch,version.replace('.','')),
    'draft': True,
    'prerelease': False
  }
  data=json.dumps(draft)
  return request(org, repo, data)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--org", default="tendermint", help="GitHub organization")
  parser.add_argument("--repo", default="tendermint", help="GitHub repository")
  parser.add_argument("--branch", default=os.environ.get('CIRCLE_BRANCH'), help="Branch to build from, e.g.: v1.0")
  parser.add_argument("--version", default=os.environ.get('CIRCLE_TAG'), help="Version number for binary, e.g.: v1.0.0")
  args = parser.parse_args()

  if not os.environ.has_key('GITHUB_USERNAME'):
    raise parser.error('environment variable GITHUB_USERNAME is required')

  if not os.environ.has_key('GITHUB_TOKEN'):
    raise parser.error('environment variable GITHUB_TOKEN is required')

  release = create_draft(args.org,args.repo,args.branch,args.version)

  print(release["id"])

