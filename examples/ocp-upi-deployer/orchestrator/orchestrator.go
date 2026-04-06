package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/infrastructure"
	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// ============================================================================
// CORE ORCHESTRATION
// ============================================================================
// Phase implementations are in orchestrator_phases.go
// Utility functions are in orchestrator_utils.go

// Orchestrator coordinates the entire deployment workflow
type Orchestrator struct {
	config         *MultiClusterConfig
	hmcClient      *hmc.HmcRestClient
	sshClient      *communication.SSHClient
	networkManager *infrastructure.NetworkManager
	verbose        bool
}

// Getter methods for accessing private fields
func (o *Orchestrator) GetHMCClient() *hmc.HmcRestClient {
	return o.hmcClient
}

func (o *Orchestrator) GetSSHClient() *communication.SSHClient {
	return o.sshClient
}

func (o *Orchestrator) GetVerbose() bool {
	return o.verbose
}

// NewOrchestrator creates a new deployment orchestrator
func NewOrchestrator(config *MultiClusterConfig, verbose bool) (*Orchestrator, error) {
	return &Orchestrator{
		config:  config,
		verbose: verbose,
	}, nil
}

// Initialize sets up connections to HMC and helper node
func (o *Orchestrator) Initialize() error {
	fmt.Println("Initializing orchestrator...")

	// Connect to HMC
	fmt.Printf("Connecting to HMC at %s...\n", o.config.HMC.IP)
	o.hmcClient = hmc.NewHmcRestClient(o.config.HMC.IP)
	if err := o.hmcClient.Login(o.config.HMC.Username, o.config.HMC.Password, o.verbose); err != nil {
		return fmt.Errorf("failed to connect to HMC: %w", err)
	}
	fmt.Println("✓ Connected to HMC")

	// Connect to helper node
	fmt.Printf("Connecting to helper node at %s...\n", o.config.HelperNode.IP)
	sshClient := communication.NewSSHClient(
		o.config.HelperNode.IP,
		o.config.HelperNode.SSHUser,
		o.config.HelperNode.SSHKeyFile,
		o.verbose,
	)
	if err := sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to helper node: %w", err)
	}
	o.sshClient = sshClient
	fmt.Println("✓ Connected to helper node")

	// Initialize network manager
	o.networkManager = infrastructure.NewNetworkManager(o.sshClient, o.verbose)

	return nil
}

// Close cleans up connections
func (o *Orchestrator) Close() error {
	if o.sshClient != nil {
		return o.sshClient.Close()
	}
	return nil
}

// LockState creates a lock file to prevent concurrent executions
func (o *Orchestrator) LockState(ctx *ClusterContext) error {
	stateFile := GetStateFilePath(ctx.Name)
	lockFile := stateFile + ".lock"

	// os.O_EXCL ensures we only open the file if it doesn't already exist
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("state is locked by another process (lock file exists: %s). If this is an error, delete the lock file manually", lockFile)
		}
		return fmt.Errorf("failed to create state lock: %w", err)
	}
	defer f.Close()

	lockInfo := fmt.Sprintf("Locked at %s by PID %d", time.Now().Format(time.RFC3339), os.Getpid())
	f.WriteString(lockInfo)

	if o.verbose {
		fmt.Printf("State locked successfully: %s\n", lockFile)
	}
	return nil
}

// UnlockState removes the lock file
func (o *Orchestrator) UnlockState(ctx *ClusterContext) {
	stateFile := GetStateFilePath(ctx.Name)
	lockFile := stateFile + ".lock"

	if err := os.Remove(lockFile); err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: Failed to remove state lock file: %v\n", err)
		}
	} else if o.verbose {
		fmt.Printf("State unlocked successfully: %s\n", lockFile)
	}
}

