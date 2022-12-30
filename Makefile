lint: fmt
	golangci-lint run --enable-all --fix

fmt:
	gofmt -w .

test:
	ROOT_DIR=${PWD} go test -failfast ./...

#docker

docker_up du:
	docker-compose up -d --build

build:
	docker buildx build . -f ./docker/Dockerfile -t soulgarden/logfowd:0.0.6 --platform linux/amd64,linux/arm64/v8 --push

gen:
	go generate ./...