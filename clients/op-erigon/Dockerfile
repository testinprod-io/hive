FROM testinprod/op-erigon:latest
# If image not on dockerhub, clone https://github.com/testinprod-io/erigon
# and build locally. Run `docker build . op-erigon/v1.2` at root of erigon

# The upstream erigon container uses a non-root user, but we need
# to install additional commands, so switch back to root.
USER root

# socat for routing websocket requests
RUN apk add --update bash curl jq socat tar
RUN apk add --no-cache ca-certificates
RUN /usr/local/bin/erigon --version | sed -e 's/erigon version \(.*\)/\1/' > /version.txt

ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8545 8546 8551 30303 30303/udp

ENTRYPOINT ["/entrypoint.sh"]
