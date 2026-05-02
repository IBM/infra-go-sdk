package main

import (
	"context"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	svc "github.com/sudeeshjohn/svc-go-sdk"
	"golang.org/x/crypto/ssh"
)

const (
	version = "1.0.0"
)

func main() {
	// Command-line flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	validateOnly := flag.Bool("validate", false, "Only validate configuration without deploying")
	resumeFrom := flag.String("resume", "", "Resume deployment from a specific phase")
	showVersion := flag.Bool("version", false, "Show version information")
	
	flag.Parse()
	
	if *showVersion {
		fmt.Printf("OpenShift UPI Deployer for IBM Power Systems v%s\n", version)
		os.Exit(0)
	}
	
	// Load configuration
	log.Printf("📋 Loading configuration from %s...", *configFile)
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}
	log.Println("✅ Configuration loaded successfully")
	
	// Validate configuration
	log.Println("\n🔍 Validating configuration...")
	validator := NewValidator(config)
	if err := validator.Validate(); err != nil {
		log.Fatalf("❌ Configuration validation failed:\n%v", err)
	}
	log.Println("✅ Configuration validation passed")
	
	if *validateOnly {
		log.Println("\n✅ Validation complete. Exiting (--validate flag set)")
		os.Exit(0)
	}
	
	// Create orchestrator
	orchestrator := NewOrchestrator(config)
	
	// Load or create deployment state
	if *resumeFrom != "" {
		log.Printf("\n🔄 Resuming deployment from phase: %s", *resumeFrom)
		if err := orchestrator.LoadState(); err != nil {
			log.Fatalf("❌ Failed to load deployment state: %v", err)
		}
		orchestrator.state.CurrentPhase = *resumeFrom
	} else {
		log.Println("\n🚀 Starting new deployment...")
		orchestrator.InitializeState()
	}
	
	// Execute deployment
	if err := orchestrator.Deploy(); err != nil {
		log.Fatalf("❌ Deployment failed: %v", err)
	}
	
	log.Println("\n🎉 Deployment completed successfully!")
	log.Printf("📊 Deployment summary:")
	log.Printf("   - Total LPARs created: %d", len(orchestrator.state.CreatedLPARs))
	log.Printf("   - Total volumes created: %d", len(orchestrator.state.CreatedVolumes))
	log.Printf("   - Completed phases: %d", len(orchestrator.state.CompletedPhases))
	
	if config.Advanced.SaveKubeconfig {
		log.Printf("   - Kubeconfig saved to: %s", config.Advanced.KubeconfigPath)
	}
}

// =============================================================================
// ORCHESTRATOR
// =============================================================================

// Orchestrator manages the deployment workflow
type Orchestrator struct {
	config      *Config
	state       *DeploymentState
	hmcClient   *hmc.HmcRestClient
	svcClient   *svc.Client
	startTime   time.Time
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config *Config) *Orchestrator {
	return &Orchestrator{
		config: config,
		state:  &DeploymentState{
			CreatedLPARs:   make(map[string]LPARState),
			CreatedVolumes: make(map[string]VolumeState),
		},
	}
}

// InitializeState initializes a new deployment state
func (o *Orchestrator) InitializeState() {
	o.state = &DeploymentState{
		CurrentPhase:    "",
		CompletedPhases: []string{},
		CreatedLPARs:    make(map[string]LPARState),
		CreatedVolumes:  make(map[string]VolumeState),
		Timestamp:       time.Now().Format(time.RFC3339),
		Status:          "in_progress",
	}
	o.startTime = time.Now()
}

// LoadState loads deployment state from file
func (o *Orchestrator) LoadState() error {
	data, err := os.ReadFile(o.config.Advanced.StateFile)
	if err != nil {
		return fmt.Errorf("failed to read state file: %v", err)
	}
	
	if err := json.Unmarshal(data, &o.state); err != nil {
		return fmt.Errorf("failed to parse state file: %v", err)
	}
	
	log.Printf("📂 Loaded deployment state:")
	log.Printf("   - Current phase: %s", o.state.CurrentPhase)
	log.Printf("   - Completed phases: %d", len(o.state.CompletedPhases))
	log.Printf("   - Created LPARs: %d", len(o.state.CreatedLPARs))
	
	return nil
}

// SaveState saves deployment state to file
func (o *Orchestrator) SaveState() error {
	o.state.Timestamp = time.Now().Format(time.RFC3339)
	
	data, err := json.MarshalIndent(o.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}
	
	if err := os.WriteFile(o.config.Advanced.StateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}
	
	return nil
}

// Deploy executes the deployment workflow
func (o *Orchestrator) Deploy() error {
	log.Println("\n" + strings.Repeat("=", 80))
	log.Println("🚀 OPENSHIFT UPI DEPLOYMENT ON IBM POWER SYSTEMS")
	log.Println(strings.Repeat("=", 80))
	
	// Connect to HMC
	if err := o.connectHMC(); err != nil {
		return fmt.Errorf("failed to connect to HMC: %v", err)
	}
	defer o.hmcClient.Logoff(context.Background())
	
	// Connect to storage backend if needed
	if o.config.Storage.Type == "svc" {
		if err := o.connectSVC(); err != nil {
			return fmt.Errorf("failed to connect to SVC: %v", err)
		}
	}
	
	// Execute each phase
	for _, phase := range o.config.Deployment.Phases {
		// Skip if already completed
		if o.isPhaseCompleted(phase) {
			log.Printf("⏭️  Skipping completed phase: %s", phase)
			continue
		}
		
		// Execute phase
		if err := o.executePhase(phase); err != nil {
			o.state.Status = "failed"
			o.state.Error = err.Error()
			o.SaveState()
			return fmt.Errorf("phase '%s' failed: %v", phase, err)
		}
		
		// Mark phase as completed
		o.state.CompletedPhases = append(o.state.CompletedPhases, phase)
		o.state.CurrentPhase = phase
		
		// Save state after each phase if configured
		if o.config.Advanced.SaveStateOnEachPhase {
			if err := o.SaveState(); err != nil {
				log.Printf("⚠️  Warning: failed to save state: %v", err)
			}
		}
	}
	
	// Mark deployment as completed
	o.state.Status = "completed"
	o.state.CurrentPhase = "completed"
	o.SaveState()
	
	return nil
}

// connectHMC establishes connection to HMC
func (o *Orchestrator) connectHMC() error {
	log.Printf("\n🔌 Connecting to HMC at %s...", o.config.HMC.IP)
	
	o.hmcClient = hmc.NewHmcRestClient(o.config.HMC.IP)
	if err := o.hmcClient.Login(o.config.HMC.Username, o.config.HMC.Password, o.config.Deployment.Verbose); err != nil {
		return err
	}
	
	log.Println("✅ Connected to HMC successfully")
	return nil
}

