# Logfowd

[![Go Report Card](https://goreportcard.com/badge/github.com/soulgarden/logfowd)](https://goreportcard.com/report/github.com/soulgarden/logfowd)

Logfowd collects logs from k8s using filesystem events and sends it to elasticsearch. The main goal is low memory and cpu consumption.

Supports ES 7.x, k8s 1.14+

### Install with helm
    make create_namespace

    make helm_install

### Linters

    make lint

### Tests

    make test
