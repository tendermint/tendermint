INVENTORY?=inventory/hosts

include common/make/ansible.mk

deploy\:%: venv/bin/ansible
	@$(ACTIVATE_VENV); \
	echo "Deploying network \"$*\"..."; \
	cd networks/$*/; \
	INVENTORY=../../$(INVENTORY) $(MAKE) deploy