// connectSVC establishes connection to SVC
func (o *Orchestrator) connectSVC() error {
	log.Printf("\n🔌 Connecting to SVC at %s...", o.config.Storage.SVC.IP)
	
	o.svcClient = svc.NewClient(
		o.config.Storage.SVC.IP,
		o.config.Storage.SVC.Username,
		o.config.Storage.SVC.Password,
	).WithTLSInsecure()
	
	// Test connection
	if _, err := o.svcClient.Lssystem(); err != nil {
		return fmt.Errorf("failed to connect to SVC: %v", err)
	}
	
	log.Println("✅ Connected to SVC successfully")
	return nil
}

// executePhase executes a specific deployment phase
func (o *Orchestrator) executePhase(phase string) error {
	log.Printf("\n" + strings.Repeat("-", 80))
	log.Printf("📍 Phase: %s", phase)
	log.Printf(strings.Repeat("-", 80))
	
	startTime := time.Now()
	
	var err error
	switch phase {
	case "validate_config":
		err = o.phaseValidateConfig()
	case "check_resources":
		err = o.phaseCheckResources()
	case "create_lpars":
		err = o.phaseCreateLPARs()
	case "setup_helper_node":
		err = o.phaseSetupHelperNode()
	case "run_playbook":
		err = o.phaseRunPlaybook()
	default:
		err = fmt.Errorf("unknown phase: %s", phase)
	}
	
	duration := time.Since(startTime)
	
	if err != nil {
		log.Printf("❌ Phase failed after %v: %v", duration, err)
		return err
	}
	
	log.Printf("✅ Phase completed successfully in %v", duration)
	return nil
}

// isPhaseCompleted checks if a phase has already been completed
func (o *Orchestrator) isPhaseCompleted(phase string) bool {
	for _, completed := range o.state.CompletedPhases {
		if completed == phase {
			return true
		}
	}
	return false
}

// =============================================================================
// PHASE IMPLEMENTATIONS (Stubs - to be implemented)
// =============================================================================

func (o *Orchestrator) phaseValidateConfig() error {
	log.Println("🔍 Validating configuration...")
	validator := NewValidator(o.config)
	return validator.Validate()
}

func (o *Orchestrator) phaseCheckResources() error {
	log.Println("📊 Checking available resources on Power systems...")
	
	// Get all nodes that need to be created
	nodes := o.config.GetAllNodes()
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes configured for deployment")
	}
	
	// Group nodes by system to check resources per system
	nodesBySystem := o.config.GetNodesBySystem()
	
	// Check each system
	for systemName, systemNodes := range nodesBySystem {
		log.Printf("  Checking system: %s", systemName)
		
		// Get system details from HMC
		_, systemDetails, err := o.hmcClient.GetManagedSystemByName(context.Background(), systemName, o.config.Deployment.Verbose)
		if err != nil {
			return fmt.Errorf("failed to get system details for '%s': %v", systemName, err)
		}
		
		if systemDetails == nil {
			return fmt.Errorf("system '%s' not found on HMC", systemName)
		}
		
		// Calculate total resources needed for this system
		var totalMemoryMB int
		var totalProcessorUnits float64
		
		for _, node := range systemNodes {
			totalMemoryMB += node.LPAR.Memory.DesiredMB
			totalProcessorUnits += node.LPAR.Processor.Units
		}
		
		// Get available resources from system
		availableMemoryMB := systemDetails.MemoryConfig.CurrentAvailableSystemMemory
		availableProcessorUnits := systemDetails.ProcessorConfig.CurrentAvailableSystemProcessorUnits
		
		log.Printf("    Memory Required: %d MB, Available: %.0f MB", totalMemoryMB, availableMemoryMB)
		log.Printf("    Processor Units Required: %.2f, Available: %.2f", totalProcessorUnits, availableProcessorUnits)
		
		// Check if sufficient memory is available
		if float64(totalMemoryMB) > availableMemoryMB {
			return fmt.Errorf("insufficient memory on system '%s': need %d MB, only %.0f MB available",
				systemName, totalMemoryMB, availableMemoryMB)
		}
		
		// Check if sufficient processor units are available
		if totalProcessorUnits > availableProcessorUnits {
			return fmt.Errorf("insufficient processor units on system '%s': need %.2f units, only %.2f available",
				systemName, totalProcessorUnits, availableProcessorUnits)
		}
		
		// Check LPAR limit
		neededLPARs := len(systemNodes)
		maxLPARs := int(systemDetails.MaximumPartitions)
		
		if neededLPARs > 0 {
			log.Printf("    LPAR Slots: %d needed, max allowed: %d", neededLPARs, maxLPARs)
			
			// Note: We can't check current LPAR count without querying all partitions
			// This is a basic check against the maximum allowed
			if neededLPARs > maxLPARs {
				return fmt.Errorf("too many LPARs requested for system '%s': need %d, max allowed: %d",
					systemName, neededLPARs, maxLPARs)
			}
		}
		
		log.Printf("  ✅ System '%s' has sufficient resources", systemName)
	}
	
	log.Println("✅ All systems have sufficient resources for deployment")
	return nil
}

func (o *Orchestrator) phaseCreateLPARs() error {
	log.Println("🏗️  Creating LPARs with storage and network...")
	
	verbose := o.config.Deployment.Verbose
	
	// Get all nodes that need LPARs (SNO or HA mode)
	nodes := o.config.GetAllNodes()
	
	for _, node := range nodes {
		log.Printf("\n📦 Processing node: %s (%s)", node.Name, node.Role)
		
		// Get system details
		system, err := o.config.GetPowerSystem(node.SystemName)
		if err != nil {
			return fmt.Errorf("failed to get system %s: %v", node.SystemName, err)
		}
		
		// Resolve system UUID
		sysUUID, err := o.resolveSystemUUID(node.SystemName)
		if err != nil {
			return fmt.Errorf("failed to resolve system UUID: %v", err)
		}
		
		// Check if LPAR already exists
		if err := o.ensureLparDoesNotExist(sysUUID, node.Name); err != nil {
			return err
		}
		
		// Create LPAR
		log.Printf("[%s] Creating LPAR...", node.Name)
		lparDetails, err := o.createLPAR(sysUUID, node)
		if err != nil {
			return fmt.Errorf("failed to create LPAR %s: %v", node.Name, err)
		}
		lparUUID := lparDetails.MetadataID
		log.Printf("[%s] ✅ LPAR Created! UUID: %s", node.Name, lparUUID)
		
		// Resolve Virtual Switch UUID
		log.Printf("[%s] Resolving Virtual Switch '%s'...", node.Name, system.VswitchName)
		vswitchUUID, err := o.resolveVirtualSwitch(sysUUID, system.VswitchName)
		if err != nil {
			return fmt.Errorf("failed to resolve virtual switch: %v", err)
		}
		log.Printf("[%s] ✅ Virtual Switch Resolved: %s", node.Name, vswitchUUID)
		
		// Attach Network Adapter
		log.Printf("[%s] Attaching VLAN %d to LPAR...", node.Name, system.VlanID)
		adapter, err := o.hmcClient.CreateClientNetworkAdapter(context.Background(), sysUUID, lparUUID, vswitchUUID, system.VlanID, verbose)
		if err != nil {
			return fmt.Errorf("failed to add network adapter: %v", err)
		}
		macAddress := hmc.FormatMACAddress(adapter.MACAddress)
		log.Printf("[%s] ✅ Network Adapter Attached", node.Name)
		log.Printf("[%s]    MAC Address: %s", node.Name, macAddress)
		log.Printf("[%s]    Virtual Slot: %s", node.Name, adapter.VirtualSlotNumber)
		
		// Provision Storage
		log.Printf("[%s] Provisioning storage volumes...", node.Name)
		volumes, err := o.provisionStorage(sysUUID, lparUUID, node)
		if err != nil {
			return fmt.Errorf("failed to provision storage: %v", err)
		}
		log.Printf("[%s] ✅ Storage provisioned: %d volumes", node.Name, len(volumes))
		
		// Save LPAR configuration to profile
		profileName := lparDetails.DefaultProfileName
		log.Printf("[%s] Saving configuration to profile '%s'...", node.Name, profileName)
		if err := o.hmcClient.SaveCurrentLparConfig(context.Background(), lparUUID, profileName, true, verbose); err != nil {
			return fmt.Errorf("failed to save LPAR configuration: %v", err)
		}
		log.Printf("[%s] ✅ Configuration saved to profile", node.Name)
		
		// Store LPAR state
		o.state.CreatedLPARs[node.Name] = LPARState{
			Name:       node.Name,
			UUID:       lparUUID,
			SystemName: node.SystemName,
			IP:         node.IP,
			MACAddress: macAddress,
			Status:     "created",
			Volumes:    volumes,
		}
		
		log.Printf("[%s] ✅ LPAR creation complete", node.Name)
	}
	
	// Save state after creating all LPARs
	if o.config.Advanced.SaveStateOnEachPhase {
		if err := o.SaveState(); err != nil {
			log.Printf("⚠️  Warning: Failed to save state: %v", err)
		}
	}
	
	log.Println("\n✅ All LPARs created successfully")
	return nil
}

