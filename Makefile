lint: fmt
	golangci-lint run --enable-all --fix

fmt:
	gofmt -w .

test:
	ROOT_DIR=${PWD} go test -failfast ./...

#docker

build:
	docker build . -f ./docker/Dockerfile -t soulgarden/logfowd:0.0.4 --platform linux/amd64
	docker push soulgarden/logfowd:0.0.4
