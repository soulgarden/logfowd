FROM golang:1.21.5-alpine3.18 as builder

ENV GOPATH=/go

COPY . $GOPATH/src/github.com/soulgarden/logfowd

WORKDIR $GOPATH/src/github.com/soulgarden/logfowd

RUN go get github.com/mailru/easyjson && \
    go install github.com/mailru/easyjson/...@latest &&  \
    go generate ./... && \
    go build -o /bin/logfowd

FROM alpine:3.18

RUN adduser -S www-data -G www-data

COPY --from=builder --chown=www-data /bin/logfowd /bin/logfowd

CMD ["/bin/logfowd", "worker"]