INVENTORY?=../../inventory/hosts
NUM_CLIENTS?=1000
HATCH_RATE?=20
RUN_TIME?=60s
FAST_MODE?=false
RESULTS_OUTPUT_DIR?=/tmp/003-kvstore-loadtest-distributed
.PHONY: execute

execute:
	mkdir -p $(RESULTS_OUTPUT_DIR)
	rm -rf $(RESULTS_OUTPUT_DIR)/*
	ansible-playbook -i $(INVENTORY) \
		-e "{\"num_clients\":${NUM_CLIENTS}}" \
		-e "{\"hatch_rate\":${HATCH_RATE}}" \
		-e "{\"run_time\":${RUN_TIME}}" \
		-e "{\"fast_mode\":${FAST_MODE}}" \
		-e "{\"results_output_dir\":\"${RESULTS_OUTPUT_DIR}\"}" \
		deploy.yml
