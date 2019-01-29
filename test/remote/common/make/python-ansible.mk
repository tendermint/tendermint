ACTIVATE_VENV := source venv/bin/activate
.PHONY: update_deps

venv:
	python3 -m venv venv

venv/bin/ansible: venv
	@echo "Installing Ansible..."; \
	$(ACTIVATE_VENV); \
	pip install -U -r requirements.txt; \
	touch venv/bin/ansible

update_deps: venv
	@echo "Updating Python dependencies..."; \
	$(ACTIVATE_VENV); \
	pip install -U -r requirements.txt; \
	touch venv/bin/ansible
