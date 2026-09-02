# Release cases

## Validation

```sh
# chart structure, yaml parsing of every rendered doc
helm lint deploy/wavelen -f deploy/values-vps.yaml
helm lint deploy/wavelen -f deploy/values-k3d-mac.yaml   
helm lint deploy/wavelen --strict                        # INFO/WARNING become failures

# full render to stdout. Catches template errors, bad indentation, missing required values
helm template wavelen deploy/wavelen -f deploy/values-vps.yaml

# hooks are excluded from a release manifest but included in a render
helm template wavelen deploy/wavelen -f deploy/values-vps.yaml --no-hooks

helm template wavelen deploy/wavelen -f deploy/values-vps.yaml --debug
```

Against a cluster

```sh
# render, then API server validates every object against its schema
helm template wavelen deploy/wavelen -f deploy/values-vps.yaml --validate
# same, through the install path
helm upgrade --install wavelen deploy/wavelen -f deploy/values-vps.yaml --dry-run=server

# catches an immutable-field violation (like a changed spec.selector)
helm template wavelen deploy/wavelen -f deploy/values-vps.yaml | kubectl apply --dry-run=server -f -
```

## Usual

The image comes from CI.
A push to `main` runs the release job
	-> computes the next patch version
	-> publishes `ghcr.io/salandered/wavelen:<version>`
	-> pushes the matching `v<version>` git tag.

Bump `Chart.appVersion` to that version, then

```sh
make k8s/apply/vps
```

`Chart.version` is a separate field, bumped when the templates or values change, not the app.

## Full reinstall

```sh
helm uninstall wavelen
kubectl delete job wavelen-migrate --ignore-not-found
kubectl wait --for=delete pod/postgres-0 --timeout=120s --ignore-not-found
kubectl delete pvc pgdata-postgres-0
make k8s/apply/vps
```

## Back up

```sh
kubectl exec postgres-0 -- sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' > backup.sql
```

## Restore

Empty the volume (see below), wait for Postgres to be Ready, restore, and only then apply the chart.

```sh
kubectl exec -i postgres-0 -- sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < backup.sql
```

## Keep the release, delete the volume

```sh
# Scale the StatefulSet down:
kubectl scale statefulset postgres --replicas=0
#  Wait for the pod to go away
kubectl wait --for=delete pod/postgres-0 --timeout=120s --ignore-not-found
#  Delete the claim
kubectl delete pvc pgdata-postgres-0
# Scale back up
kubectl scale statefulset postgres --replicas=1
# Wait for Postgres to be Ready
kubectl wait --for=condition=ready pod/postgres-0 --timeout=180s
# Apply the chart
make k8s/apply/vps
```

The StatefulSet controller recreates the claim from `volumeClaimTemplates` when the pod
comes back (hence the waiting `--for=condition=ready`)

If the prev job is still there: `kubectl delete job wavelen-migrate --ignore-not-found`

## What usually survives (and should)

- **`wavelen-db`** - created by `kubectl create secret` outside the chart.
- **`wavelen-tls`** - cert-manager does not delete the Secret unless
  `enableCertificateOwnerRef` is on. `cert-manager-values.yaml` does not set it.
- **`pgdata-postgres-0`** - generated from `volumeClaimTemplates`. `kubectl delete -f` and `helm uninstall` won't remove it.
  It's a good thing, deleting it destroys the data.
