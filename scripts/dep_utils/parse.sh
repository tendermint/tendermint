#! /bin/bash
set -euo pipefail

LIB=$1

dep status | grep "$LIB" | awk '{print $4}'
