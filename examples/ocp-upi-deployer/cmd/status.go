package cmd

import (
	"fmt"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/orchestrator"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// Status shows deployment status
func Status(orch *orchestrator.Orchestrator, clusterName string) error {
	if clusterName == "" {
		return fmt.Errorf("cluster name is required for status command")
	}

	fmt.Printf("=== Cluster Status: %s ===\n\n", clusterName)

	status, err := orch.GetClusterStatus(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	fmt.Println(status)
	return nil
}

// StatusFromClusterDir shows deployment status by loading config from cluster directory
func StatusFromClusterDir(clusterName string) error {
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

	// Get and display status
	fmt.Printf("=== Cluster Status: %s ===\n\n", clusterName)

	status, err := orch.GetClusterStatus(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	// Print the formatted status string
	fmt.Println(status)

	return nil
}


// Made with Bob
