FROM golang:1.17.2-alpine3.14 as builder

ENV GOPATH=/go

COPY . $GOPATH/src/github.com/soulgarden/logfowd

WORKDIR $GOPATH/src/github.com/soulgarden/logfowd

RUN go build -o /bin/logfowd

FROM alpine:3.14

RUN adduser -S www-data -G www-data

COPY --from=builder --chown=www-data /bin/logfowd /bin/logfowd

CMD ["/bin/logfowd", "worker"]