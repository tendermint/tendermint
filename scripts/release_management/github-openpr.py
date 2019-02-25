#!/usr/bin/env python

# Open a PR against the develop branch. --branch required.
# Optimized for CircleCI

import json
import os
import argparse
import httplib
from base64 import b64encode


def request(org, repo, data):
  user_and_pass = b64encode(b"{0}:{1}".format(os.environ['GITHUB_USERNAME'], os.environ['GITHUB_TOKEN'])).decode("ascii")
  headers = {
    'User-Agent': 'tenderbot',
    'Accept': 'application/vnd.github.v3+json',
    'Authorization': 'Basic %s' % user_and_pass
  }

  conn = httplib.HTTPSConnection('api.github.com', timeout=5)
  conn.request('POST', '/repos/{0}/{1}/pulls'.format(org,repo), data, headers)
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
  parser.add_argument("--org", default="tendermint", help="GitHub organization. Defaults to tendermint.")
  parser.add_argument("--repo", default="tendermint", help="GitHub repository. Defaults to tendermint.")
  parser.add_argument("--head", help="The name of the branch where your changes are implemented.", required=True)
  parser.add_argument("--base", help="The name of the branch you want the changes pulled into.", required=True)
  parser.add_argument("--title", default="Security release {0}".format(os.environ.get('CIRCLE_TAG')), help="The title of the pull request.")
  args = parser.parse_args()

  if not os.environ.has_key('GITHUB_USERNAME'):
    raise parser.error('GITHUB_USERNAME not set.')

  if not os.environ.has_key('GITHUB_TOKEN'):
    raise parser.error('GITHUB_TOKEN not set.')

  if os.environ.get('CIRCLE_TAG') is None:
    raise parser.error('CIRCLE_TAG not set.')

  result = request(args.org, args.repo, data=json.dumps({'title':"{0}".format(args.title),'head':"{0}".format(args.head),'base':"{0}".format(args.base),'body':"<Please fill in details.>"}))
  print(result['html_url'])
