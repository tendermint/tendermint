#! /bin/bash
set -e

go build -o server main.go
./server > /dev/null &
PID=$!
sleep 2


R1=`curl -s 'http://localhost:8008/hello_world?name="my_world"&num=5'`


R2=`curl -s --data @data.json http://localhost:8008`

kill -9 $PID

if [[ "$R1" != "$R2" ]]; then
	echo "responses are not identical:"
	echo "R1: $R1"
	echo "R2: $R2"
	exit 1
fi
echo "Success"
