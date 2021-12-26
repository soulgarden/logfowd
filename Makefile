lint: fmt
	golangci-lint run --enable-all --fix

test:
	go clean -testcache
	CONFIGOR_ENV=local ROOT_DIR=${PWD} go test -count=1 -failfast -p 1 ./...

fmt:
	gofmt -w .

#docker

build:
	docker build . -f ./docker/Dockerfile -t soulgarden/logfowd:0.0.1
	docker push soulgarden/logfowd:0.0.1
