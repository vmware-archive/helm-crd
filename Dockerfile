FROM golang:1.9 as gobuild
WORKDIR /go/src/github.com/bitnami-labs/helm-crd/
COPY . .
RUN make controller-static

FROM alpine:3.6
RUN apk --no-cache add ca-certificates
COPY --from=gobuild /go/src/github.com/bitnami-labs/helm-crd/controller-static /controller
CMD ["/controller"]
