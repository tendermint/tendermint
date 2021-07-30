#!/bin/sh

go run github.com/vektra/mockery/v2 --disable-version-string --case underscore --name $*
