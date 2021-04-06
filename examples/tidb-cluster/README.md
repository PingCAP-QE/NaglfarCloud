# Guide

## Install TiDB Operator

```bash
kubectl apply -f https://raw.githubusercontent.com/pingcap/tidb-operator/v1.1.11/manifests/crd.yaml
helm repo add pingcap https://charts.pingcap.org/
helm install tidb-operator pingcap/tidb-operator --namespace=tidb-admin --version=v1.1.11 -f ./values-tidb-operator.yaml --create-namespace
```

## Deploy a TiDB cluster

```bash
kubectl apply -f basic.yaml
```
