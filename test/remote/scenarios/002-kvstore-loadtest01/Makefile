NUM_CLIENTS?=1000
HATCH_RATE?=20
RUN_TIME?=60s
DATETIME=$(shell date +%Y%m%dT%H%M%S)
CSV_OUTPUT_FILE?=/tmp/002-kvstore-loadtest01-$(DATETIME)
LOG_FILE?=/tmp/002-kvstore-loadtest01-$(DATETIME).log
.PHONY: execute

execute:
	@locust -f locust_file.py \
		--no-web \
		--csv $(CSV_OUTPUT_FILE) \
		-c $(NUM_CLIENTS) \
		-r $(HATCH_RATE) \
		-t $(RUN_TIME) \
		--logfile $(LOG_FILE)
