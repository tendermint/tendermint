INVENTORY?=inventory/hosts

include common/make/ansible.mk

network\:%: venv/bin/ansible
	@$(ACTIVATE_VENV); \
	echo "Deploying network \"$*\"..."; \
	cd networks/$*/; \
	INVENTORY=../../$(INVENTORY) $(MAKE) deploy
