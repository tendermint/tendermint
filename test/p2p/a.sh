#! /bin/bash

h1=10
h2=0
while [ $h2 -lt $(($h1+3)) ]; do
	echo "$h2"
	h2=$(($h2+1))
done