func (o *Orchestrator) phaseSetupHelperNode() error {
	log.Println("🛠️  Setting up helper node...")
	
	// Create output directory for generated files
	outputDir := "./generated"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	
	// Step 1: Generate setup-bastion.sh script
	log.Println("[Helper] Generating setup-bastion.sh script...")
	setupScript := o.generateBastionSetupScript()
	setupScriptPath := filepath.Join(outputDir, "setup-bastion.sh")
	if err := os.WriteFile(setupScriptPath, []byte(setupScript), 0755); err != nil {
		return fmt.Errorf("failed to write setup-bastion.sh: %v", err)
	}
	log.Printf("[Helper] ✅ Generated: %s", setupScriptPath)
	
	// Step 2: Generate ansible-vars.yaml with MAC addresses from created LPARs
	log.Println("[Helper] Generating ansible-vars.yaml with LPAR MAC addresses...")
	ansibleVars := o.generateAnsibleVars()
	ansibleVarsPath := filepath.Join(outputDir, "ansible-vars.yaml")
	if err := os.WriteFile(ansibleVarsPath, []byte(ansibleVars), 0644); err != nil {
		return fmt.Errorf("failed to write ansible-vars.yaml: %v", err)
	}
	log.Printf("[Helper] ✅ Generated: %s", ansibleVarsPath)
	
	// Step 3: Generate Ansible inventory file
	log.Println("[Helper] Generating Ansible inventory...")
	inventory := o.generateInventory()
	inventoryPath := filepath.Join(outputDir, "inventory")
	if err := os.WriteFile(inventoryPath, []byte(inventory), 0644); err != nil {
		return fmt.Errorf("failed to write inventory: %v", err)
	}
	log.Printf("[Helper] ✅ Generated: %s", inventoryPath)
	
	// Step 4: Run setup-bastion.sh script via SSH
	helperIP := o.config.HelperNode.IP
	sshUser := o.config.HelperNode.SSHUser
	sshKeyFile := o.config.HelperNode.SSHKeyFile
	
	if sshKeyFile != "" && helperIP != "" {
		log.Printf("[Helper] Connecting to %s@%s...", sshUser, helperIP)
		if err := o.executeSetupScript(setupScriptPath, ansibleVarsPath, inventoryPath); err != nil {
			log.Printf("[Helper] ⚠️  Setup script execution failed: %v", err)
			return err
		}
		log.Println("[Helper] ✅ Helper node setup script completed")
		log.Println("[Helper] ℹ️  Installed packages and configured base system")
	} else {
		return fmt.Errorf("SSH configuration required (ssh_key_file and helper node IP)")
	}
	
	return nil
}

func (o *Orchestrator) phaseRunPlaybook() error {
	log.Println("📦 Running Ansible playbook...")
	
	helperIP := o.config.HelperNode.IP
	sshUser := o.config.HelperNode.SSHUser
	sshKeyFile := o.config.HelperNode.SSHKeyFile
	
	if sshKeyFile == "" || helperIP == "" {
		return fmt.Errorf("SSH configuration required (ssh_key_file and helper node IP)")
	}
	
	log.Printf("[Playbook] Connecting to %s@%s...", sshUser, helperIP)
	if err := o.executeAnsiblePlaybook(); err != nil {
		log.Printf("[Playbook] ⚠️  Ansible playbook execution failed: %v", err)
		return err
	}
	
	log.Println("[Playbook] ✅ Ansible playbook completed successfully")
	log.Println("[Playbook] ℹ️  Configured services:")
	log.Println("[Playbook]    - DHCP/DNS/TFTP services (dnsmasq)")
	log.Println("[Playbook]    - HTTP server (httpd)")
	log.Println("[Playbook]    - PXE boot configuration")
	log.Println("[Playbook]    - Downloaded RHCOS images")
	log.Println("[Playbook]    - Netbooted SNO master LPAR")
	log.Println("[Playbook]    - All services started and enabled")
	
	return nil
}


// =============================================================================
// HELPER FUNCTIONS FOR LPAR CREATION
// =============================================================================

// resolveSystemUUID resolves the UUID of a managed system by name
func (o *Orchestrator) resolveSystemUUID(systemName string) (string, error) {
	verbose := o.config.Deployment.Verbose
	systems, err := o.hmcClient.GetManagedSystemQuickAll(context.Background(), verbose)
	if err != nil {
		return "", fmt.Errorf("failed to get managed systems: %v", err)
	}
	
	for _, system := range systems {
		if strings.EqualFold(system.SystemName, systemName) {
			if verbose {
				log.Printf("[HMC] Resolved Managed System UUID: %s", system.UUID)
			}
			return system.UUID, nil
		}
	}
	
	return "", fmt.Errorf("managed system '%s' not found", systemName)
}

// ensureLparDoesNotExist checks if an LPAR with the given name already exists
func (o *Orchestrator) ensureLparDoesNotExist(systemUUID, lparName string) error {
	verbose := o.config.Deployment.Verbose
	if verbose {
		log.Printf("[HMC] Verifying LPAR name '%s' is unique...", lparName)
	}
	
	_, existingUUID, err := o.hmcClient.GetLogicalPartitionByName(context.Background(), systemUUID, lparName, false)
	if err == nil && existingUUID != "" {
		return fmt.Errorf("LPAR with name '%s' already exists (UUID: %s)", lparName, existingUUID)
	}
	
	return nil
}

