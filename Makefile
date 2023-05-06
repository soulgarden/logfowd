lint: fmt
	golangci-lint run --enable-all --fix .

fmt:
	gofmt -w .

test:
	ROOT_DIR=${PWD} go test -failfast ./...

gen:
	go generate ./...

#docker

docker_up du:
	docker-compose up -d --build

docker_down dd:
	docker-compose down

build:
	docker buildx build . -f ./docker/Dockerfile -t soulgarden/logfowd:0.0.9 --platform linux/amd64,linux/arm64/v8 --push

#helm

create_namespace:
	kubectl create -f ./helm/namespace-logging.json

helm_install:
	helm install -n=logging logfowd helm/logfowd --wait

helm_upgrade:
	helm upgrade -n=logging logfowd helm/logfowd --wait

helm_delete:
	helm uninstall -n=logging logfowd
