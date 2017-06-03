import os
from slackclient import SlackClient
import argparse

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Slack notification script")
    parser.add_argument('--channel','-c', required=True, help="Slack channel.")
    parser.add_argument('--message','-m', required=True, help="The message to send.")
    parser.add_argument('--username', required=False, help="Username used on Slack")
    parser.add_argument('--api_token', required=False, help="Slack API token. Can be set in SLACK_API_TOKEN environment variable too.")
    parser.add_argument('--icon_emoji','-i', required=False, help="Icon for the message.")
    args = parser.parse_args()

    username = args.username
    api_token=args.api_token
    if api_token is None:
        api_token=os.getenv("SLACK_API_TOKEN")
    message = args.message
    channel = args.channel
    icon_emoji = args.icon_emoji

    slack_client = SlackClient(api_token)
    apitest = slack_client.api_call("api.test")
    if not apitest['ok']:
        raise ValueError("api.test error: {0}".format(apitest['error']))

    authtest = slack_client.api_call("auth.test")
    if not authtest['ok']:
        raise ValueError("auth.test error: {0}".format(authtest['error']))

    if username is None:
        username = authtest['user']

    if icon_emoji is None:
        result = slack_client.api_call("chat.postMessage", channel=channel, text=message, username=username, icon_emoji=icon_emoji)
    else:
        result = slack_client.api_call("chat.postMessage", channel=channel, text=message, username=username)

    if not result['ok']:
        raise ValueError("Message error: {0}".format(result['error']))

