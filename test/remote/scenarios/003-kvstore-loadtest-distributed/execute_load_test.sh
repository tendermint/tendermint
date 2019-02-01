#!/bin/sh
set -e

TARGET_HOSTS=${TARGET_HOSTS:-"http://tik0.sredev.co:26657"}
CSV_OUTPUT_FILE=${CSV_OUTPUT_FILE:-"./loadtest"}
NUM_CLIENTS=${NUM_CLIENTS:-"1000"}
HATCH_RATE=${HATCH_RATE:-"20"}
EXPECTED_SLAVE_COUNT=${EXPECTED_SLAVE_COUNT:-"3"}
RUN_TIME=${RUN_TIME:-"60s"}
LOG_FILE=${LOG_FILE:-"./loadtest.log"}
STDOUT_FILE=${STDOUT_FILE:-"loadtest.stdout.log"}
LOADTEST_MASTER_NODE=${LOADTEST_MASTER_NODE:-"tok0"}
LOADTEST_MASTER_HOSTNAME=${LOADTEST_MASTER_HOSTNAME:-"tok0.sredev.co"}

# Parameters common to both master and slaves
LOCUST_PARAMS="--csv ${CSV_OUTPUT_FILE} -c ${NUM_CLIENTS} -r ${HATCH_RATE} --logfile ${LOG_FILE}"

source venv/bin/activate

if [ "${INVENTORY_HOSTNAME}" == "${LOADTEST_MASTER_NODE}" ]; then
    HOST_URLS=${TARGET_HOSTS} \
        locust -f locust_file.py \
        --no-web \
        --master \
        --master-bind-host 0.0.0.0 \
        --master-bind-port 5557 \
        --expect-slaves ${EXPECTED_SLAVE_COUNT} \
        -t ${RUN_TIME} \
        ${LOCUST_PARAMS} > ${STDOUT_FILE} 2>&1
else
    HOST_URLS=${TARGET_HOSTS} \
        locust -f locust_file.py \
        --no-web \
        --slave \
        --master-host ${LOADTEST_MASTER_HOSTNAME} \
        --master-port 5557 \
        ${LOCUST_PARAMS} > ${STDOUT_FILE} 2>&1
fi
