# this is just a temporary tool for testing purposes only 

rm -rf output

mkdir output

echo running byzantine test

go test -run=TestByzantineSimple -v > output/byzantine.txt

cd output

echo extracting test results

sed -i '' '/switch=/d' ./byzantine.txt

grep -i 'validator=0' ./byzantine.txt > validator0.txt

grep -i 'validator=1' ./byzantine.txt > validator1.txt

grep -i 'validator=2' ./byzantine.txt > validator2.txt

grep -i -E 'Added to |Signed and |Received Proposal' ./validator0.txt > validator0-votes.txt

grep -i -E 'Added to |Signed and |Received Proposal' ./validator1.txt > validator1-votes.txt

grep -i -E 'Added to |Signed and |Received Proposal' ./validator2.txt > validator2-votes.txt

echo task finished