// createLPAR creates a new LPAR with the specified configuration
func (o *Orchestrator) createLPAR(systemUUID string, node NodeInfo) (*hmc.LogicalPartitionDetailed, error) {
	verbose := o.config.Deployment.Verbose
	
	// Determine sharing mode based on processor type
	// "shared" processor type typically uses "uncapped" sharing mode
	// "dedicated" processor type uses "keep idle procs" or similar
	sharingMode := "uncapped"
	if node.LPAR.Processor.Type == "dedicated" {
		sharingMode = "keep idle procs"
	}
	
	req := hmc.CreateLparRequest{
		Name:             node.Name,
		OsType:           node.LPAR.OSType,
		MinMem:           node.LPAR.Memory.MinMB,
		DesiredMem:       node.LPAR.Memory.DesiredMB,
		MaxMem:           node.LPAR.Memory.MaxMB,
		MinProcUnits:     node.LPAR.Processor.MinUnits,
		DesiredProcUnits: node.LPAR.Processor.Units,
		MaxProcUnits:     node.LPAR.Processor.MaxUnits,
		MinVcpus:         node.LPAR.Processor.MinProcs,
		DesiredVcpus:     node.LPAR.Processor.VirtualProcs,
		MaxVcpus:         node.LPAR.Processor.MaxProcs,
		SharingMode:      sharingMode,
	}
	
	lparDetails, err := o.hmcClient.CreateLogicalPartition(systemUUID, req, verbose)
	if err != nil {
		return nil, fmt.Errorf("LPAR creation failed: %v", err)
	}
	
	return lparDetails, nil
}

// resolveVirtualSwitch resolves the UUID of a virtual switch by name
func (o *Orchestrator) resolveVirtualSwitch(systemUUID, vswitchName string) (string, error) {
	verbose := o.config.Deployment.Verbose
	switches, err := o.hmcClient.GetVirtualSwitchQuickAll(context.Background(), systemUUID, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve Virtual Switches: %v", err)
	}
	
	for _, s := range switches {
		if strings.EqualFold(s.SwitchName, vswitchName) {
			return s.UUID, nil
		}
	}
	
	return "", fmt.Errorf("virtual Switch '%s' not found", vswitchName)
}

// provisionStorage provisions all storage volumes for a node
func (o *Orchestrator) provisionStorage(systemUUID, lparUUID string, node NodeInfo) ([]string, error) {
	verbose := o.config.Deployment.Verbose
	volumes := []string{}
	
	// Get VIOS configuration from storage backend
	if o.config.Storage.Type != "vios" {
		return nil, fmt.Errorf("only VIOS storage backend is currently supported")
	}
	
	// Find VIOS configuration for this system
	var viosConfig *VIOSConfig
	for i := range o.config.Storage.VIOS {
		if strings.EqualFold(o.config.Storage.VIOS[i].SystemName, node.SystemName) {
			viosConfig = &o.config.Storage.VIOS[i]
			break
		}
	}
	
	if viosConfig == nil {
		return nil, fmt.Errorf("no VIOS configuration found for system %s", node.SystemName)
	}
	
	log.Printf("[%s] Using VIOS: %s, Volume Group: %s", node.Name, viosConfig.VIOSName, viosConfig.VolumeGroup)
	
	// Get storage requirements from node configuration
	// For OpenShift, we only need a boot disk - etcd and container storage
	// will be managed by OpenShift on the boot disk
	bootDiskGB := node.LPAR.Storage.BootDiskGB
	
	// Create volume name with timestamp for uniqueness
	// AIX/VIOS has 15-char limit for LV names, so use compact format
	timestamp := time.Now().Unix() % 10000
	cleanName := strings.ReplaceAll(node.Name, " ", "")
	cleanName = strings.ReplaceAll(cleanName, "-", "")
	if len(cleanName) > 6 {
		cleanName = cleanName[:6]
	}
	
	// Format: <name>_b<timestamp> (max 15 chars)
	// Example: snoM_b1234 (10 chars)
	diskName := fmt.Sprintf("%s_b%d", cleanName, timestamp)
	
	log.Printf("[%s] Creating boot disk (%d GB)...", node.Name, bootDiskGB)
	
	viosUUID, viosName, err := o.provisionVirtualDisk(
		node.SystemName,
		systemUUID,
		diskName,
		viosConfig.VIOSName,
		viosConfig.VolumeGroup,
		bootDiskGB*1024, // Convert GB to MB
	)
	if err != nil {
		return nil, fmt.Errorf("failed to provision boot disk: %v", err)
	}
	
	log.Printf("[%s] ✅ Boot disk created on VIOS '%s'", node.Name, viosName)
	
	// Map disk to LPAR
	log.Printf("[%s] Mapping boot disk to LPAR...", node.Name)
	mappingUUID, err := o.hmcClient.CreateVirtualDiskMaps(systemUUID, viosUUID, lparUUID, []string{diskName}, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to map boot disk: %v", err)
	}
	
	if mappingUUID == "SUCCESS_WITH_RMC_WARNING" {
		log.Printf("[%s] ✅ Boot disk mapped (ignored RMC warning for offline LPAR)", node.Name)
	} else {
		log.Printf("[%s] ✅ Boot disk mapped successfully", node.Name)
	}
	
	volumes = append(volumes, diskName)
	
	// Store volume state
	o.state.CreatedVolumes[diskName] = VolumeState{
		Name:       diskName,
		VolumeID:   diskName,
		SizeGB:     bootDiskGB,
		AttachedTo: node.Name,
		Status:     "mapped",
	}
	
	return volumes, nil
}

// provisionVirtualDisk creates a virtual disk on VIOS
func (o *Orchestrator) provisionVirtualDisk(sysName, systemUUID, diskName, targetVios, targetVg string, diskSizeMB int) (string, string, error) {
	verbose := o.config.Deployment.Verbose
	requiredGB := float64(diskSizeMB) / 1024.0
	
	// Get VIOS list
	viosList, err := o.hmcClient.GetVirtualIOServersQuick(context.Background(), systemUUID, verbose)
	if err != nil || len(viosList) == 0 {
		return "", "", fmt.Errorf("failed to fetch VIOS instances for system")
	}
	
	var finalViosUUID, finalViosName, finalVgName string
	
	// Find matching VIOS and VG
	for _, vios := range viosList {
		// Filter by VIOS if provided
		if targetVios != "" && !strings.EqualFold(vios.PartitionName, targetVios) {
			continue
		}
		
		vgList, err := o.hmcClient.GetVolumeGroups(context.Background(), vios.UUID, verbose)
		if err != nil {
			continue
		}
		
		for _, vg := range vgList {
			// Filter by VG if provided
			if targetVg != "" && !strings.EqualFold(vg.GroupName, targetVg) {
				continue
			}
			
			// Check for naming collision
			collision := false
			for _, vd := range vg.VirtualDisks {
				if strings.EqualFold(vd.DiskName, diskName) {
					collision = true
					break
				}
			}
			if collision {
				continue
			}
			
			// Parse free space
			var freeSpaceGB float64
			if _, err := fmt.Sscanf(vg.FreeSpace, "%f", &freeSpaceGB); err != nil {
				continue
			}
			
			// Check capacity
			if freeSpaceGB >= requiredGB {
				finalViosUUID = vios.UUID
				finalViosName = vios.PartitionName
				finalVgName = vg.GroupName
				break
			}
		}
		
		if finalVgName != "" {
			break
		}
	}
	
	if finalVgName == "" {
		return "", "", fmt.Errorf("no suitable Volume Group found with %.2f GB free space", requiredGB)
	}
	
	if verbose {
		log.Printf("[Storage] Selected VIOS: %s, VG: %s", finalViosName, finalVgName)
	}
	
	// Create virtual disk
	if err := o.hmcClient.CreateVirtualDisk(context.Background(), sysName, finalViosUUID, finalViosName, finalVgName, diskName, diskSizeMB, verbose); err != nil {
		return "", "", fmt.Errorf("failed to create virtual disk: %v", err)
	}
	
	return finalViosUUID, finalViosName, nil
}

