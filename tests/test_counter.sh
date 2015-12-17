
# so we can test other languages
if [[ "$COUNTER_APP" == "" ]]; then
	COUNTER_APP="counter"
fi

echo "Testing counter app for: $COUNTER_APP"

# run the counter app
$COUNTER_APP &> /dev/null &
PID=`echo $!`

if [[ "$?" != 0 ]]; then
	echo "Error running tmsp command"
	echo $OUTPUT
	exit 1
fi

sleep 1
OUTPUT=`(tmsp batch) <<STDIN 
set_option serial on
get_hash
append_tx abc
STDIN`

if [[ "$?" != 0 ]]; then
	echo "Error running tmsp command"
	echo $OUTPUT
	exit 1
fi

# why can't we pick up the non-zero exit code here?
# echo $?

HASH1=`echo "$OUTPUT" | tail -n +2 | head -n 1`
if [[ "${HASH1}" != "" ]]; then
	echo "Expected opening hash to be empty. Got $HASH1"	
	exit 1
fi

OUTPUT=`(tmsp batch) <<STDIN 
set_option serial on
append_tx 0x00
get_hash
append_tx 0x01
get_hash
STDIN`

if [[ "$?" != 0 ]]; then
	echo "Error running tmsp command"
	echo $OUTPUT
	exit 1
fi

HASH1=`echo "$OUTPUT" | tail -n +3 | head -n 1`
HASH2=`echo "$OUTPUT" | tail -n +5 | head -n 1`

if [[ "${HASH1:0:2}" != "01" ]]; then
	echo "Expected hash to lead with 01. Got $HASH1"	
	exit 1
fi

if [[ "${HASH2:0:2}" != "02" ]]; then
	echo "Expected hash to lead with 02. Got $HASH2"	
	exit 1
fi

echo "... Pass!"
echo ""

ps -p $PID > /dev/null
if [[ "$?" == "0" ]]; then
	kill -9 $PID
fi

