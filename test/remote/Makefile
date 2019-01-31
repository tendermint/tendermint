GOPATH?=$(shell go env GOPATH)
# NOTE: INVENTORY variable *must* be absolute path
INVENTORY?=$(GOPATH)/src/github.com/tendermint/tendermint/test/remote/inventory/hosts
LOGS_DIR?=/tmp

include common/make/python-ansible.mk

# Deploy test network with the specified name
deploy\:%: venv/bin/ansible
	@$(ACTIVATE_VENV); \
	echo "Deploying network \"$*\"..."; \
	$(MAKE) -C networks/$*/ deploy

# Fetch Tendermint logs from remote machines for the given network
fetch-tm-logs\:%: venv/bin/ansible
	@$(ACTIVATE_VENV); \
	$(MAKE) -C networks/$*/ fetch-tm-logs

# Check the status of the given network
status\:%: venv/bin/ansible
	@$(MAKE) -C networks/$*/ status

# Execute the specified test scenario
scenario\:%: venv/bin/ansible
	@echo "Executing scenario: $*..."
	@$(ACTIVATE_VENV); \
	$(MAKE) -C scenarios/$*/ execute