// =============================================================================
// HELPER FUNCTIONS FOR HELPER NODE SETUP
// =============================================================================

// generateBastionSetupScript generates the setup-bastion.sh script
func (o *Orchestrator) generateBastionSetupScript() string {
	scriptTemplate := `#!/bin/bash
# Setup script for OpenShift UPI Helper/Bastion Node
# Auto-generated by ocp-upi-deployer

set -e

echo "========================================================================="
echo " Setting up OpenShift UPI Helper Node"
echo "========================================================================="

# Detect distribution and version
DISTRO=$(lsb_release -ds 2>/dev/null || cat /etc/*release 2>/dev/null | head -n1 || uname -om || echo "")
OS_VERSION=$(lsb_release -rs 2>/dev/null || cat /etc/*release 2>/dev/null | grep "VERSION_ID" | awk -F "=" '{print $2}' | sed 's/"*//g' || echo "")

echo "Detected OS: $DISTRO"
echo "OS Version: $OS_VERSION"

# Install Ansible based on distribution
if [[ "$DISTRO" != *CentOS* ]]; then
    echo "Installing Ansible for RHEL..."
    if [[ $(cat /etc/redhat-release | sed 's/[^0-9.]*//g') > 8.5 ]]; then
        sudo subscription-manager repos --enable codeready-builder-for-rhel-9-ppc64le-rpms
        sudo yum install -y ansible-core
    else
        sudo subscription-manager repos --enable ansible-2.9-for-rhel-8-ppc64le-rpms
        sudo yum install -y ansible
    fi
else
    echo "Installing Ansible for CentOS..."
    if [[ $OS_VERSION != "8"* ]]; then
        sudo yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
        sudo yum install -y ansible-core
    else
        sudo yum install -y epel-release epel-next-release
        sudo yum config-manager --set-enabled powertools
        sudo yum install -y ansible
    fi
fi

# Install Ansible collections
echo "Installing Ansible collections..."
sudo ansible-galaxy collection install community.crypto --upgrade
sudo ansible-galaxy collection install community.general --upgrade
sudo ansible-galaxy collection install ansible.posix --upgrade
sudo ansible-galaxy collection install kubernetes.core --upgrade

# Install required packages
echo "Installing required packages..."
sudo yum install -y wget jq git net-tools vim tar unzip \
    python3 python3-pip python3-jmespath \
    coreos-installer grub2-tools-extra bind-utils \
    dnsmasq httpd haproxy sshpass

# Setup PXE boot files
echo "Setting up PXE boot files..."
sudo grub2-mknetdir --net-directory=/var/lib/tftpboot

# Create required directories
sudo mkdir -p /var/www/html/ignition
sudo mkdir -p /var/www/html/rhcos
sudo mkdir -p /var/lib/tftpboot/pxelinux
sudo mkdir -p /etc/dnsmasq.d
sudo mkdir -p /root/.openshift

# Setup passwordless SSH to HMC
echo "========================================================================="
echo " Setting up passwordless SSH to HMC..."
echo "========================================================================="

# Generate SSH key if it doesn't exist
if [ ! -f ~/.ssh/id_ed25519 ]; then
    echo "Generating ED25519 SSH key..."
    ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N "" -C "helper-node-to-hmc"
fi

# Also generate RSA key for compatibility
if [ ! -f ~/.ssh/id_rsa ]; then
    echo "Generating RSA SSH key for compatibility..."
    ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N "" -C "helper-node-to-hmc-rsa"
fi

# Configure SSH client for HMC
cat > ~/.ssh/config << 'SSHCONFIG'
Host {{.HMCHost}}
    HostName {{.HMCHost}}
    User {{.HMCUser}}
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    IdentityFile ~/.ssh/id_ed25519
    IdentityFile ~/.ssh/id_rsa
SSHCONFIG

chmod 600 ~/.ssh/config

echo "Copying SSH public key to HMC {{.HMCHost}}..."
echo "This will require HMC password: {{.HMCPassword}}"

# Try ED25519 key first
if sshpass -p '{{.HMCPassword}}' ssh {{.HMCUser}}@{{.HMCHost}} "mkauthkeys -a \"$(cat ~/.ssh/id_ed25519.pub)\"" 2>/dev/null; then
    echo "✅ ED25519 key added to HMC successfully"
else
    echo "⚠️  ED25519 key failed, trying RSA key..."
    if sshpass -p '{{.HMCPassword}}' ssh {{.HMCUser}}@{{.HMCHost}} "mkauthkeys -a \"$(cat ~/.ssh/id_rsa.pub)\"" 2>/dev/null; then
        echo "✅ RSA key added to HMC successfully"
    else
        echo "❌ Failed to add SSH key to HMC"
        echo "You may need to manually run:"
        echo "  ssh {{.HMCUser}}@{{.HMCHost}} \"mkauthkeys -a \\\"\$(cat ~/.ssh/id_ed25519.pub)\\\"\""
    fi
fi

# Test passwordless SSH
echo "Testing passwordless SSH to HMC..."
if ssh -o BatchMode=yes -o ConnectTimeout=5 {{.HMCUser}}@{{.HMCHost}} "echo 'SSH test successful'" 2>/dev/null; then
    echo "✅ Passwordless SSH to HMC is working!"
else
    echo "⚠️  Passwordless SSH test failed - you may need to configure it manually"
fi

echo "========================================================================="
echo " ✅ Helper node setup complete!"
echo " Next steps:"
echo "   1. Copy ansible-vars.yaml to this server"
echo "   2. Run the Ansible playbook to configure services"
echo "========================================================================="
`
	
	// Parse and execute the template
	tmpl, err := template.New("setup-bastion").Parse(scriptTemplate)
	if err != nil {
		log.Printf("[Helper] ⚠️  Failed to parse setup script template: %v", err)
		return scriptTemplate // Return unprocessed template as fallback
	}
	
	// Prepare template data
	data := struct {
		HMCHost     string
		HMCUser     string
		HMCPassword string
	}{
		HMCHost:     o.config.HMC.IP,
		HMCUser:     o.config.HMC.Username,
		HMCPassword: o.config.HMC.Password,
	}
	
	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("[Helper] ⚠️  Failed to execute setup script template: %v", err)
		return scriptTemplate // Return unprocessed template as fallback
	}
	
	return buf.String()
}

