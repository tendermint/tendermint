INVENTORY?=../../inventory/hosts
NUM_CLIENTS?=1000
HATCH_RATE?=20
RUN_TIME?=60s
FAST_MODE?=false
.PHONY: execute

execute:
	rm -rf /tmp/003-kvstore-loadtest-distributed
	ansible-playbook -i $(INVENTORY) \
		-e "{\"num_clients\":${NUM_CLIENTS}}" \
		-e "{\"hatch_rate\":${HATCH_RATE}}" \
		-e "{\"run_time\":${RUN_TIME}}" \
		-e "{\"fast_mode\":${FAST_MODE}}" \
		deploy.yml
