.PHONY: dev frontend build docker-build-amd64

dev: frontend

frontend:
	npm run dev

build:
	npm run build

docker-build-amd64:
	docker build --platform linux/amd64 -t starfire-frontend .
