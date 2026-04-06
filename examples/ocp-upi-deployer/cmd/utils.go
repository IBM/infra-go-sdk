package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

const (
	clustersDir       = "clusters"
	clusterConfigFile = "config.yaml"
	clusterStateFile  = "state.json"
)

// getClusterDir returns the directory path for a cluster
func getClusterDir(clusterName string) string {
	return filepath.Join(clustersDir, clusterName)
}

// ensureClusterDir creates the cluster directory if it doesn't exist
func ensureClusterDir(clusterName string) error {
	dir := getClusterDir(clusterName)
	return os.MkdirAll(dir, 0755)
}

// getClusterConfigPath returns the path to the cluster's config file
func getClusterConfigPath(clusterName string) string {
	return filepath.Join(getClusterDir(clusterName), clusterConfigFile)
}

// getClusterStatePath returns the path to the cluster's state file
func getClusterStatePath(clusterName string) string {
	return filepath.Join(getClusterDir(clusterName), clusterStateFile)
}

// clusterDirExists checks if a cluster directory exists
func clusterDirExists(clusterName string) bool {
	dir := getClusterDir(clusterName)
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return destFile.Sync()
}

// listClusters returns a list of all cluster directories
func listClusters() ([]string, error) {
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read clusters directory: %w", err)
	}

	var clusters []string
	for _, entry := range entries {
		if entry.IsDir() {
			clusters = append(clusters, entry.Name())
		}
	}

	return clusters, nil
}

// findCluster finds a cluster by name in the configuration
func findCluster(config *types.MultiClusterConfig, name string) *types.ClusterRef {
	for i := range config.Clusters {
		if config.Clusters[i].Name == name {
			return &config.Clusters[i]
		}
	}
	return nil
}

// Made with Bob
