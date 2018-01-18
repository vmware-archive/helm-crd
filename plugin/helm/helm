#!/bin/sh

set -e
test $KUBECTL_PLUGINS_GLOBAL_FLAG_V -gt 4 && set -x
alias kubectl="$KUBECTL_PLUGINS_CALLER -n $KUBECTL_PLUGINS_CURRENT_NAMESPACE -v=$KUBECTL_PLUGINS_GLOBAL_FLAG_V"

values_yaml() {
    if [ "$KUBECTL_PLUGINS_LOCAL_FLAG_VALUES" != "" ]; then
        cat "$KUBECTL_PLUGINS_LOCAL_FLAG_VALUES"
    fi

    IFS=,; set -- $KUBECTL_PLUGINS_LOCAL_FLAG_SET; IFS=
    for kv; do
        echo "${kv%%=*}: \"${kv#*=}\""
    done
}

json_escape() {
    sed 's/([\"])/\\\1/g'
}

subcommand="$1"; shift
case $subcommand in
    init)
        kubectl apply -n kube-system -f "$KUBECTL_PLUGINS_LOCAL_FLAG_URL"
        ;;

    list)
        kubectl get helmreleases "$@"
        ;;

    install)
        chart="$1"; shift
        {
            cat <<EOF
apiVersion: helm.bitnami.com/v1
kind: HelmRelease
metadata:
  name: $KUBECTL_PLUGINS_LOCAL_FLAG_NAME
  generateName: ${chart}-
spec:
  repoUrl: $KUBECTL_PLUGINS_LOCAL_FLAG_REPO
  chartName: $chart
  version: $KUBECTL_PLUGINS_LOCAL_FLAG_VERSION
  values: |
EOF
            values_yaml | sed 's/^    //'
        } |
            kubectl create -f-
        ;;

    upgrade)
        name="$1"; shift

        values=$(values_yaml)
        q=\"
        patch=$(cat <<EOF
{"metadata": {"annotations": {"helm.bitnami.com/k8s-53379-workaround": "$(date +%s)"}}},
{"spec": {
${KUBECTL_PLUGINS_LOCAL_FLAG_VERSION:+${q}version${q}: ${q}$KUBECTL_PLUGINS_LOCAL_FLAG_VERSION${q},}
${values:+${q}values${q}: ${q}$(echo $values | json_escape)${q},}
"repoUrl": "$KUBECTL_PLUGINS_LOCAL_FLAG_REPO"
}}
EOF
             )
        # NB: --type=strategic is broken for CRDs (k8s v1.8)
        kubectl patch helmrelease $name -p "$patch" --type=merge
        ;;

    delete)
        kubectl delete helmrelease "$@"
        ;;

    *)
        echo "Unknown subcommand: $subcommand" >&2
        exit 1
esac
