INVENTORY?=../../inventory/hosts
.PHONY: execute

execute:
	rm -rf /tmp/003-kvstore-loadtest-distributed
	ansible-playbook -i $(INVENTORY) deploy.yml
