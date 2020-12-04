#!/usr/bin/env bash
set -e

# Get the tag from the version, or try to figure it out.
if [ -z "$TAG" ]; then
	TAG=$(awk -F\" '/TMCoreSemVer =/ { print $2; exit }' < ../version/version.go)
fi
if [ -z "$TAG" ]; then
		echo "Please specify a tag."
		exit 1
fi

TAG_NO_PATCH=${TAG%.*}

read -p "==> Build 3 docker images with the following tags (latest, $TAG, $TAG_NO_PATCH)? y/n" -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
		docker build -t "dashpay/tenderdash" -t "dashpay/tenderdash:$TAG" -t "dashpay/tenderdash:$TAG_NO_PATCH" ..
fi
