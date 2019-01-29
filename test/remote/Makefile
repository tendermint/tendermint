INVENTORY?=inventory/hosts

include common/make/python-ansible.mk

deploy\:%: venv/bin/ansible
	@$(ACTIVATE_VENV); \
	echo "Deploying network \"$*\"..."; \
	cd networks/$*/; \
	INVENTORY=../../$(INVENTORY) $(MAKE) deploy

scenario\:%:
	@echo "Executing scenario: $*..."
	@$(ACTIVATE_VENV); \
	cd scenarios/$*/; \
	$(MAKE) execute
