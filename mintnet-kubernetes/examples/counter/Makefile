create:
	@echo "==> Creating deployment"
	@kubectl create -f app.yaml

destroy:
	@echo "==> Destroying deployment"
	@kubectl delete -f app.yaml
	@kubectl delete pvc -l app=tm

.PHONY: create destroy
