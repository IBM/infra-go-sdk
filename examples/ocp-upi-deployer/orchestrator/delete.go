package orchestrator

import (
	"fmt"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/infrastructure"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/services"
	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// ClusterDeleter handles the complete teardown of a deployed cluster
type ClusterDeleter struct {
	hmcClient      *hmc.HmcRestClient
	sshClient      *communication.SSHClient
	networkManager *infrastructure.NetworkManager
	orchestrator   *Orchestrator // Needed to use the LockState/UnlockState methods
	verbose        bool
}

// NewClusterDeleter creates a new instance of the deletion manager
func NewClusterDeleter(hmcClient *hmc.HmcRestClient, sshClient *communication.SSHClient, orch *Orchestrator, verbose bool) *ClusterDeleter {
	return &ClusterDeleter{
		hmcClient:      hmcClient,
		sshClient:      sshClient,
		networkManager: infrastructure.NewNetworkManager(sshClient, verbose),
		orchestrator:   orch,
		verbose:        verbose,
	}
}

// CleanupCluster cleans up a cluster deployment using strictly the state file
func (d *ClusterDeleter) CleanupCluster(ctx *ClusterContext) error {
	fmt.Printf("\nCleaning up cluster: %s using state file...\n", ctx.Name)

	if err := d.orchestrator.LockState(ctx); err != nil {
		return fmt.Errorf("cannot start cleanup: %w", err)
	}
	defer d.orchestrator.UnlockState(ctx)

	// 1. Delete LPARs and Storage using State
	lparProvisioner := infrastructure.NewLPARProvisioner(ctx, d.hmcClient)
	if err := lparProvisioner.DeleteAll(); err != nil {
		fmt.Printf("Warning: Failed to delete LPARs and Storage: %v\n", err)
	}

	// 2. Remove dnsmasq configuration files explicitly
	fmt.Println("Removing dnsmasq configuration files...")
	dnsmasqGen := services.NewDNSmasqGenerator(ctx, d.verbose)
	if err := dnsmasqGen.Cleanup(d.sshClient); err != nil {
		fmt.Printf("Warning: Failed to clean dnsmasq configs: %v\n", err)
	}

	// 3. Remove other Helper Node Artifacts using State
	fmt.Println("Removing other Helper Node artifacts...")
	if ctx.State != nil && len(ctx.State.HelperFiles) > 0 {
		for _, fileOrDir := range ctx.State.HelperFiles {
			fmt.Printf("  Removing: %s\n", fileOrDir)
			d.sshClient.ExecuteCommand(fmt.Sprintf("sudo rm -rf %s", fileOrDir))
		}
		// Clear the files from state
		ctx.State.HelperFiles = []string{}
	} else {
		fmt.Println("  ℹ No Helper Node artifacts found in state file.")
	}

	// 4. Clean up dnsmasq lease file
	fmt.Println("Cleaning up dnsmasq lease entries...")
	if err := dnsmasqGen.CleanupLeases(d.sshClient); err != nil {
		fmt.Printf("Warning: Failed to clean dnsmasq leases: %v\n", err)
	}

	// 5. Restart Services to apply deletions
	fmt.Println("Restarting services to flush deleted configurations...")
	if err := d.sshClient.SystemctlRestart("haproxy"); err != nil {
		fmt.Printf("Warning: Failed to restart haproxy: %v\n", err)
	}
	if err := d.sshClient.SystemctlRestart("dnsmasq"); err != nil {
		fmt.Printf("Warning: Failed to restart dnsmasq: %v\n", err)
	}

	// 6. Remove IP aliases
	fmt.Println("Removing IP aliases...")
	if err := d.removeIPAliases(ctx); err != nil {
		fmt.Printf("Warning: Failed to remove IP aliases: %v\n", err)
	}

	fmt.Printf("✓ Cluster cleanup completed: %s\n", ctx.Name)
	return nil
}

// removeIPAliases removes IP aliases for cluster VIPs
func (d *ClusterDeleter) removeIPAliases(ctx *ClusterContext) error {
	iface := ctx.HelperNode.NetworkInterface

	// Convert CIDR to netmask for deletion
	netmask := infrastructure.CidrToNetmask(ctx.ClusterConfig.Network.NetworkCIDR)

	// Use NetworkManager to remove VIP and persistent configuration
	if err := d.networkManager.RemoveVIPAlias(iface, ctx.VIP, netmask, ctx.Name); err != nil {
		fmt.Printf("Warning: Failed to remove VIP: %v\n", err)
	}

	return nil
}
