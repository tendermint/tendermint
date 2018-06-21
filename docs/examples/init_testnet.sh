#!/usr/bin/env bash

# make all the files
tendermint init --home ./tester/node0
tendermint init --home ./tester/node1
tendermint init --home ./tester/node2
tendermint init --home ./tester/node3

file0=./tester/node0/config/genesis.json
file1=./tester/node1/config/genesis.json
file2=./tester/node2/config/genesis.json
file3=./tester/node3/config/genesis.json

genesis_time=`cat $file0 | jq '.genesis_time'`
chain_id=`cat $file0 | jq '.chain_id'`

value0=`cat $file0 | jq '.validators[0].pub_key.value'`
value1=`cat $file1 | jq '.validators[0].pub_key.value'`
value2=`cat $file2 | jq '.validators[0].pub_key.value'`
value3=`cat $file3 | jq '.validators[0].pub_key.value'`

rm $file0
rm $file1
rm $file2
rm $file3

echo "{
  \"genesis_time\": $genesis_time,
  \"chain_id\": $chain_id,
  \"validators\": [
    {
     \"pub_key\": {
       \"type\": \"tendermint/PubKeyEd25519\",
       \"value\": $value0
     },
      \"power:\": 10,
      \"name\":, \"\"
    },
    {
     \"pub_key\": {
       \"type\": \"tendermint/PubKeyEd25519\",
       \"value\": $value1
     },
      \"power:\": 10,
      \"name\":, \"\"
    },
    {
     \"pub_key\": {
       \"type\": \"tendermint/PubKeyEd25519\",
       \"value\": $value2
     },
      \"power:\": 10,
      \"name\":, \"\"
    },
    {
     \"pub_key\": {
       \"type\": \"tendermint/PubKeyEd25519\",
       \"value\": $value3
     },
      \"power:\": 10,
      \"name\":, \"\"
    }
   ],
   \"app_hash\": \"\"
}" >> $file0

cp $file0 $file1
cp $file0 $file2
cp $file2 $file3