// generateAnsibleVars generates ansible-vars.yaml using the template with MAC addresses from created LPARs
func (o *Orchestrator) generateAnsibleVars() string {
	// Read the template file
	tmplPath := filepath.Join("templates", "ansible-vars.yaml.tmpl")
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		log.Printf("[Helper] ⚠️  Failed to read template %s: %v", tmplPath, err)
		log.Println("[Helper] Falling back to basic generation...")
		return o.generateAnsibleVarsBasic()
	}
	
	// Create a copy of config and populate MAC addresses from created LPARs
	configWithMACs := *o.config
	
	// For SNO mode, populate the SNONode with MAC address from state
	if configWithMACs.SNONode != nil {
		if lpar, exists := o.state.CreatedLPARs[configWithMACs.SNONode.Name]; exists {
			// Create a copy of SNONode and add MAC address
			snoWithMAC := *configWithMACs.SNONode
			snoWithMAC.MACAddress = lpar.MACAddress
			configWithMACs.SNONode = &snoWithMAC
		}
	}
	
	// For HA mode, populate Masters with MAC addresses from state
	if configWithMACs.Masters != nil {
		mastersWithMACs := *configWithMACs.Masters
		for i := range mastersWithMACs.Nodes {
			if lpar, exists := o.state.CreatedLPARs[mastersWithMACs.Nodes[i].Name]; exists {
				mastersWithMACs.Nodes[i].MACAddress = lpar.MACAddress
			}
		}
		configWithMACs.Masters = &mastersWithMACs
	}
	
	// For HA mode, populate Workers with MAC addresses from state
	if configWithMACs.Workers != nil {
		workersWithMACs := *configWithMACs.Workers
		for i := range workersWithMACs.Nodes {
			if lpar, exists := o.state.CreatedLPARs[workersWithMACs.Nodes[i].Name]; exists {
				workersWithMACs.Nodes[i].MACAddress = lpar.MACAddress
			}
		}
		configWithMACs.Workers = &workersWithMACs
	}
	
	// Define custom template functions
	funcMap := template.FuncMap{
		"replace": func(s, old, new string) string {
			return strings.Replace(s, old, new, -1) // Replace all occurrences
		},
	}
	
	// Parse the template with custom functions
	tmpl, err := template.New("ansible-vars").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		log.Printf("[Helper] ⚠️  Failed to parse template: %v", err)
		log.Println("[Helper] Falling back to basic generation...")
		return o.generateAnsibleVarsBasic()
	}
	
	// Execute the template with the config (now with MAC addresses)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, &configWithMACs); err != nil {
		log.Printf("[Helper] ⚠️  Failed to execute template: %v", err)
		log.Println("[Helper] Falling back to basic generation...")
		return o.generateAnsibleVarsBasic()
	}
	
	return buf.String()
}

// generateInventory generates Ansible inventory file using the template
func (o *Orchestrator) generateInventory() string {
	// Read the template file
	tmplPath := filepath.Join("templates", "inventory.tmpl")
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		log.Printf("[Helper] ⚠️  Failed to read template %s: %v", tmplPath, err)
		// Fallback to simple inventory
		return fmt.Sprintf("[bastion]\n%s ansible_connection=local\n", o.config.HelperNode.IP)
	}
	
	// Parse and execute template
	tmpl, err := template.New("inventory").Parse(string(tmplContent))
	if err != nil {
		log.Printf("[Helper] ⚠️  Failed to parse inventory template: %v", err)
		return fmt.Sprintf("[bastion]\n%s ansible_connection=local\n", o.config.HelperNode.IP)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, o.config); err != nil {
		log.Printf("[Helper] ⚠️  Failed to execute inventory template: %v", err)
		return fmt.Sprintf("[bastion]\n%s ansible_connection=local\n", o.config.HelperNode.IP)
	}
	
	return buf.String()
}

// generateAnsibleVarsBasic generates a basic ansible-vars.yaml (fallback)
func (o *Orchestrator) generateAnsibleVarsBasic() string {
	var sb strings.Builder
	
	sb.WriteString("# Ansible variables for OpenShift UPI deployment\n")
	sb.WriteString("# Auto-generated by ocp-upi-deployer (basic mode)\n\n")
	
	// Cluster configuration
	sb.WriteString("# Cluster Configuration\n")
	sb.WriteString(fmt.Sprintf("cluster_name: %s\n", o.config.Network.ClusterName))
	sb.WriteString(fmt.Sprintf("base_domain: %s\n", o.config.Network.BaseDomain))
	sb.WriteString(fmt.Sprintf("openshift_version: \"%s\"\n\n", o.config.OpenShift.Version))
	
	// Network configuration
	sb.WriteString("# Network Configuration\n")
	sb.WriteString(fmt.Sprintf("network_cidr: %s\n", o.config.Network.NetworkCIDR))
	sb.WriteString(fmt.Sprintf("gateway: %s\n", o.config.Network.Gateway))
	sb.WriteString(fmt.Sprintf("netmask: %s\n", o.config.Network.Netmask))
	sb.WriteString(fmt.Sprintf("nameserver: %s\n\n", o.config.Network.Nameserver))
	
	// Helper node configuration
	sb.WriteString("# Helper Node Configuration\n")
	sb.WriteString(fmt.Sprintf("helper_ip: %s\n", o.config.HelperNode.IP))
	sb.WriteString(fmt.Sprintf("helper_hostname: %s\n", o.config.HelperNode.Hostname))
	sb.WriteString(fmt.Sprintf("network_interface: %s\n\n", o.config.HelperNode.NetworkInterface))
	
	// DHCP configuration
	sb.WriteString("# DHCP Configuration\n")
	sb.WriteString(fmt.Sprintf("dhcp_range_start: %s\n", o.config.HelperNode.Services.Dnsmasq.DHCPRangeStart))
	sb.WriteString(fmt.Sprintf("dhcp_range_end: %s\n", o.config.HelperNode.Services.Dnsmasq.DHCPRangeEnd))
	sb.WriteString(fmt.Sprintf("dhcp_lease_time: %s\n\n", o.config.HelperNode.Services.Dnsmasq.DHCPLeaseTime))
	
	// Nodes with MAC addresses from created LPARs
	sb.WriteString("# Cluster Nodes (with MAC addresses from created LPARs)\n")
	sb.WriteString("nodes:\n")
	
	// Add nodes from created LPARs
	for name, lpar := range o.state.CreatedLPARs {
		sb.WriteString(fmt.Sprintf("  - name: %s\n", name))
		sb.WriteString(fmt.Sprintf("    ip: %s\n", lpar.IP))
		sb.WriteString(fmt.Sprintf("    mac: %s\n", lpar.MACAddress))
		
		// Determine role from node name or config
		role := "master" // Default
		if o.config.IsSNO() {
			role = "sno"
		}
		sb.WriteString(fmt.Sprintf("    role: %s\n", role))
	}
	
	sb.WriteString("\n")
	return sb.String()
}

