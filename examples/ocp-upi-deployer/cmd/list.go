package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// List lists all managed clusters
func List() error {
	fmt.Println("=== Managed Clusters ===")

	clusters, err := listClusters()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	if len(clusters) == 0 {
		fmt.Println("No clusters found.")
		fmt.Printf("Clusters are stored in the '%s' directory.\n", clustersDir)
		return nil
	}

	// Print header
	fmt.Printf("%-20s %-12s %-20s %-20s\n", "CLUSTER NAME", "STATUS", "PHASE", "LAST UPDATED")
	fmt.Println(strings.Repeat("-", 80))

	// List each cluster
	for _, clusterName := range clusters {
		stateFile := getClusterStatePath(clusterName)

		// Try to load state
		state, err := loadStateFile(stateFile)
		if err != nil {
			fmt.Printf("%-20s %-12s %-20s %-20s\n",
				clusterName, "unknown", "N/A", "N/A")
			continue
		}

		// Format timestamp
		timestamp := "N/A"
		if state.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, state.Timestamp); err == nil {
				timestamp = t.Format("2006-01-02 15:04:05")
			}
		}

		fmt.Printf("%-20s %-12s %-20s %-20s\n",
			clusterName,
			state.Status,
			state.CurrentPhase,
			timestamp)
	}

	fmt.Printf("\nTotal clusters: %d\n", len(clusters))
	return nil
}

// loadStateFile loads a state file
func loadStateFile(path string) (*types.DeploymentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state types.DeploymentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Made with Bob
