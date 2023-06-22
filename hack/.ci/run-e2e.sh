#!/usr/bin/env bash
#
# Copyright (C) 2023 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

kubectl apply --server-side -f ./pkg/api/scylla/v1alpha1/scylla.scylladb.com_nodeconfigs.yaml

kubectl apply --server-side -f - << EOF
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: cluster
spec:
  # TODO: make local disk setup create a disk img on a filesystem so we don't need the SSD.
  localDiskSetup:
    filesystems:
    - device: /dev/nvme0n1
      type: xfs
    mounts:
    - device: /dev/nvme0n1
      mountPoint: /mnt/persistent-volumes
      unsupportedOptions:
      - prjquota
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: role
      operator: Equal
      value: scylla-clusters
EOF

kubectl -n local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/scylladb/k8s-local-volume-provisioner/dcfceb52d9122192e50236463060cb5151d90dc4/deploy/kubernetes/local-csi-driver/{00_namespace,10_csidriver,10_driver_serviceaccount,10_provisioner_clusterrole,20_provisioner_clusterrolebinding,50_daemonset}.yaml
kubectl -n local-csi-driver patch daemonset/local-csi-driver --type=json -p='[{"op": "add", "path": "/spec/template/spec/nodeSelector/pool", "value":"workers"}]'
# FIXME: The storage class should live alongside the manifests.
kubectl -n local-csi-driver apply --server-side -f https://raw.githubusercontent.com/scylladb/k8s-local-volume-provisioner/dcfceb52d9122192e50236463060cb5151d90dc4/example/storageclass_xfs.yaml
# TODO: Our tests shouldn't require a default storage class but rather have an option to specify which one to use
kubectl patch storageclass scylladb-local-xfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

REENTRANT=true timeout -v 16m ./hack/ci-deploy.sh quay.io/scylladb-dev/ci:scylla-operator-${SOCI_PR_TAG}

# We need to deploy the operator first to setup the disks.
kubectl -n local-csi-driver rollout status --timeout=5m daemonset/local-csi-driver

ingress_address="$( kubectl -n haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' )"

kubectl create namespace e2e
kubectl -n e2e create clusterrolebinding e2e --clusterrole=cluster-admin --serviceaccount=e2e:default

# kubectl -n e2e create poddisruptionbudget e2e --selector=app=e2e --max-unavailable=0
# timeout -v 70m kubectl -n e2e run -l=app=e2e --attach --restart=Never --image=quay.io/tnozicka/scylla-operator:latest --image-pull-policy=Always --command=true e2e -- scylla-operator-tests run all \
# --loglevel=2 \
# --artifacts-dir="${ARTIFACTS}" \
# --timeout=60m \
# --feature-gates=AllAlpha=true,AllBeta=true \
# --override-ingress-address="${ingress_address}"
# FIXME: cp artifacts like junit

ARTIFACTS_DIR="${ARTIFACTS}" timeout -v 10m ./hack/ci-gather-artifacts.sh
