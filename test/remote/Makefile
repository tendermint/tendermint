INVENTORY?=inventory/hosts

include common/make/python-ansible.mk

deploy\:%: venv/bin/ansible
	@$(ACTIVATE_VENV); \
	echo "Deploying network \"$*\"..."; \
	INVENTORY=../../$(INVENTORY) $(MAKE) -C cd networks/$*/ deploy

status\:%: venv/bin/ansible
	@$(MAKE) -C networks/$*/ status

scenario\:%: venv/bin/ansible
	@echo "Executing scenario: $*..."
	@$(ACTIVATE_VENV); \
	$(MAKE) -C scenarios/$*/ execute
