.PHONY: deploy-prod deploy-server deploy-worker deploy-tx-indexer deploy-grafana

NS ?= plugin-fee

deploy-prod: deploy-configs deploy-server deploy-worker deploy-tx-indexer deploy-grafana

deploy-configs:
	kubectl -n $(NS) apply -f deploy/prod

deploy-server:
	kubectl -n $(NS) apply -f deploy/01_server.yaml
	kubectl -n $(NS) rollout status deployment/server --timeout=300s

deploy-worker:
	kubectl -n $(NS) apply -f deploy/01_worker.yaml
	kubectl -n $(NS) rollout status deployment/worker --timeout=300s

deploy-tx-indexer:
	kubectl -n $(NS) apply -f deploy/01_tx_indexer.yaml
	kubectl -n $(NS) rollout status deployment/tx-indexer --timeout=300s

deploy-grafana:
	kubectl -n $(NS) apply -f deploy/02_grafana_dashboard.yaml
