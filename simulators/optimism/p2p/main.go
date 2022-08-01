package main

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-proposer/rollupclient"
	"github.com/stretchr/testify/assert"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
)

func main() {
	suite := hivesim.Suite{
		Name:        "optimism p2p",
		Description: "This suite runs the P2P protocol tests",
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "simple p2p testnet",
		Description: `This test run.`,
		Run:         func(t *hivesim.T) { runP2PTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runP2PTests runs the P2P tests between the sequencer and verifier.
func runP2PTests(t *hivesim.T) {
	handleErr := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	d := optimism.NewDevnet(t)

	d.InitContracts()
	d.InitRollupHardhat()
	d.AddEth1()
	// wait for L1 to come online before deploying
	{
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_, err := d.GetEth1(0).EthClient().ChainID(ctx)
		handleErr(err)
	}
	d.DeployL1Hardhat()

	// sequencer stack, on top of first eth1 node
	d.AddOpL2()
	d.AddOpNode(0, 0)
	d.AddOpBatcher(0, 0, 0)
	d.AddOpProposer(0, 0, 0)

	// TODO: pass optimism.UnprefixedParams{flag env vars here}.Params()
	//  hivesim start option to the op nodes to configure p2p networking

	// verifier A
	d.AddOpL2()
	d.AddOpNode(0, 1) // we attach to the same L1 node, so we don't need to configure L1 networking.

	// verifier B
	d.AddOpL2()
	d.AddOpNode(0, 2)

	t.Log("waiting for nodes to get onto the network")
	time.Sleep(time.Second * 10)

	seq := d.GetOpNode(0)
	verifA := d.GetOpNode(1)
	verifB := d.GetOpNode(2)

	seqCl := seq.RollupClient()
	verifACl := verifA.RollupClient()
	verifBCl := verifB.RollupClient()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(time.Second * 4)
		defer ticker.Stop()

		syncStat := func(name string, cl *rollupclient.RollupClient) *driver.SyncStatus {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
			seqStat, err := cl.SyncStatus(ctx)
			cancel()
			if err != nil {
				t.Error("failed to get sync status from %s op-node: %v", name, err)
			}
			return seqStat // may be nil
		}

		for {
			select {
			case <-ticker.C:
				// Check that all clients are synced
				seqStat := syncStat("sequencer", seqCl)
				verAStat := syncStat("verifier-A", verifACl)
				verBStat := syncStat("verifier-B", verifBCl)
				assert.Equal(t, seqStat, verAStat, "sequencer and verifier A should be synced")
				assert.Equal(t, verAStat, verBStat, "verifier A and verifier B should be synced")
			case <-ctx.Done():
				t.Log("exiting sync checking loop")
				return
			}
		}
	}()

	// Run testnet for duration of 3 sequence windows
	time.Sleep(time.Second * time.Duration(d.L1Cfg.Clique.Period*d.RollupCfg.SeqWindowSize*3))
	cancel()

	// TODO: Add P2P tests
}