package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/orchestrator"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// Delete deletes cluster(s)
func Delete(orch *orchestrator.Orchestrator, config *types.MultiClusterConfig, clusterName string) error {
	fmt.Println("=== Starting Deletion ===")

	// Initialize connections
	if err := orch.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}
	defer orch.Close()

	clusters := config.Clusters
	if clusterName != "" {
		// Delete specific cluster
		cluster := findCluster(config, clusterName)
		if cluster == nil {
			return fmt.Errorf("cluster '%s' not found in configuration", clusterName)
		}
		clusters = []types.ClusterRef{*cluster}
	} else {
		fmt.Printf("⚠️ WARNING: No specific cluster provided. This will delete ALL %d clusters defined in config.yaml!\n", len(clusters))
		// Optional: You could add a prompt here in the future asking for a "Y/N" confirmation.
	}

	// Delete each cluster
	for _, clusterRef := range clusters {
		fmt.Printf("\n=== Deleting Cluster: %s ===\n", clusterRef.Name)

		// Load cluster configuration
		clusterConfig, err := clusterRef.GetClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to load cluster config for %s: %w", clusterRef.Name, err)
		}

		// Load deployment state from cluster directory
		stateFile := getClusterStatePath(clusterRef.Name)

		state, err := orch.LoadState(stateFile)
		if err != nil {
			fmt.Printf("Warning: Could not load state file for %s: %v\n", clusterRef.Name, err)
			fmt.Println("Proceeding with cleanup based on configuration...")
			state = &types.DeploymentState{
				ClusterName:    clusterRef.Name,
				CreatedLPARs:   make(map[string]types.LPARState),
				CreatedVolumes: make(map[string]types.VolumeState),
			}
		}

		// Create cluster context
		ctx := &types.ClusterContext{
			Name:          clusterRef.Name,
			Type:          clusterRef.Type,
			OCPVersion:    clusterRef.OCPVersion,
			VIP:           clusterRef.VIP,
			ClusterConfig: clusterConfig,
			HelperNode:    config.HelperNode,
			HMC:           config.HMC,
			State:         state,
			Verbose:       orch.GetVerbose(),
		}

		// Create the dedicated Deleter and run cleanup
		deleter := orchestrator.NewClusterDeleter(orch.GetHMCClient(), orch.GetSSHClient(), orch, orch.GetVerbose())
		if err := deleter.CleanupCluster(ctx); err != nil {
			return fmt.Errorf("failed to cleanup cluster %s: %w", clusterRef.Name, err)
		}

		// Ask if user wants to remove cluster directory
		fmt.Printf("\nRemove cluster directory '%s'? [y/N]: ", getClusterDir(clusterRef.Name))
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			clusterDir := getClusterDir(clusterRef.Name)
			if err := os.RemoveAll(clusterDir); err != nil {
				fmt.Printf("Warning: Failed to remove cluster directory: %v\n", err)
			} else {
				fmt.Printf("✓ Cluster directory removed: %s\n", clusterDir)
			}
		} else {
			fmt.Printf("Cluster directory preserved: %s\n", getClusterDir(clusterRef.Name))
		}

		fmt.Printf("\n=== Cluster '%s' Deleted Successfully ===\n", clusterRef.Name)
	}

	return nil
}

// DeleteFromClusterDir deletes a cluster by loading config from cluster directory
func DeleteFromClusterDir(clusterName string) error {
	// Check if cluster directory exists
	if !clusterDirExists(clusterName) {
		return fmt.Errorf("cluster '%s' not found. Use 'list' command to see all clusters", clusterName)
	}

	// Load config from cluster directory
	configPath := getClusterConfigPath(clusterName)
	config, err := types.LoadMultiClusterConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load cluster config from %s: %w", configPath, err)
	}

	// Create orchestrator
	orch, err := orchestrator.NewOrchestrator(config, false)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Initialize connections to HMC and helper node
	if err := orch.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}

	// Find the cluster in config
	var clusterRef *types.ClusterRef
	for i := range config.Clusters {
		if config.Clusters[i].Name == clusterName {
			clusterRef = &config.Clusters[i]
			break
		}
	}

	if clusterRef == nil {
		return fmt.Errorf("cluster '%s' not found in config", clusterName)
	}

	fmt.Printf("\n=== Deleting Cluster: %s ===\n", clusterRef.Name)

	// Load cluster configuration
	clusterConfig, err := clusterRef.GetClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to load cluster config for %s: %w", clusterRef.Name, err)
	}

	// Load deployment state from cluster directory
	stateFile := getClusterStatePath(clusterRef.Name)

	state, err := orch.LoadState(stateFile)
	if err != nil {
		fmt.Printf("Warning: Could not load state file for %s: %v\n", clusterRef.Name, err)
		fmt.Println("Proceeding with cleanup based on configuration...")
		state = &types.DeploymentState{
			ClusterName:    clusterRef.Name,
			CreatedLPARs:   make(map[string]types.LPARState),
			CreatedVolumes: make(map[string]types.VolumeState),
		}
	}

	// Create cluster context
	ctx := &types.ClusterContext{
		Name:          clusterRef.Name,
		Type:          clusterRef.Type,
		OCPVersion:    clusterRef.OCPVersion,
		VIP:           clusterRef.VIP,
		ClusterConfig: clusterConfig,
		HelperNode:    config.HelperNode,
		HMC:           config.HMC,
		State:         state,
		Verbose:       orch.GetVerbose(),
	}

	// Create the dedicated Deleter and run cleanup
	deleter := orchestrator.NewClusterDeleter(orch.GetHMCClient(), orch.GetSSHClient(), orch, orch.GetVerbose())
	if err := deleter.CleanupCluster(ctx); err != nil {
		return fmt.Errorf("failed to cleanup cluster %s: %w", clusterRef.Name, err)
	}

	// Ask if user wants to remove cluster directory
	fmt.Printf("\nRemove cluster directory '%s'? [y/N]: ", getClusterDir(clusterRef.Name))
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) == "y" {
		clusterDir := getClusterDir(clusterRef.Name)
		if err := os.RemoveAll(clusterDir); err != nil {
			fmt.Printf("Warning: Failed to remove cluster directory: %v\n", err)
		} else {
			fmt.Printf("Cluster directory removed: %s\n", clusterDir)
		}
	} else {
		fmt.Printf("Cluster directory preserved: %s\n", getClusterDir(clusterRef.Name))
	}

	fmt.Printf("\n=== Cluster '%s' Deleted Successfully ===\n", clusterRef.Name)
	return nil
}

// Made with Bob
