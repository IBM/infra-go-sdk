package cmd

import (
	"fmt"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/orchestrator"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/validation"
)

// Validate validates cluster configuration(s)
func Validate(orch *orchestrator.Orchestrator, config *types.MultiClusterConfig, clusterName string) error {
	fmt.Println("=== Validating Configuration ===")

	clusters := config.Clusters
	if clusterName != "" {
		// Validate specific cluster
		cluster := findCluster(config, clusterName)
		if cluster == nil {
			return fmt.Errorf("cluster '%s' not found in configuration", clusterName)
		}
		clusters = []types.ClusterRef{*cluster}
	}

	for _, clusterRef := range clusters {
		fmt.Printf("Validating cluster: %s\n", clusterRef.Name)

		// Load cluster-specific configuration
		clusterConfig, err := clusterRef.GetClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to load cluster config for %s: %w", clusterRef.Name, err)
		}

		// Create validator with SSH client (will be nil here since orchestrator isn't initialized, safely skipping remote checks)
		validator := validation.NewValidator(config, clusterConfig, clusterRef.Name, nil, false)
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("validation failed for cluster %s: %w", clusterRef.Name, err)
		}

		fmt.Printf("✓ Cluster '%s' configuration is valid\n\n", clusterRef.Name)
	}

	fmt.Println("=== All Validations Passed ===")
	return nil
}

// Made with Bob
