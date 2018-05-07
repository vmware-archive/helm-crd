# CustomResourceDefinition for Helm

This is an **experimental** CRD controller for Helm releases.

You can use this to install, upgrade and delete charts in your cluster
via regular Kubernetes API objects that look like:

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

```
d=$HOME/.kube/plugins/helm
mkdir -p $d; cd $d
wget \
 https://raw.githubusercontent.com/bitnami-labs/helm-crd/master/plugin/helm/helm \
 https://raw.githubusercontent.com/bitnami-labs/helm-crd/master/plugin/helm/plugin.yaml
chmod +x helm
```

You now have a new kubectl plugin!  See `kubectl plugin helm --help`
for the new subcommands.

Run `kubectl plugin helm init` to perform the server-side install.

### Server-side only

If you don't want (or need) the kubectl plugin, you can install the
server-side components directly with:

```
kubectl apply -n kube-system -f deploy/tiller-crd.yaml
```

This will create the CRD, and replace(!) any existing
`kube-system/tiller-deploy` with an unmodified tiller v2.9.0 release
with the tiller port restricted *and* a new `controller` sidecar.

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

### Does this prevent `helm` CLI usage?

No.  The `helm` CLI tool accesses tiller using a Kubernetes port-forward
into the tiller pod, and this is unaffected by the presence of this
controller.  Port forward access can be blocked via RBAC policy
(`pods/portforward` resource) if desired - and leaves HelmRelease resource
objects as the only way to access tiller functionality.
