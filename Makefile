lint: fmt
	golangci-lint run --enable-all --fix

fmt:
	gofmt -w .

test:
	ROOT_DIR=${PWD} go test -failfast ./...

#docker

build:
	docker buildx build . -f ./docker/Dockerfile -t soulgarden/logfowd:0.0.5 --platform linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64/v8 --push
