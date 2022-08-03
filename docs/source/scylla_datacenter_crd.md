# Scylla Datacenter CRD

Scylla database clusters can be created and configured using the `scylladatacenters.scylla.scylladb.com` custom resource definition (CRD).

Please refer to the the [user guide walk-through](generic.md) for deployment instructions.
This page will explain all the available configuration options on the Scylla CRD.

## Sample

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaDatacenter
metadata:
  name: simple-cluster
  namespace: scylla
spec:
  image: docker.io/scylladb/scylla:5.0.0
  enableDeveloperMode: true
  cpuset: false
  removeOrphanedPVs: true
  repairs:
  - name: "weekly us-east-1 repair"
    intensity: "2"
    interval: "7d"
    dc: ["us-east-1"]
  backups:
  - name: "daily users backup"
    rateLimit: ["50"]
    location: ["s3:cluster-backups"]
    interval: "1d"
    keyspace: ["users"]
  - name: "weekly full cluster backup"
    rateLimit: ["50"]
    location: ["s3:cluster-backups"]
    interval: "7d"
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500G
          storageClassName: local-raid-disks
        scyllaContainer:
          resources:
            requests:
              cpu: 8
              memory: 32Gi
            limits:
              cpu: 8
              memory: 32Gi
        placement:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                  - key: failure-domain.beta.kubernetes.io/region
                    operator: In
                    values:
                      - us-east-1
                  - key: failure-domain.beta.kubernetes.io/zone
                    operator: In
                    values:
                      - us-east-1a
          tolerations:
            - key: role
              operator: Equal
              value: scylla-clusters
              effect: NoSchedule
```

## Settings Explanation

Use `kubectl explain scylladatacenters.scylla.scylladb.com` to find out more about each CRD field.