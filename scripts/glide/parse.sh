#! /bin/bash
set -euo pipefail

GLIDE=$1
LIB=$2

cat $GLIDE | grep -A1 $LIB | grep -v $LIB | awk '{print $2}'
