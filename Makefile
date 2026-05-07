.PHONY: up down build test test-python test-go test-ts lint lint-python lint-go lint-ts clean logs

up:
	docker compose up -d --build

down:
	docker compose down

build:
	docker compose build

test: test-python test-go test-ts

test-python:
	cd event-bus && pip install -r requirements.txt -q && pytest -v

test-go:
	cd api-gateway && go test -v ./...

test-ts:
	cd dashboard && npm install && npm test

lint: lint-python lint-go lint-ts

lint-python:
	cd event-bus && flake8 --max-line-length=120 --exclude=__pycache__ app.py test_app.py

lint-go:
	cd api-gateway && go vet ./...

lint-ts:
	cd dashboard && npm run lint

logs:
	docker compose logs -f

clean:
	docker compose down -v --rmi local
	rm -rf dashboard/node_modules dashboard/dist
	rm -rf api-gateway/api-gateway
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true

health:
	@echo "Event Bus:" && curl -s http://localhost:5001/health | python3 -m json.tool
	@echo "API Gateway:" && curl -s http://localhost:8080/health | python3 -m json.tool
	@echo "Dashboard:" && curl -s http://localhost:3000/health | python3 -m json.tool