// DeployCluster deploys a single cluster
func (o *Orchestrator) DeployCluster(clusterRef ClusterRef) error {
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("DEPLOYING CLUSTER: %s (%s)\n", clusterRef.Name, clusterRef.Type)
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")

	// Load cluster-specific configuration
	clusterConfig, err := clusterRef.GetClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Create cluster context
	ctx := &ClusterContext{
		Name:          clusterRef.Name,
		Type:          clusterRef.Type,
		OCPVersion:    clusterRef.OCPVersion,
		VIP:           clusterRef.VIP,
		ClusterConfig: clusterConfig,
		HelperNode:    o.config.HelperNode,
		HMC:           o.config.HMC,
		Verbose:       o.verbose,
		State: &DeploymentState{
			StateVersion:    1,
			ClusterName:     clusterRef.Name,
			CurrentPhase:    "initializing",
			CompletedPhases: []string{},
			PhaseHistory:    []PhaseExecution{},
			CreatedLPARs:    make(map[string]LPARState),
			CreatedVolumes:  make(map[string]VolumeState),
			IPAliases:       []IPAliasState{},
			ServiceEndpoints: ServiceEndpoints{
				HTTPServerURL: fmt.Sprintf("http://%s:8080/%s", o.config.HelperNode.IP, clusterRef.Name),
				TFTPServerIP:  o.config.HelperNode.IP,
				APIURL:        fmt.Sprintf("https://api.%s.%s:6443", clusterRef.Name, clusterConfig.Network.BaseDomain),
				IngressURL:    fmt.Sprintf("https://console-openshift-console.apps.%s.%s", clusterRef.Name, clusterConfig.Network.BaseDomain),
				ConsoleURL:    fmt.Sprintf("https://console-openshift-console.apps.%s.%s", clusterRef.Name, clusterConfig.Network.BaseDomain),
				IgnitionURLs:  make(map[string]string),
			},
			Timestamp: time.Now().Format(time.RFC3339),
			Status:    "in_progress",
		},
	}

	// Acquire State Lock
	if err := o.LockState(ctx); err != nil {
		return fmt.Errorf("cannot start deployment: %w", err)
	}
	defer o.UnlockState(ctx)

	// Execute deployment phases
	phases := clusterConfig.Deployment.Phases
	if len(phases) == 0 {
		phases = []string{
			"validate",
			"setup_helper_services",
			"setup_http",
			"download_images",
			"create_lpars",
			"setup_dns",
			"setup_dhcp",
			"setup_pxe",
			"setup_haproxy",
			"generate_ignition",
			"power_on",
			"wait_bootstrap",
			"wait_installation",
		}
	}

	for _, phase := range phases {
		fmt.Printf("\n--- Phase: %s ---\n", phase)
		ctx.State.CurrentPhase = phase
		o.saveState(ctx) // Save state immediately when phase starts

		// Start phase tracking
		phaseExec := o.startPhase(ctx, phase)

		if err := o.executePhase(ctx, phase); err != nil {
			// Mark phase as failed
			o.endPhase(ctx, phaseExec, "failed", err.Error())

			ctx.State.Status = "failed"
			ctx.State.Error = err.Error()
			o.saveState(ctx)

			if clusterConfig.Deployment.CleanupOnFailure {
				fmt.Printf("\nDeployment failed. Cleaning up...\n")
				// Use the new Deleter
				deleter := NewClusterDeleter(o.hmcClient, o.sshClient, o, o.verbose)
				deleter.CleanupCluster(ctx)
			}

			return fmt.Errorf("phase '%s' failed: %w", phase, err)
		}

		// Mark phase as completed
		o.endPhase(ctx, phaseExec, "completed", "")

		ctx.State.CompletedPhases = append(ctx.State.CompletedPhases, phase)
		o.saveState(ctx)

		fmt.Printf("✓ Phase '%s' completed\n", phase)
	}

	ctx.State.Status = "completed"
	ctx.State.CurrentPhase = "completed"
	o.saveState(ctx)

	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("CLUSTER DEPLOYMENT COMPLETED: %s\n", clusterRef.Name)
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")

	return nil
}