// executeHelperSetupViaSSH executes the setup script on helper node via SSH
func (o *Orchestrator) executeHelperSetupViaSSH(setupScriptPath, ansibleVarsPath, inventoryPath string) error {
	helperIP := o.config.HelperNode.IP
	sshUser := o.config.HelperNode.SSHUser
	sshKeyFile := o.config.HelperNode.SSHKeyFile
	
	log.Printf("[SSH] Connecting to %s@%s...", sshUser, helperIP)
	
	// Create SSH client
	client, err := o.createSSHClient(helperIP, sshUser, sshKeyFile)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()
	
	log.Println("[SSH] ✅ Connected successfully")
	
	// Step 1: Copy setup-bastion.sh to helper node
	log.Println("[SSH] Copying setup-bastion.sh to helper node...")
	if err := o.scpFile(client, setupScriptPath, "/tmp/setup-bastion.sh"); err != nil {
		return fmt.Errorf("failed to copy setup-bastion.sh: %v", err)
	}
	log.Println("[SSH] ✅ setup-bastion.sh copied")
	
	// Step 2: Copy ansible-vars.yaml to helper node
	log.Println("[SSH] Copying ansible-vars.yaml to helper node...")
	if err := o.scpFile(client, ansibleVarsPath, "/tmp/ansible-vars.yaml"); err != nil {
		return fmt.Errorf("failed to copy ansible-vars.yaml: %v", err)
	}
	log.Println("[SSH] ✅ ansible-vars.yaml copied")
	
	// Step 3: Copy inventory file to helper node
	log.Println("[SSH] Copying inventory file to helper node...")
	if err := o.scpFile(client, inventoryPath, "/tmp/inventory"); err != nil {
		return fmt.Errorf("failed to copy inventory: %v", err)
	}
	log.Println("[SSH] ✅ inventory copied")
	
	// Step 4: Execute setup-bastion.sh
	log.Println("[SSH] Executing setup-bastion.sh (this may take several minutes)...")
	if err := o.executeSSHCommand(client, "sudo bash /tmp/setup-bastion.sh", true); err != nil {
		return fmt.Errorf("failed to execute setup-bastion.sh: %v", err)
	}
	log.Println("[SSH] ✅ setup-bastion.sh completed")
	
	// Step 5: Clone Ansible playbook repository
	log.Println("[SSH] Cloning ocp4-ai-powervm repository...")
	cloneCmd := "cd /root && rm -rf ocp4-ai-powervm && git clone https://github.com/cs-zhang/ocp4-ai-powervm.git"
	if err := o.executeSSHCommand(client, cloneCmd, true); err != nil {
		return fmt.Errorf("failed to clone repository: %v", err)
	}
	log.Println("[SSH] ✅ Repository cloned")
	
	// Step 6: Copy ansible-vars.yaml and inventory to repository
	log.Println("[SSH] Copying ansible-vars.yaml and inventory to repository...")
	copyCmd := "cp /tmp/ansible-vars.yaml /root/ocp4-ai-powervm/vars.yaml && cp /tmp/inventory /root/ocp4-ai-powervm/inventory"
	if err := o.executeSSHCommand(client, copyCmd, false); err != nil {
		return fmt.Errorf("failed to copy vars.yaml and inventory: %v", err)
	}
	log.Println("[SSH] ✅ vars.yaml and inventory copied to repository")
	
	// Step 7: Run Ansible playbook with inventory file
	log.Println("[SSH] Running Ansible playbook (this will take 10-15 minutes)...")
	log.Println("[SSH] This will install and configure: DHCP, DNS, TFTP, HTTP services")
	ansibleCmd := "cd /root/ocp4-ai-powervm && ansible-playbook -i inventory -e @vars.yaml playbooks/main.yaml"
	if err := o.executeSSHCommand(client, ansibleCmd, true); err != nil {
		return fmt.Errorf("failed to run Ansible playbook: %v", err)
	}
	log.Println("[SSH] ✅ Ansible playbook completed successfully")
	
	// Step 8: Verify services are running
	log.Println("[SSH] Verifying services...")
	verifyCmd := "systemctl is-active dnsmasq httpd"
	if err := o.executeSSHCommand(client, verifyCmd, true); err != nil {
		log.Printf("[SSH] ⚠️  Warning: Service verification failed: %v", err)
		log.Println("[SSH] Services may need manual verification")
	} else {
		log.Println("[SSH] ✅ Services verified and running")
	}
	
	return nil
}

// executeSetupScript executes the setup-bastion.sh script on helper node via SSH
func (o *Orchestrator) executeSetupScript(setupScriptPath, ansibleVarsPath, inventoryPath string) error {
	helperIP := o.config.HelperNode.IP
	sshUser := o.config.HelperNode.SSHUser
	sshKeyFile := o.config.HelperNode.SSHKeyFile
	
	log.Printf("[SSH] Connecting to %s@%s...", sshUser, helperIP)
	
	// Create SSH client
	client, err := o.createSSHClient(helperIP, sshUser, sshKeyFile)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()
	
	log.Println("[SSH] ✅ Connected successfully")
	
	// Step 1: Copy setup-bastion.sh to helper node
	log.Println("[SSH] Copying setup-bastion.sh to helper node...")
	if err := o.scpFile(client, setupScriptPath, "/tmp/setup-bastion.sh"); err != nil {
		return fmt.Errorf("failed to copy setup-bastion.sh: %v", err)
	}
	log.Println("[SSH] ✅ setup-bastion.sh copied")
	
	// Step 2: Copy ansible-vars.yaml to helper node
	log.Println("[SSH] Copying ansible-vars.yaml to helper node...")
	if err := o.scpFile(client, ansibleVarsPath, "/tmp/ansible-vars.yaml"); err != nil {
		return fmt.Errorf("failed to copy ansible-vars.yaml: %v", err)
	}
	log.Println("[SSH] ✅ ansible-vars.yaml copied")
	
	// Step 3: Copy inventory file to helper node
	log.Println("[SSH] Copying inventory file to helper node...")
	if err := o.scpFile(client, inventoryPath, "/tmp/inventory"); err != nil {
		return fmt.Errorf("failed to copy inventory: %v", err)
	}
	log.Println("[SSH] ✅ inventory copied")
	
	// Step 4: Copy pull-secret.json to helper node
	pullSecretPath := o.config.OpenShift.PullSecretFile
	if pullSecretPath != "" {
		// Expand ~ to home directory
		if strings.HasPrefix(pullSecretPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				pullSecretPath = filepath.Join(homeDir, pullSecretPath[2:])
			}
		}
		log.Println("[SSH] Copying pull-secret.json to helper node...")
		if err := o.scpFile(client, pullSecretPath, "/root/.openshift/pull-secret.json"); err != nil {
			return fmt.Errorf("failed to copy pull-secret.json: %v", err)
		}
		log.Println("[SSH] ✅ pull-secret.json copied")
	}
	
	// Step 5: Execute setup-bastion.sh
	log.Println("[SSH] Executing setup-bastion.sh (this may take several minutes)...")
	if err := o.executeSSHCommand(client, "sudo bash /tmp/setup-bastion.sh", true); err != nil {
		return fmt.Errorf("failed to execute setup-bastion.sh: %v", err)
	}
	log.Println("[SSH] ✅ setup-bastion.sh completed")
	
	return nil
}

