# CustomResourceDefinition for Helm

This is an **experimental** CRD controller for Helm charts.

You can use this to manage Helm charts (releases) in your cluster via
regular Kubernetes API objects that look like:

```yaml
apiVersion: helm.bitnami.com/v1
kind: HelmRelease
metadata:
  name: mydb
spec:
  # 'stable' repo
  repoUrl: https://kubernetes-charts.storage.googleapis.com
  chartName: mariadb
  version: 2.0.1
  values: |
    mariadbDatabase: mydb
    mariadbPassword: sekret
    mariadbRootPassword: supersekret
    mariadbUser: myuser
```

## Advantages:

- **Familiar.** Integrates well with other tools like `kubectl
  apply`, [kubecfg] and declarative/gitops workflows.

- **More secure.** `HelmRelease` objects can be restricted via RBAC
  policy, including limiting access by namespace.

[kubecfg]: https://github.com/ksonnet/kubecfg

## Install:

`kubectl apply -n kube-system -f deploy/tiller-crd.yaml`

This will create the CRD, and replace(!) any existing
`kube-system/tiller-deploy` with an unmodified tiller v2.7.0 release
*and* a new `controller` sidecar.

To use, start creating API objects similar to the example above.

## FAQ

### Does this replace `helm` CLI tool?

Only for the _most basic_ operations.  If you just want to consume
(install/remove/update) public upstream charts, then _yes_.

If you want to do anything else then _no_, you will still need `helm`.
In particular, installing a chart from local disk, or _developing_ a
chart requires `helm` and is unchanged with this controller.

### Does this affect upstream chart development?

No.  Charts themselves are unchanged, and chart development workflow
and tools remains the same as before.  In particular, developing
charts still requires use of the `helm` CLI tool and "port forward"
access to the tiller container.

Note that this last point implies full access over the cluster used
for development (just as before this controller) and chart development
is best performed against dedicated and disposable test clusters
(minikube, etc).
