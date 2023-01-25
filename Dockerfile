FROM quay.io/scylladb/scylla-operator-images:golang-1.19 AS builder
WORKDIR /go/src/github.com/scylladb/scylla-operator
COPY . .
RUN make build --warn-undefined-variables

FROM quay.io/scylladb/scylla-operator-images:base-ubuntu

COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator /usr/bin/
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator-tests /usr/bin/
COPY ./hack/gke /usr/local/lib/scylla-operator/gke
ENTRYPOINT ["/usr/bin/scylla-operator"]