// ResumeCluster resumes deployment of a cluster from the last failed phase
func (o *Orchestrator) ResumeCluster(clusterRef ClusterRef) error {
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("RESUMING CLUSTER DEPLOYMENT: %s (%s)\n", clusterRef.Name, clusterRef.Type)
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")

	// Load cluster-specific configuration
	clusterConfig, err := clusterRef.GetClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Determine state file path
	stateFile := GetStateFilePath(clusterRef.Name)

	// Load existing state
	fmt.Printf("Loading deployment state from: %s\n", stateFile)
	state, err := o.LoadState(stateFile)
	if err != nil {
		return fmt.Errorf("failed to load deployment state: %w (hint: state file may not exist, use normal deploy instead)", err)
	}

	// Validate state
	if state.Status == "completed" {
		fmt.Printf("✓ Cluster '%s' deployment already completed\n", clusterRef.Name)
		return nil
	}

	// Display current state
	fmt.Printf("\nCurrent deployment state:\n")
	fmt.Printf("  Status: %s\n", state.Status)
	fmt.Printf("  Last phase: %s\n", state.CurrentPhase)
	fmt.Printf("  Completed phases: %v\n", state.CompletedPhases)
	if state.Error != "" {
		fmt.Printf("  Last error: %s\n", state.Error)
	}
	fmt.Println()

	// Create cluster context with loaded state
	ctx := &ClusterContext{
		Name:          clusterRef.Name,
		Type:          clusterRef.Type,
		OCPVersion:    clusterRef.OCPVersion,
		VIP:           clusterRef.VIP,
		ClusterConfig: clusterConfig,
		HelperNode:    o.config.HelperNode,
		HMC:           o.config.HMC,
		Verbose:       o.verbose,
		State:         state,
	}

	// Acquire State Lock
	if err := o.LockState(ctx); err != nil {
		return fmt.Errorf("cannot resume deployment: %w", err)
	}
	defer o.UnlockState(ctx)

	// Determine which phases to execute
	allPhases := clusterConfig.Deployment.Phases
	if len(allPhases) == 0 {
		allPhases = []string{
			"validate",
			"setup_helper_services",
			"create_lpars",
			"setup_dnsmasq",
			"setup_http",
			"setup_haproxy",
			"download_images",
			"generate_ignition",
			"power_on",
			"wait_bootstrap",
			"wait_installation",
		}
	}

	// Find remaining phases (skip completed ones)
	var remainingPhases []string
	completedMap := make(map[string]bool)
	for _, phase := range state.CompletedPhases {
		completedMap[phase] = true
	}

	for _, phase := range allPhases {
		if !completedMap[phase] {
			remainingPhases = append(remainingPhases, phase)
		}
	}

	if len(remainingPhases) == 0 {
		fmt.Println("✓ All phases already completed")
		ctx.State.Status = "completed"
		ctx.State.CurrentPhase = "completed"
		o.saveState(ctx)
		return nil
	}

	// Always populate defaults when resuming (even if validate phase was completed)
	// This ensures fields like Hostname are populated when resuming
	if clusterConfig.IsSNO() && clusterConfig.SNONode != nil {
		if clusterConfig.SNONode.Hostname == "" {
			clusterConfig.SNONode.Hostname = clusterRef.Name
		}
		if clusterConfig.SNONode.Name == "" {
			clusterConfig.SNONode.Name = clusterConfig.SNONode.Hostname + "-master"
		}
	}

	fmt.Printf("Resuming from phase: %s\n", remainingPhases[0])
	fmt.Printf("Remaining phases: %v\n\n", remainingPhases)

	// Execute remaining phases
	for _, phase := range remainingPhases {
		fmt.Printf("\n--- Phase: %s ---\n", phase)
		ctx.State.CurrentPhase = phase
		o.saveState(ctx) // Save state immediately when phase starts

		if err := o.executePhase(ctx, phase); err != nil {
			ctx.State.Status = "failed"
			ctx.State.Error = err.Error()
			o.saveState(ctx)

			if clusterConfig.Deployment.CleanupOnFailure {
				fmt.Printf("\nDeployment failed. Cleaning up...\n")
				// Use the new Deleter
				deleter := NewClusterDeleter(o.hmcClient, o.sshClient, o, o.verbose)
				deleter.CleanupCluster(ctx)
			}

			return fmt.Errorf("phase '%s' failed: %w", phase, err)
		}

		ctx.State.CompletedPhases = append(ctx.State.CompletedPhases, phase)
		o.saveState(ctx)

		fmt.Printf("✓ Phase '%s' completed\n", phase)
	}

	ctx.State.Status = "completed"
	ctx.State.CurrentPhase = "completed"
	ctx.State.Error = ""
	o.saveState(ctx)

	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("CLUSTER DEPLOYMENT RESUMED AND COMPLETED: %s\n", clusterRef.Name)
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")

	return nil
}

