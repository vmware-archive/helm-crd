#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

### Workaround for issue: https://github.com/kubernetes/code-generator/issues/6
mkdir -p ${GOPATH}/src/k8s.io/kubernetes/hack/boilerplate 
cp ${SCRIPT_ROOT}/hack/boilerplate.go.txt ${GOPATH}/src/k8s.io/kubernetes/hack/boilerplate/

${CODEGEN_PKG}/generate-groups.sh \
    all \
    github.com/bitnami-labs/helm-crd/pkg/client \
    github.com/bitnami-labs/helm-crd/pkg/apis \
    helm.bitnami.com:v1
