INVENTORY?=../../inventory/hosts
NUM_CLIENTS?=1000
HATCH_RATE?=20
RUN_TIME?=60s
RUN_TIMEOUT?=70
FAST_MODE?=false
RESULTS_OUTPUT_DIR?=/tmp/003-kvstore-loadtest-distributed
.PHONY: execute fetch-results

execute:
	mkdir -p $(RESULTS_OUTPUT_DIR)
	rm -rf $(RESULTS_OUTPUT_DIR)/*
	ansible-playbook -i $(INVENTORY) \
		-e "{\"num_clients\":${NUM_CLIENTS}}" \
		-e "{\"hatch_rate\":${HATCH_RATE}}" \
		-e "{\"run_time\":${RUN_TIME}}" \
		-e "{\"run_timeout\":${RUN_TIMEOUT}}" \
		-e "{\"fast_mode\":${FAST_MODE}}" \
		-e "{\"results_output_dir\":\"${RESULTS_OUTPUT_DIR}\"}" \
		deploy.yml

# Command to help to just fetch the logs again
fetch-results:
	mkdir -p $(RESULTS_OUTPUT_DIR)
	rm -rf $(RESULTS_OUTPUT_DIR)/*
	ansible-playbook -i $(INVENTORY) \
		-e "{\"results_output_dir\":\"${RESULTS_OUTPUT_DIR}\"}" \
		fetch-logs.yml
