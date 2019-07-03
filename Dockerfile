FROM golang:1.9 as gobuild
WORKDIR /go/src/github.com/bitnami-labs/helm-crd/
COPY . .
RUN make controller-static

FROM bitnami/minideb:stretch
RUN install_packages ca-certificates
COPY --from=gobuild /go/src/github.com/bitnami-labs/helm-crd/controller-static /controller
CMD ["/controller"]
