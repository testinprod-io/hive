#!/bin/bash
set -exu

# note: geth wants an integer log level (see L1 hive definition)
VERBOSITY=${HIVE_ETH1_LOGLEVEL:-3}

ERIGON_DATA_DIR=/db
ERIGON_CHAINDATA_DIR="$ERIGON_DATA_DIR/geth/chaindata"

CHAIN_ID=$(cat /genesis.json | jq -r .config.chainId)

if [ ! -d "$ERIGON_CHAINDATA_DIR" ]; then
	echo "$ERIGON_CHAINDATA_DIR missing, running init"
	echo "Initializing genesis."
	/usr/local/bin/erigon --log.console.verbosity="$VERBOSITY" init \
		--datadir="$ERIGON_DATA_DIR" \
		"/genesis.json"
else
	echo "$ERIGON_CHAINDATA_DIR exists."
fi

# We must set miner.gaslimit to the gas limit in genesis
# in the command below!
GAS_LIMIT_HEX=$(jq -r .gasLimit < /genesis.json | sed s/0x//i | tr '[:lower:]' '[:upper:]')
GAS_LIMIT=$(echo "obase=10; ibase=16; $GAS_LIMIT_HEX" | bc)

# erigon ALWAYS sets websocket port equal to its http port: 8545,
# while others(op-batcher) try to communicate with erigon via 8546.
# To overcome this issue, proxy requests using socat
socat tcp-l:8546,fork,reuseaddr tcp:127.0.0.1:8545 &
# avoid race
sleep 1

# We check for env variables that may not be bound so we need to disable `set -u` for this section.
EXTRA_FLAGS=""
set +u
if [ "$HIVE_OP_EXEC_DISABLE_TX_GOSSIP" != "" ]; then
    EXTRA_FLAGS="$EXTRA_FLAGS --rollup.disabletxpoolgossip=$HIVE_OP_EXEC_DISABLE_TX_GOSSIP"
else
    EXTRA_FLAGS="$EXTRA_FLAGS --rollup.disabletxpoolgossip=true"
fi
if [ "$HIVE_OP_GETH_SEQUENCER_HTTP" != "" ]; then
    EXTRA_FLAGS="$EXTRA_FLAGS --rollup.sequencerhttp $HIVE_OP_GETH_SEQUENCER_HTTP"
fi
set -u

/usr/local/bin/erigon \
  --datadir="$ERIGON_DATA_DIR" \
  --log.console.verbosity="$VERBOSITY" \
  --http \
  --http.corsdomain="*" \
  --http.vhosts="*" \
  --http.addr=0.0.0.0 \
  --http.port=8545 \
  --http.api=admin,debug,eth,net,txpool,web3 \
  --ws \
  --authrpc.jwtsecret="/hive/input/jwt-secret.txt" \
  --authrpc.port=8551 \
  --authrpc.addr=0.0.0.0 \
  --nodiscover \
  --no-downloader \
  --maxpeers=50 \
  --nat extip:`hostname -i` \
  --miner.gaslimit=$GAS_LIMIT \
  --networkid=$CHAIN_ID \
  $EXTRA_FLAGS \
  "$@"
