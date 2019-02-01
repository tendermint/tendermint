INVENTORY?=../../inventory/hosts
LOGS_DIR?=/tmp
FAST_MODE?=false
.PHONY: deploy fetch-tm-logs status

include ../../common/make/tendermint.mk

deploy: $(TMBIN)
	rm -rf /tmp/nodes/
	tendermint testnet \
		--v 4 \
		--n 0 \
		--o /tmp/nodes/ \
		--hostname-prefix tik \
		--node-dir-prefix tik
	./update-node-config.sh
	ansible-playbook -i $(INVENTORY) \
		-e "{\"fast_mode\":${FAST_MODE}}" \
		deploy.yml
	./check-status.sh

fetch-tm-logs:
	ansible-playbook -i $(INVENTORY) fetch-tm-logs.yml

status:
	@SLEEP=0 ./check-status.sh