// executeAnsiblePlaybook runs the Ansible playbook on helper node via SSH
func (o *Orchestrator) executeAnsiblePlaybook() error {
	helperIP := o.config.HelperNode.IP
	sshUser := o.config.HelperNode.SSHUser
	sshKeyFile := o.config.HelperNode.SSHKeyFile
	
	log.Printf("[SSH] Connecting to %s@%s...", sshUser, helperIP)
	
	// Create SSH client
	client, err := o.createSSHClient(helperIP, sshUser, sshKeyFile)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()
	
	log.Println("[SSH] ✅ Connected successfully")
	
	// Step 1: Clone Ansible playbook repository
	playbookRepo := o.config.Ansible.PlaybookRepo
	if playbookRepo == "" {
		playbookRepo = "https://github.com/cs-zhang/ocp4-ai-powervm.git"
	}
	playbookBranch := o.config.Ansible.PlaybookBranch
	if playbookBranch == "" {
		playbookBranch = "main"
	}
	
	log.Printf("[SSH] Cloning Ansible playbook repository: %s (branch: %s)...", playbookRepo, playbookBranch)
	cloneCmd := fmt.Sprintf("cd /root && rm -rf ocp4-ai-powervm && git clone -b %s %s ocp4-ai-powervm", playbookBranch, playbookRepo)
	if err := o.executeSSHCommand(client, cloneCmd, true); err != nil {
		return fmt.Errorf("failed to clone repository: %v", err)
	}
	log.Println("[SSH] ✅ Repository cloned")
	
	// Step 2: Copy ansible-vars.yaml and inventory to repository
	log.Println("[SSH] Copying ansible-vars.yaml and inventory to repository...")
	copyCmd := "cp /tmp/ansible-vars.yaml /root/ocp4-ai-powervm/vars.yaml && cp /tmp/inventory /root/ocp4-ai-powervm/inventory"
	if err := o.executeSSHCommand(client, copyCmd, false); err != nil {
		return fmt.Errorf("failed to copy vars.yaml and inventory: %v", err)
	}
	log.Println("[SSH] ✅ vars.yaml and inventory copied to repository")
	
	// Step 3: Run Ansible playbook with inventory file
	log.Println("[SSH] Running Ansible playbook (this will take 10-15 minutes)...")
	log.Println("[SSH] This will install and configure: DHCP, DNS, TFTP, HTTP services")
	log.Println("[SSH] The playbook will also netboot the SNO master LPAR automatically")
	ansibleCmd := "cd /root/ocp4-ai-powervm && ansible-playbook -i inventory -e @vars.yaml playbooks/main.yaml"
	if err := o.executeSSHCommand(client, ansibleCmd, true); err != nil {
		return fmt.Errorf("failed to run Ansible playbook: %v", err)
	}
	log.Println("[SSH] ✅ Ansible playbook completed successfully")
	
	// Step 4: Verify services are running
	log.Println("[SSH] Verifying services...")
	verifyCmd := "systemctl is-active dnsmasq httpd"
	if err := o.executeSSHCommand(client, verifyCmd, true); err != nil {
		log.Printf("[SSH] ⚠️  Warning: Service verification failed: %v", err)
		log.Println("[SSH] Services may need manual verification")
	} else {
		log.Println("[SSH] ✅ Services verified and running")
	}
	
	return nil
}

// createSSHClient creates an SSH client connection
func (o *Orchestrator) createSSHClient(host, user, keyFile string) (*ssh.Client, error) {
	// Read private key
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key file: %v", err)
	}
	
	// Parse private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %v", err)
	}
	
	// Configure SSH client
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For simplicity; use proper host key checking in production
		Timeout:         30 * time.Second,
	}
	
	// Connect to SSH server
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %v", err)
	}
	
	return client, nil
}

// scpFile copies a local file to remote host via SCP
func (o *Orchestrator) scpFile(client *ssh.Client, localPath, remotePath string) error {
	// Read local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %v", err)
	}
	
	// Create SCP session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()
	
	// Set up stdin pipe
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}
	
	// Start SCP command
	go func() {
		defer stdin.Close()
		// SCP protocol: C0644 <size> <filename>\n<content>
		fmt.Fprintf(stdin, "C0644 %d %s\n", len(content), filepath.Base(remotePath))
		stdin.Write(content)
		fmt.Fprint(stdin, "\x00") // Null byte to signal end
	}()
	
	// Execute SCP command
	if err := session.Run(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return fmt.Errorf("failed to execute SCP: %v", err)
	}
	
	return nil
}

// executeSSHCommand executes a command on remote host via SSH
func (o *Orchestrator) executeSSHCommand(client *ssh.Client, command string, streamOutput bool) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()
	
	if streamOutput {
		// Stream output to console
		var stdoutBuf, stderrBuf bytes.Buffer
		session.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		session.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
		
		if err := session.Run(command); err != nil {
			return fmt.Errorf("command failed: %v\nStderr: %s", err, stderrBuf.String())
		}
	} else {
		// Just execute without streaming
		output, err := session.CombinedOutput(command)
		if err != nil {
			return fmt.Errorf("command failed: %v\nOutput: %s", err, string(output))
		}
	}
	
	return nil
}

// printManualInstructions prints manual setup instructions
func (o *Orchestrator) printManualInstructions(helperIP, sshUser string) {
	log.Println("")
	log.Println("========================================================================")
	log.Println(" 📋 MANUAL SETUP INSTRUCTIONS")
	log.Println("========================================================================")
	log.Println("")
	log.Println("Generated files are available in: ./generated/")
	log.Println("")
	log.Println("Step 1: Copy files to helper node")
	log.Printf("  scp ./generated/setup-bastion.sh %s@%s:/tmp/\n", sshUser, helperIP)
	log.Printf("  scp ./generated/ansible-vars.yaml %s@%s:/tmp/\n", sshUser, helperIP)
	log.Println("")
	log.Println("Step 2: SSH to helper node and run setup script")
	log.Printf("  ssh %s@%s\n", sshUser, helperIP)
	log.Println("  sudo bash /tmp/setup-bastion.sh")
	log.Println("")
	log.Println("Step 3: Run Ansible playbook (after setup completes)")
	log.Println("  # Copy your Ansible playbook to helper node")
	log.Println("  # Run: ansible-playbook -e @/tmp/ansible-vars.yaml your-playbook.yml")
	log.Println("")
	log.Println("========================================================================")
	log.Println("")
}

// Made with Bob
