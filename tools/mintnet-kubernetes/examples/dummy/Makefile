create:
	@echo "==> Creating deployment"
	@kubectl create -f app.yaml
	@echo "==> Waiting 10s until it is probably ready"
	@sleep 10
	@echo "==> Creating monitor and transacter pods"
	@kubectl create -f tm-monitor-pod.yaml
	@kubectl create -f transacter-pod.yaml

destroy:
	@echo "==> Destroying deployment"
	@kubectl delete -f transacter-pod.yaml
	@kubectl delete -f tm-monitor-pod.yaml
	@kubectl delete -f app.yaml
	@kubectl delete pvc -l app=tm

.PHONY: create destroy
