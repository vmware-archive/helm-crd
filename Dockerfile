FROM golang:1.9 as gobuild
WORKDIR /go/src/github.com/bitnami/helm-crd/
COPY . .
RUN make controller-static

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=gobuild /go/src/github.com/bitnami/helm-crd/controller-static /controller
CMD ["/controller"]
