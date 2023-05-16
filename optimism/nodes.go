package optimism

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	gethp2p "github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/hive/hivesim"
)

// These ports are exposed on the docker containers, and accessible via the docker network that the hive test runs in.
// These are container-ports: they are not exposed to the host,
// and so multiple containers can use the same port.
// Some eth1 client definitions hardcode them, others make them configurable, these should not be changed.
const (
	HttpRPCPort = 8545
	WsRPCPort   = 8546
	EnginePort  = 8551
	// RollupRPCPort is set to the default EL RPC port,
	// since Hive defaults to RPC / caching / liveness checks on this port.
	RollupRPCPort = 8545
	OpnodeP2PPort = 9300
)

type ELNode struct {
	*hivesim.Client
}

func (e *ELNode) HttpRpcEndpoint() string {
	return fmt.Sprintf("http://%v:%d", e.IP, HttpRPCPort)
}

func (e *ELNode) EngineEndpoint() string {
	return fmt.Sprintf("http://%v:%d", e.IP, EnginePort)
}

func (e *ELNode) WsRpcEndpoint() string {
	// carried over from older mergenet ws connection problems, idk why clients are different
	switch e.Client.Type {
	case "besu":
		return fmt.Sprintf("ws://%v:%d/ws", e.IP, WsRPCPort)
	case "nethermind":
		return fmt.Sprintf("http://%v:%d/ws", e.IP, WsRPCPort) // upgrade
	default:
		return fmt.Sprintf("ws://%v:%d", e.IP, WsRPCPort)
	}
}

func (e *ELNode) EthClient() *ethclient.Client {
	return ethclient.NewClient(e.RPC())
}

type Eth1Node struct {
	ELNode
}

type OpContracts struct {
	*hivesim.Client
}

// OpL2Engine extends ELNode since it has all the same endpoints, except it is used for L2
type OpL2Engine struct {
	ELNode
}

// temporal extenstion of OpL2Engine
// Direct RPC wrapper will be eventually moved to op-erigon or op-geth
type OpL2EngineExtended struct {
	*OpL2Engine
}

func (e *OpL2EngineExtended) NodeInfo(ctx context.Context) (*gethp2p.NodeInfo, error) {
	var output *gethp2p.NodeInfo
	err := e.RPC().CallContext(ctx, &output, "admin_nodeInfo")
	return output, err

}

func (e *OpL2EngineExtended) Peers(ctx context.Context) (*gethp2p.PeerInfo, error) {
	var output *gethp2p.PeerInfo
	err := e.RPC().CallContext(ctx, &output, "admin_peers")
	return output, err
}

func (e *OpL2EngineExtended) AddPeer(ctx context.Context, enode string) (bool, error) {
	var output bool
	err := e.RPC().CallContext(ctx, &output, "admin_addPeer", enode)
	return output, err
}

// only interested in txhash for p2p testing
// will be updated to use op-geth's RPCTransaction struct
type RPCTransactionHash struct {
	Hash common.Hash `json:"hash"`
}

func (e *OpL2EngineExtended) TxPoolContent(ctx context.Context) (map[string]map[string]map[string]*RPCTransactionHash, error) {
	// only return necessary information(tx hash) for testing tx gossip
	var output map[string]map[string]map[string]*RPCTransactionHash
	err := e.RPC().CallContext(ctx, &output, "txpool_content")
	return output, err
}

func (e *OpL2EngineExtended) ConnectPeer(ctx context.Context, neighbor *OpL2EngineExtended) error {
	nodeInfo, err := neighbor.NodeInfo(ctx)
	if err != nil {
		return err
	}
	// sanity check by parsing result
	_, err = enode.Parse(enode.ValidSchemes, nodeInfo.Enode)
	if err != nil {
		return err
	}
	_, err = e.AddPeer(ctx, nodeInfo.Enode)
	if err != nil {
		return err
	}
	return nil
}

type OpNode struct {
	*hivesim.Client
	p2pKey  *ecdsa.PrivateKey
	p2pAddr string
}

func (e *OpNode) HttpRpcEndpoint() string {
	return fmt.Sprintf("http://%v:%d", e.IP, RollupRPCPort)
}

func (e *OpNode) RollupClient() *sources.RollupClient {
	return sources.NewRollupClient(client.NewBaseRPCClient(e.RPC()))
}

func (e *OpNode) P2PClient() *p2p.Client {
	return p2p.NewClient(e.RPC())
}

func (e *OpNode) P2PKey() *ecdsa.PrivateKey {
	return e.p2pKey
}

func (e *OpNode) P2PAddr() string {
	return e.p2pAddr
}

type ProposerNode struct {
	*hivesim.Client
}

type BatcherNode struct {
	*hivesim.Client
}