// executePhase executes a single deployment phase
// ============================================================================
// STATE MANAGEMENT
// ============================================================================

// startPhase begins tracking a deployment phase
func (o *Orchestrator) startPhase(ctx *ClusterContext, phaseName string) *PhaseExecution {
	phaseExec := &PhaseExecution{
		PhaseName:  phaseName,
		Status:     "running",
		StartTime:  time.Now().Format(time.RFC3339),
		RetryCount: 0,
		Artifacts:  []string{},
	}

	// Add to phase history
	ctx.State.PhaseHistory = append(ctx.State.PhaseHistory, *phaseExec)

	return phaseExec
}

// endPhase completes tracking of a deployment phase
func (o *Orchestrator) endPhase(ctx *ClusterContext, phaseExec *PhaseExecution, status string, errorMsg string) {
	endTime := time.Now()
	phaseExec.EndTime = endTime.Format(time.RFC3339)
	phaseExec.Status = status
	phaseExec.Error = errorMsg

	// Calculate duration
	if startTime, err := time.Parse(time.RFC3339, phaseExec.StartTime); err == nil {
		phaseExec.DurationSec = endTime.Sub(startTime).Seconds()
	}

	// Update the phase in history (find and replace)
	for i := range ctx.State.PhaseHistory {
		if ctx.State.PhaseHistory[i].PhaseName == phaseExec.PhaseName &&
			ctx.State.PhaseHistory[i].StartTime == phaseExec.StartTime {
			ctx.State.PhaseHistory[i] = *phaseExec
			break
		}
	}

	if o.verbose {
		fmt.Printf("  Phase '%s' %s (duration: %.2fs)\n", phaseExec.PhaseName, status, phaseExec.DurationSec)
	}
}

// saveState saves the deployment state to a file ATOMICALLY with backups
func (o *Orchestrator) saveState(ctx *ClusterContext) error {
	if !ctx.ClusterConfig.Advanced.SaveStateOnEachPhase {
		return nil
	}

	stateFile := GetStateFilePath(ctx.Name)

	ctx.State.Timestamp = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(ctx.State, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// 1. Create a backup of the existing state file before we overwrite it
	if _, err := os.Stat(stateFile); err == nil {
		backupFile := stateFile + ".backup"
		existingData, readErr := os.ReadFile(stateFile)
		if readErr == nil {
			// Ignore errors on backup creation, it shouldn't block the main save
			_ = os.WriteFile(backupFile, existingData, 0644)
		}
	}

	// 2. Write to a temporary file first
	tmpFile := stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	// 3. Atomically rename the temp file to the actual state file
	if err := os.Rename(tmpFile, stateFile); err != nil {
		return fmt.Errorf("failed to atomically save state file: %w", err)
	}

	if o.verbose {
		fmt.Printf("State saved atomically to: %s\n", stateFile)
	}

	return nil
}

// LoadState loads deployment state from a file
func (o *Orchestrator) LoadState(stateFile string) (*DeploymentState, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state DeploymentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// GetClusterStatus returns the current status of a cluster
func (o *Orchestrator) GetClusterStatus(clusterName string) (string, error) {
	stateFile := GetStateFilePath(clusterName)

	state, err := o.LoadState(stateFile)
	if err != nil {
		return "", fmt.Errorf("failed to load state: %w", err)
	}

	status := fmt.Sprintf(`Cluster: %s
Status: %s
Current Phase: %s
Completed Phases: %v
LPARs Created: %d
Timestamp: %s
`, state.ClusterName, state.Status, state.CurrentPhase,
		state.CompletedPhases, len(state.CreatedLPARs), state.Timestamp)

	if state.Error != "" {
		status += fmt.Sprintf("Error: %s\n", state.Error)
	}

	return status, nil
}
