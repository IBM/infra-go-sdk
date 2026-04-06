package cmd

import (
	"fmt"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/orchestrator"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// Deploy deploys cluster(s)
func Deploy(orch *orchestrator.Orchestrator, config *types.MultiClusterConfig, configFile, clusterName string, resume bool) error {
	if resume {
		fmt.Println("=== Resuming Deployment ===")
	} else {
		fmt.Println("=== Starting Deployment ===")
	}

	// Initialize connections
	if err := orch.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}
	defer orch.Close()

	clusters := config.Clusters
	if clusterName != "" {
		// Deploy specific cluster
		cluster := findCluster(config, clusterName)
		if cluster == nil {
			return fmt.Errorf("cluster '%s' not found in configuration", clusterName)
		}
		clusters = []types.ClusterRef{*cluster}
	}

	// Deploy each cluster
	for _, clusterRef := range clusters {
		// Create cluster directory
		if err := ensureClusterDir(clusterRef.Name); err != nil {
			return fmt.Errorf("failed to create cluster directory for %s: %w", clusterRef.Name, err)
		}

		// Copy config file to cluster directory (only for new deployments)
		if !resume {
			destConfig := getClusterConfigPath(clusterRef.Name)
			if err := copyFile(configFile, destConfig); err != nil {
				return fmt.Errorf("failed to copy config to cluster directory: %w", err)
			}
			fmt.Printf("✓ Config copied to %s\n", destConfig)
		}

		if resume {
			if err := orch.ResumeCluster(clusterRef); err != nil {
				return fmt.Errorf("failed to resume cluster %s: %w", clusterRef.Name, err)
			}
		} else {
			if err := orch.DeployCluster(clusterRef); err != nil {
				return fmt.Errorf("failed to deploy cluster %s: %w", clusterRef.Name, err)
			}
		}
	}

	fmt.Println("\n=== Deployment Completed Successfully ===")
	return nil
}

// ResumeFromClusterDir resumes a deployment by loading config from cluster directory
func ResumeFromClusterDir(clusterName string) error {
	// Check if cluster directory exists
	if !clusterDirExists(clusterName) {
		return fmt.Errorf("cluster '%s' not found. Use 'list' command to see all clusters", clusterName)
	}

	fmt.Println("=== Resuming Deployment ===")

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

	// Initialize connections
	if err := orch.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}

	// Find the cluster in config
	cluster := findCluster(config, clusterName)
	if cluster == nil {
		return fmt.Errorf("cluster '%s' not found in config", clusterName)
	}

	// Resume the cluster deployment
	fmt.Printf("\n=== Resuming Cluster: %s ===\n", cluster.Name)

	if err := orch.ResumeCluster(*cluster); err != nil {
		return fmt.Errorf("failed to resume cluster %s: %w", cluster.Name, err)
	}

	fmt.Printf("\n=== Cluster '%s' Deployment Resumed Successfully ===\n", cluster.Name)
	return nil
}

// Made with Bob
