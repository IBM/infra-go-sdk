package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/cmd"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/orchestrator"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

const version = "0.1.0"

func main() {
	// Define command-line flags
	command := flag.String("command", "", "Command to execute: deploy, delete, validate, status, list, version")
	configFile := flag.String("config", "config.yaml", "Path to multi-cluster configuration file")
	clusterName := flag.String("cluster", "", "Cluster name (optional, deploys all if not specified)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	resume := flag.Bool("resume", false, "Resume deployment from last failed phase")
	flag.Parse()

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Handle version command
	if *command == "version" {
		fmt.Printf("OpenShift UPI Deployer v%s\n", version)
		fmt.Println("A tool for deploying OpenShift clusters on IBM Power Systems")
		os.Exit(0)
	}

	// Handle help command
	if *command == "help" || *command == "-h" || *command == "--help" {
		printUsage()
		os.Exit(0)
	}

	// Validate command
	if *command == "" {
		printUsage()
		os.Exit(1)
	}

	// List command doesn't need config file from current directory
	if *command == "list" {
		if err := cmd.List(); err != nil {
			fmt.Printf("\nError: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Status command loads config from cluster directory
	if *command == "status" {
		if *clusterName == "" {
			fmt.Println("Error: cluster name is required for status command")
			os.Exit(1)
		}
		if err := cmd.StatusFromClusterDir(*clusterName); err != nil {
			fmt.Printf("\nError: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Delete command loads config from cluster directory
	if *command == "delete" {
		if *clusterName == "" {
			fmt.Println("Error: cluster name is required for delete command")
			os.Exit(1)
		}
		if err := cmd.DeleteFromClusterDir(*clusterName); err != nil {
			fmt.Printf("\nError: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Resume deployment loads config from cluster directory
	if *command == "deploy" && *resume {
		if *clusterName == "" {
			fmt.Println("Error: cluster name is required for resume")
			os.Exit(1)
		}
		if err := cmd.ResumeFromClusterDir(*clusterName); err != nil {
			fmt.Printf("\nError: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load configuration for other commands (deploy without resume, validate)
	config, err := types.LoadMultiClusterConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create orchestrator
	orch, err := orchestrator.NewOrchestrator(config, *verbose)
	if err != nil {
		log.Fatalf("Failed to create orchestrator: %v", err)
	}

	// ---------------------------------------------------------
	// Graceful Shutdown & Signal Handling
	// ---------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n\n[WARNING] Interrupt signal received! Attempting graceful shutdown...")
		fmt.Println("Closing SSH connections to abort current tasks and release state locks. Please wait...")

		// Closing the orchestrator drops the SSH connection.
		// Any blocked ExecuteCommand will instantly fail, allowing the program
		// to safely trigger defer statements (like UnlockState) before exiting.
		orch.Close()
		cancel()
	}()

	// Execute CLI logic
	if err := runCLI(ctx, orch, config, *configFile, *command, *clusterName, *resume); err != nil {
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}
}

// runCLI routes the command execution and safely returns errors instead of using log.Fatalf
func runCLI(ctx context.Context, orch *orchestrator.Orchestrator, config *types.MultiClusterConfig, configFile, command, clusterName string, resume bool) error {
	switch command {
	case "validate":
		return cmd.Validate(orch, config, clusterName)
	case "deploy":
		return cmd.Deploy(orch, config, configFile, clusterName, resume)
	case "delete":
		return cmd.Delete(orch, config, clusterName)
	case "status":
		return cmd.Status(orch, clusterName)
	case "list":
		return cmd.List()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		return fmt.Errorf("invalid command provided")
	}
}

// printUsage prints usage information
func printUsage() {
	fmt.Println("OpenShift UPI Deployer - Deploy OpenShift clusters on IBM Power Systems")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ocp-upi-deployer -command <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  validate    Validate cluster configuration(s)")
	fmt.Println("  deploy      Deploy cluster(s)")
	fmt.Println("  delete      Delete a cluster")
	fmt.Println("  status      Show cluster deployment status")
	fmt.Println("  list        List all managed clusters")
	fmt.Println("  version     Show version information")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config string")
	fmt.Println("        Path to multi-cluster configuration file (default: config.yaml)")
	fmt.Println("  -cluster string")
	fmt.Println("        Cluster name (optional for validate/deploy, required for delete/status)")
	fmt.Println("  -verbose")
	fmt.Println("        Enable verbose output")
	fmt.Println("  -resume")
	fmt.Println("        Resume deployment from last failed phase")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Validate all clusters")
	fmt.Println("  ocp-upi-deployer -command validate -config config.yaml")
	fmt.Println()
	fmt.Println("  # Deploy specific cluster")
	fmt.Println("  ocp-upi-deployer -command deploy -config config.yaml -cluster ocp-sno")
	fmt.Println()
	fmt.Println("  # Resume failed deployment (loads config from cluster directory)")
	fmt.Println("  ocp-upi-deployer -command deploy -cluster ocp-sno -resume")
	fmt.Println()
	fmt.Println("  # List all managed clusters")
	fmt.Println("  ocp-upi-deployer -command list")
	fmt.Println()
	fmt.Println("  # Show cluster status (loads config from cluster directory)")
	fmt.Println("  ocp-upi-deployer -command status -cluster ocp-sno")
	fmt.Println()
	fmt.Println("  # Delete cluster (loads config from cluster directory)")
	fmt.Println("  ocp-upi-deployer -command delete -cluster ocp-sno")
	fmt.Println()
}

// Made with Bob
