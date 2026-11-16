// -------------------------------------------------------------------------------
// Cluster Formation Tests
//
// Project: Wander
//
// The three servers form one healthy Raft cluster and the three clients join
// it with no join addresses configured: servers and clients find each other
// only through the local Consul agent on every node.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

// formationTimeout bounds how long a fresh cluster may take to settle.
const formationTimeout = 2 * time.Minute

// TestCluster_Formed proves the servers form one Raft cluster with a leader,
// three voters, the failure tolerance of three servers, and Wander's 1.6.5
// build tag on every member.
func TestCluster_Formed(t *testing.T) {
	client := servers["wander-server-1"]

	t.Run("raft_peers", func(t *testing.T) {
		eventually(t, formationTimeout, func() error {
			peers, err := client.Status().Peers()
			if err != nil {
				return err
			}
			if len(peers) != 3 {
				return fmt.Errorf("%d raft peers, want 3: %v", len(peers), peers)
			}
			return nil
		})
	})

	t.Run("leader", func(t *testing.T) {
		eventually(t, formationTimeout, func() error {
			leader, err := client.Status().Leader()
			if err != nil {
				return err
			}
			if leader == "" {
				return errors.New("no leader")
			}
			return nil
		})
	})

	t.Run("voters", func(t *testing.T) {
		eventually(t, formationTimeout, func() error {
			cfg, err := client.Operator().RaftGetConfiguration(nil)
			if err != nil {
				return err
			}
			voters := 0
			for _, srv := range cfg.Servers {
				if srv.Voter {
					voters++
				}
			}
			if voters != 3 {
				return fmt.Errorf("%d voters in the raft configuration, want 3", voters)
			}
			return nil
		})
	})

	t.Run("autopilot", func(t *testing.T) {
		eventually(t, formationTimeout, func() error {
			health, _, err := client.Operator().AutopilotServerHealth(nil)
			if err != nil {
				return err
			}
			if !health.Healthy || health.FailureTolerance != 1 {
				return fmt.Errorf("autopilot healthy=%t failure tolerance=%d, want true and 1",
					health.Healthy, health.FailureTolerance)
			}
			return nil
		})
	})

	t.Run("members", func(t *testing.T) {
		eventually(t, formationTimeout, func() error {
			members, err := client.Agent().Members()
			if err != nil {
				return err
			}
			alive := 0
			for _, m := range members.Members {
				if m.Status == "alive" && m.Tags["build"] == "1.6.5" {
					alive++
				}
			}
			if alive != 3 {
				return fmt.Errorf("%d members alive with build 1.6.5, want 3", alive)
			}
			return nil
		})
	})
}

// TestCluster_ClientsReady proves all three clients register as ready and
// eligible, with the docker, exec and raw_exec drivers healthy and the CNI
// bridge plugin fingerprinted.
func TestCluster_ClientsReady(t *testing.T) {
	client := servers["wander-server-1"]

	var nodes []*api.NodeListStub
	eventually(t, formationTimeout, func() error {
		list, _, err := client.Nodes().List(nil)
		if err != nil {
			return err
		}
		ready := 0
		for _, n := range list {
			if n.Status == api.NodeStatusReady && n.SchedulingEligibility == api.NodeSchedulingEligible {
				ready++
			}
		}
		if ready != 3 {
			return fmt.Errorf("%d of %d nodes ready and eligible, want 3", ready, len(list))
		}
		nodes = list
		return nil
	})

	for _, stub := range nodes {
		t.Run(stub.Name, func(t *testing.T) {
			eventually(t, formationTimeout, func() error {
				node, _, err := client.Nodes().Info(stub.ID, nil)
				if err != nil {
					return err
				}
				for _, name := range []string{"docker", "exec", "raw_exec"} {
					if d := node.Drivers[name]; d == nil || !d.Healthy {
						return fmt.Errorf("driver %s is not healthy", name)
					}
				}
				if node.Attributes["plugins.cni.version.bridge"] == "" {
					return errors.New("CNI bridge plugin is not fingerprinted")
				}
				return nil
			})
		})
	}
}
