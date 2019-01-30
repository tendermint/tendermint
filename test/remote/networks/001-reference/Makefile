INVENTORY?=../../inventory/hosts
.PHONY: deploy status

include ../../common/make/tendermint.mk

deploy: build-tmbin-linux
	rm -rf /tmp/nodes/
	tendermint testnet \
		--v 4 \
		--n 0 \
		--o /tmp/nodes/ \
		--hostname-prefix tik \
		--node-dir-prefix tik
	./fix-node-config.sh
	ansible-playbook -i $(INVENTORY) deploy.yml
	./check-status.sh

status:
	@SLEEP=0 ./check-status.sh
