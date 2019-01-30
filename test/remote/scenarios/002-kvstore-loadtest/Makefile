NUM_CLIENTS?=1000
HATCH_RATE?=20
RUN_TIME?=60s
CSV_OUTPUT_FILE?=/tmp/002-kvstore-loadtest
LOG_FILE?=/tmp/002-kvstore-loadtest.log
.PHONY: execute

execute:
	@locust -f locust_file.py \
		--no-web \
		--csv $(CSV_OUTPUT_FILE) \
		-c $(NUM_CLIENTS) \
		-r $(HATCH_RATE) \
		-t $(RUN_TIME) \
		--logfile $(LOG_FILE)
