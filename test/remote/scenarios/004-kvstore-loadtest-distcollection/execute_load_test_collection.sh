#!/bin/sh
TEST_NETWORK=${TEST_NETWORK:-"001-reference"}
REDEPLOY_BETWEEN_TESTS=${REDEPLOY_BETWEEN_TESTS:-"true"}
num_clients=${START_NUM_CLIENTS}
hatch_rate=${START_HATCH_RATE}

# truncate load test log file
cat /dev/null > ${LOADTEST_LOG}

function loadtest_log {
    echo "\n---------------------------------------------------------------------"
    echo "$1"
    echo "---------------------------------------------------------------------\n"
    echo "$1" >> ${LOADTEST_LOG}
}

while [ $num_clients -le $END_NUM_CLIENTS ]; do
    if [ "${REDEPLOY_BETWEEN_TESTS}" == "true" ]; then
        loadtest_log "Redeploying test network: ${TEST_NETWORK}..."
        FAST_MODE=${FAST_MODE} \
            make -C ../../networks/${TEST_NETWORK} deploy
        EXIT_CODE=$?
        if [ $EXIT_CODE -eq 0 ]; then
            loadtest_log "SUCCESS!"
        else
            loadtest_log "FAILED: Exit code ${EXIT_CODE}"
        fi
    fi

    loadtest_log "Executing load test for ${num_clients} clients, hatch rate = ${hatch_rate}..."
    INVENTORY=${INVENTORY} \
        NUM_CLIENTS=${num_clients} \
        HATCH_RATE=${hatch_rate} \
        RUN_TIME=${RUN_TIME} \
        RUN_TIMEOUT=${RUN_TIMEOUT} \
        FAST_MODE=${FAST_MODE} \
        RESULTS_OUTPUT_DIR=${RESULTS_OUTPUT_DIR}/${num_clients}-clients-${RUN_TIME} \
        make -C ../003-kvstore-loadtest-distributed execute
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 0 ]; then
        loadtest_log "SUCCESS!\n"
    else
        loadtest_log "FAILED: Exit code ${EXIT_CODE}\n"
    fi

    num_clients=$(( $num_clients + $INC_NUM_CLIENTS ))
    hatch_rate=$(( $hatch_rate + $INC_HATCH_RATE ))
done
