# Logfowd

[![Go Report Card](https://goreportcard.com/badge/github.com/soulgarden/logfowd)](https://goreportcard.com/report/github.com/soulgarden/logfowd)

Logfowd collects logs from k8s using filesystem events and sends it to elasticsearch. The main goal is low memory and cpu consumption.
Support ES 7.x

### Install with helm

    helm install -n=logging logfowd helm --wait --dry-run