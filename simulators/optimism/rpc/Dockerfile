# This simulation runs JSON-RPC API tests.
FROM golang:1.18-alpine3.15 as builder
RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable, from hive repo root.
# See context.txt for docker build context change.
# We use a go.work file to pull in other go modules of the hive repo.
ADD ./simulators/optimism/rpc/hive.go.work /source/go.work
ADD ./optimism /source/optimism
ADD ./simulators/optimism/rpc /source/rpc

ENV CGO_ENABLED=1

WORKDIR /source/rpc
RUN go build -gcflags="all=-N -l" -v .

RUN go install github.com/go-delve/delve/cmd/dlv@latest

EXPOSE 12345

ENTRYPOINT ["/go/bin/dlv", "--listen=:12345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "./rpc"]

# Build the simulator run container.
#FROM alpine:latest
#
#COPY --from=builder /source/rpc/rpc .
#ENTRYPOINT ["./rpc"]
