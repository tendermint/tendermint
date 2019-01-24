.PHONY: deploy stop update-prereq build-tendermint

ACTIVATE_VENV := source venv/bin/activate
TESTNET ?= tik

build-tendermint:
	@echo "Building Tendermint binary for Linux..."; \
	cd ${GOPATH}/src/github.com/tendermint/tendermint; \
	make build-linux; \
	echo "Done."

# Sets up a clean instance of this test network and fires it up
deploy: venv venv/bin/ansible
	@$(ACTIVATE_VENV); \
	ansible --version; \
	cd config; \
	echo "\nDeploying test net..."; \
	TESTNET=${TESTNET} ansible-playbook -i hosts deploy.yml; \
	echo "\nDone.";

venv:
	@echo "Setting up Python 3 virtual environment..."
	@python3 -m venv venv

venv/bin/ansible:
	@echo "Installing Ansible into virtual environment..."
	@$(ACTIVATE_VENV); \
	pip install -U -r requirements.txt;

update-prereq:
	@echo "Updating local prerequisites..."
	@$(ACTIVATE_VENV); \
	pip install -U -r requirements.txt;
	@echo "Done."

stop:
	@$(ACTIVATE_VENV); \
	cd config; \
	echo "Stopping test network..."; \
	TESTNET=${TESTNET} ansible-playbook -i hosts stop.yml; \
	echo "\nDone.";
