package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	hmc "github.com/sudeeshjohn/powerhmc-go"
	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

// LPARProvisioner handles LPAR creation and management
type LPARProvisioner struct {
	ctx       *ClusterContext
	hmcClient *hmc.HmcRestClient
}

// NewLPARProvisioner creates a new LPAR provisioner
func NewLPARProvisioner(ctx *ClusterContext, hmcClient *hmc.HmcRestClient) *LPARProvisioner {
	return &LPARProvisioner{
		ctx:       ctx,
		hmcClient: hmcClient,
	}
}

// generateDiskName creates a unique disk name using cluster name, node name, suffix, and timestamp
// Format: <cluster>-<role>-<suffix>-<hash> (max 15 chars for VIOS logical volume limit)
// Example: snonew5-n-b-a3f9 (for sno-new-5 node boot disk with timestamp-based hash)
func (l *LPARProvisioner) generateDiskName(nodeName, suffix string) string {
	// Simplify cluster name (max 7 chars)
	cName := l.ctx.Name
	cName = strings.ReplaceAll(cName, "-", "")
	if len(cName) > 7 {
		cName = cName[:7]
	}

	// Determine role abbreviation
	role := "n" // default for SNO or unknown
	nName := nodeName
	nName = strings.TrimPrefix(nName, l.ctx.Name+"-") // Remove cluster prefix if it exists

	if nName != "" && nName != nodeName {
		// Multi-node cluster - determine role
		if strings.Contains(nName, "master") {
			role = "m"
		} else if strings.Contains(nName, "worker") {
			role = "w"
		} else if strings.Contains(nName, "bootstrap") {
			role = "b"
		}
	}

	// Abbreviate suffix
	shortSuffix := "b" // boot
	if suffix == "data" {
		shortSuffix = "d"
	}

	// Generate a short hash from cluster+node+suffix+timestamp for true uniqueness
	// Include nanosecond timestamp to ensure different hash on each invocation
	timestamp := time.Now().UnixNano()
	hashInput := fmt.Sprintf("%s-%s-%s-%d", l.ctx.Name, nodeName, suffix, timestamp)
	hash := sha256.Sum256([]byte(hashInput))
	shortHash := hex.EncodeToString(hash[:])[:4] // Use first 4 chars of hash

	// Result: snonew5-n-b-a3f9 (15 chars max)
	// Format: <cluster>-<role>-<suffix>-<hash>
	diskName := fmt.Sprintf("%s-%s-%s-%s", cName, role, shortSuffix, shortHash)

	// Ensure we don't exceed 15 chars
	if len(diskName) > 15 {
		// Truncate cluster name further if needed
		excess := len(diskName) - 15
		if len(cName) > excess {
			cName = cName[:len(cName)-excess]
			diskName = fmt.Sprintf("%s-%s-%s-%s", cName, role, shortSuffix, shortHash)
		}
	}

	return diskName
}

// ProvisionAll provisions all LPARs for the cluster sequentially
func (l *LPARProvisioner) ProvisionAll() error {
	fmt.Printf("Provisioning LPARs for cluster '%s'...\n", l.ctx.Name)

	nodes := l.ctx.ClusterConfig.GetAllNodes()
	fmt.Printf("Starting sequential provisioning of %d node(s)...\n", len(nodes))

	// Provision each node sequentially
	for _, node := range nodes {
		if err := l.provisionSingleNode(node); err != nil {
			return fmt.Errorf("failed to provision node %s: %w", node.Hostname, err)
		}
	}

	fmt.Printf("\nAll LPARs provisioned successfully for cluster '%s'\n", l.ctx.Name)
	return nil
}

// provisionSingleNode handles the end-to-end creation of a single LPAR with granular state tracking
func (l *LPARProvisioner) provisionSingleNode(node NodeInfo) error {
	fmt.Printf("\nProvisioning LPAR for node: %s (%s)\n", node.Hostname, node.Role)

	// Check if LPAR already exists in state and resume from last successful step
	existingState, exists := l.ctx.State.CreatedLPARs[node.Hostname]

	var lparUUID, systemUUID string
	var volumes []VolumeState
	var macAddress, locationCode string
	powerSystem, _ := l.ctx.ClusterConfig.GetPowerSystem(node.SystemName)

	// Handle nil powerSystem
	var ps PowerSystem
	if powerSystem != nil {
		ps = *powerSystem
	}

	// Step 1: Create LPAR (just the partition itself)
	if !exists || existingState.Status == "" {
		fmt.Printf("  Step 1: Creating LPAR...\n")
		var err error
		lparUUID, systemUUID, err = l.createLPAR(node)
		if err != nil {
			return fmt.Errorf("failed to create LPAR: %w", err)
		}
		fmt.Printf("  ✓ LPAR created: %s\n", lparUUID)

		// Save state after LPAR creation
		if err := l.saveStateAfterLPARCreation(node, lparUUID, systemUUID, ps); err != nil {
			return fmt.Errorf("failed to save state after LPAR creation: %w", err)
		}
	} else {
		fmt.Printf("  Step 1: LPAR already exists (UUID: %s), skipping creation\n", existingState.UUID)
		lparUUID = existingState.UUID
		systemUUID = existingState.SystemUUID
	}

	// Step 2: Attach storage (before network adapter)
	if !exists || existingState.Status == "lpar_created" {
		fmt.Printf("  Step 2: Attaching storage...\n")

		// Ensure LPAR is powered off before attaching storage
		if err := l.ensureLPARPoweredOff(lparUUID, node.Hostname); err != nil {
			return fmt.Errorf("failed to ensure LPAR is powered off: %w", err)
		}

		var err error
		volumes, err = l.attachStorage(node, lparUUID)

		// ALWAYS save whatever volumes were successfully created to state,
		// even if the function ultimately returned an error (like a mapping failure)
		if len(volumes) > 0 {
			_ = l.saveStateAfterStorageAttachment(node, lparUUID, systemUUID, volumes, ps)
		}

		if err != nil {
			return fmt.Errorf("failed to attach storage (partial state saved): %w", err)
		}
		fmt.Printf("  ✓ Storage attached: %d volumes\n", len(volumes))

		// Save state after storage attachment
		if err := l.saveStateAfterStorageAttachment(node, lparUUID, systemUUID, volumes, ps); err != nil {
			return fmt.Errorf("failed to save state after storage attachment: %w", err)
		}
	} else {
		fmt.Printf("  Step 2: Storage already attached, skipping\n")
		// Load volumes from existing state
		for _, volName := range existingState.Volumes {
			if vol, ok := l.ctx.State.CreatedVolumes[volName]; ok {
				volumes = append(volumes, vol)
			}
		}
	}

	// Step 3: Create network adapter (after storage)
	if !exists || existingState.Status == "storage_attached" {
		fmt.Printf("  Step 3: Attaching network adapter...\n")

		// Ensure LPAR is powered off before attaching network adapter
		if err := l.ensureLPARPoweredOff(lparUUID, node.Hostname); err != nil {
			return fmt.Errorf("failed to ensure LPAR is powered off: %w", err)
		}

		var err error
		macAddress, locationCode, err = l.attachNetworkAdapter(node, systemUUID, lparUUID)
		if err != nil {
			return fmt.Errorf("failed to attach network adapter: %w", err)
		}
		fmt.Printf("  ✓ Network adapter attached: MAC=%s, Location=%s\n", macAddress, locationCode)

		// Save state after network adapter attachment
		if err := l.saveStateAfterNetworkAttachment(node, lparUUID, systemUUID, volumes, macAddress, locationCode, ps); err != nil {
			return fmt.Errorf("failed to save state after network attachment: %w", err)
		}

		// Update node configuration with MAC address for DNSmasq
		if err := l.updateNodeMACAddress(node.Hostname, macAddress); err != nil {
			return fmt.Errorf("failed to update node MAC address: %w", err)
		}
	} else {
		fmt.Printf("  Step 3: Network adapter already attached, skipping\n")
		macAddress = existingState.MACAddress
		locationCode = existingState.LocationCode
	}

	// Step 4: Save current LPAR configuration to profile
	if !exists || existingState.Status == "network_attached" {
		fmt.Printf("  Step 4: Saving LPAR configuration to profile...\n")
		profileName := "default_profile"
		if err := l.hmcClient.SaveCurrentLparConfig(lparUUID, profileName, true, l.ctx.Verbose); err != nil {
			return fmt.Errorf("failed to save LPAR configuration: %w", err)
		}
		fmt.Printf("  ✓ Configuration saved to profile: %s\n", profileName)

		// Save final state
		if err := l.saveStateAfterProfileSave(node, lparUUID, systemUUID, volumes, macAddress, locationCode, ps); err != nil {
			return fmt.Errorf("failed to save state after profile save: %w", err)
		}
	} else {
		fmt.Printf("  Step 4: Profile already saved, skipping\n")
	}

	if l.ctx.Verbose {
		fmt.Printf("  [DEBUG] [%s] LPAR state after provisioning:\n", node.Hostname)
		fmt.Printf("    UUID: %s\n", lparUUID)
		fmt.Printf("    MAC: %s\n", macAddress)
		fmt.Printf("    Location Code: '%s' (length: %d)\n", locationCode, len(locationCode))
		fmt.Printf("    Volumes: %v\n", volumes)
	}

	fmt.Printf("✓ LPAR provisioned successfully: %s (UUID: %s, MAC: %s)\n", node.Hostname, lparUUID, macAddress)
	return nil
}

// ensureLPARPoweredOff checks if LPAR is powered off, powers it off if running
func (l *LPARProvisioner) ensureLPARPoweredOff(lparUUID, hostname string) error {
	lparDetailed, err := l.hmcClient.GetLogicalPartitionDetailed(lparUUID, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get LPAR state: %w", err)
	}

	if lparDetailed.PartitionState == "running" {
		fmt.Printf("    LPAR is running, powering off before attachment...\n")
		// Power off the LPAR (signature: lparUUID, operation, immediate, verbose)

		shutdownOpt := flag.String("shutdown-option", "Immediate", "Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry")
		restart := flag.Bool("restart", false, "Restart the partition after powering off")

		fmt.Printf("    Powering off LPAR for profile query...\n")
		_, err = l.hmcClient.PowerOffPartition(lparUUID, *shutdownOpt, *restart, l.ctx.Verbose)
		if err != nil {
			return fmt.Errorf("failed to power off LPAR: %w", err)
		}

		// Wait for LPAR to power off
		fmt.Printf("    Waiting for LPAR to power off...\n")
		time.Sleep(30 * time.Second)

		// Verify it's powered off
		lparDetailed, err = l.hmcClient.GetLogicalPartitionDetailed(lparUUID, l.ctx.Verbose)
		if err != nil {
			return fmt.Errorf("failed to verify LPAR state: %w", err)
		}
		if lparDetailed.PartitionState == "running" {
			return fmt.Errorf("LPAR still running after power off attempt")
		}
		fmt.Printf("    ✓ LPAR powered off\n")
	} else {
		fmt.Printf("    LPAR is already powered off (state: %s)\n", lparDetailed.PartitionState)
	}

	return nil
}

// saveStateAfterLPARCreation saves state after LPAR creation
func (l *LPARProvisioner) saveStateAfterLPARCreation(node NodeInfo, lparUUID, systemUUID string, powerSystem PowerSystem) error {
	createdAt := time.Now().Format(time.RFC3339)
	l.ctx.State.CreatedLPARs[node.Hostname] = LPARState{
		UUID:              lparUUID,
		Name:              node.Hostname,
		SystemName:        node.SystemName,
		SystemUUID:        systemUUID,
		IP:                node.IP,
		Status:            "lpar_created",
		ProcessorUnits:    node.LPAR.Processor.Units,
		VirtualProcessors: node.LPAR.Processor.VirtualProcs,
		MemoryMB:          node.LPAR.Memory.DesiredMB,
		VLanID:            powerSystem.VlanID,
		VswitchName:       powerSystem.VswitchName,
		CreatedAt:         createdAt,
		LastModified:      createdAt,
	}

	return l.saveStateToFile()
}

// saveStateAfterStorageAttachment saves state after storage attachment
func (l *LPARProvisioner) saveStateAfterStorageAttachment(node NodeInfo, lparUUID, systemUUID string, volumes []VolumeState, powerSystem PowerSystem) error {
	var volNames []string
	for _, vol := range volumes {
		volNames = append(volNames, vol.Name)
		l.ctx.State.CreatedVolumes[vol.Name] = vol
	}

	state := l.ctx.State.CreatedLPARs[node.Hostname]
	state.Volumes = volNames
	state.Status = "storage_attached"
	state.LastModified = time.Now().Format(time.RFC3339)
	l.ctx.State.CreatedLPARs[node.Hostname] = state

	return l.saveStateToFile()
}

// saveStateAfterNetworkAttachment saves state after network adapter attachment
func (l *LPARProvisioner) saveStateAfterNetworkAttachment(node NodeInfo, lparUUID, systemUUID string, volumes []VolumeState, macAddress, locationCode string, powerSystem PowerSystem) error {
	state := l.ctx.State.CreatedLPARs[node.Hostname]
	state.MACAddress = macAddress
	state.LocationCode = locationCode
	state.Status = "network_attached"
	state.LastModified = time.Now().Format(time.RFC3339)
	l.ctx.State.CreatedLPARs[node.Hostname] = state

	return l.saveStateToFile()
}

// saveStateAfterProfileSave saves state after profile save
func (l *LPARProvisioner) saveStateAfterProfileSave(node NodeInfo, lparUUID, systemUUID string, volumes []VolumeState, macAddress, locationCode string, powerSystem PowerSystem) error {
	state := l.ctx.State.CreatedLPARs[node.Hostname]
	state.ProfileName = "default_profile"
	state.Status = "profile_saved"
	state.LastModified = time.Now().Format(time.RFC3339)
	l.ctx.State.CreatedLPARs[node.Hostname] = state

	return l.saveStateToFile()
}

// saveStateToFile saves the deployment state to a JSON file
func (l *LPARProvisioner) saveStateToFile() error {
	stateFile := GetStateFilePath(l.ctx.Name)

	l.ctx.State.Timestamp = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(l.ctx.State, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Create backup of existing state file
	if _, err := os.Stat(stateFile); err == nil {
		backupFile := stateFile + ".backup"
		existingData, readErr := os.ReadFile(stateFile)
		if readErr == nil {
			_ = os.WriteFile(backupFile, existingData, 0644)
		}
	}

	// Write new state file atomically
	tempFile := stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tempFile, stateFile); err != nil {
		return fmt.Errorf("failed to rename temp state file: %w", err)
	}

	return nil
}

// createLPAR creates a single LPAR (partition only, no adapters or storage)
func (l *LPARProvisioner) createLPAR(node NodeInfo) (string, string, error) {
	fmt.Printf("  Creating LPAR: %s on system %s...\n", node.Hostname, node.SystemName)

	// Get managed system UUID
	systemUUID, _, err := l.hmcClient.GetManagedSystemByName(node.SystemName, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get managed system %s: %w", node.SystemName, err)
	}

	// Check if LPAR already exists
	existingLPARs, err := l.hmcClient.GetLogicalPartitionsQuickAll(systemUUID, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to check existing LPARs: %w", err)
	}

	for _, lpar := range existingLPARs {
		if lpar.PartitionName == node.Hostname {
			return "", "", fmt.Errorf("LPAR with name '%s' already exists", node.Hostname)
		}
	}

	// Create LPAR request
	req := hmc.CreateLparRequest{
		Name:             node.Hostname,
		OsType:           "AIX/Linux",
		MinMem:           node.LPAR.Memory.MinMB,
		DesiredMem:       node.LPAR.Memory.DesiredMB,
		MaxMem:           node.LPAR.Memory.MaxMB,
		MinProcUnits:     node.LPAR.Processor.MinUnits,
		DesiredProcUnits: node.LPAR.Processor.Units,
		MaxProcUnits:     node.LPAR.Processor.MaxUnits,
		MinVcpus:         node.LPAR.Processor.MinProcs,
		DesiredVcpus:     node.LPAR.Processor.VirtualProcs,
		MaxVcpus:         node.LPAR.Processor.MaxProcs,
		SharingMode:      "uncapped",
	}

	fmt.Printf("    Memory: %d MB (min: %d, max: %d)\n", req.DesiredMem, req.MinMem, req.MaxMem)
	fmt.Printf("    Processors: %.2f units (min: %.2f, max: %.2f)\n", req.DesiredProcUnits, req.MinProcUnits, req.MaxProcUnits)
	fmt.Printf("    Virtual CPUs: %d (min: %d, max: %d)\n", req.DesiredVcpus, req.MinVcpus, req.MaxVcpus)
	fmt.Printf("    Sharing Mode: %s\n", req.SharingMode)

	// Create LPAR
	lparDetails, err := l.hmcClient.CreateLogicalPartition(systemUUID, req, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to create LPAR: %w", err)
	}

	lparUUID := lparDetails.MetadataID
	fmt.Printf("  ✓ LPAR created successfully (UUID: %s)\n", lparUUID)

	return lparUUID, systemUUID, nil
}

// attachNetworkAdapter creates and attaches a network adapter to the LPAR
func (l *LPARProvisioner) attachNetworkAdapter(node NodeInfo, systemUUID, lparUUID string) (string, string, error) {
	fmt.Printf("  Attaching network adapter to LPAR: %s...\n", node.Hostname)

	// Get power system config for VLAN info
	powerSystem, err := l.ctx.ClusterConfig.GetPowerSystem(node.SystemName)
	if err != nil {
		return "", "", fmt.Errorf("failed to get power system config: %w", err)
	}

	// Create network adapter and capture MAC address and location code
	macAddress, locationCode, err := l.createNetworkAdapter(systemUUID, lparUUID, powerSystem.VswitchName, powerSystem.VlanID)
	if err != nil {
		return "", "", fmt.Errorf("failed to create network adapter: %w", err)
	}

	fmt.Printf("  ✓ Network adapter created (MAC: %s, Location: %s)\n", macAddress, locationCode)

	if l.ctx.Verbose {
		fmt.Printf("  [DEBUG] Network adapter details:\n")
		fmt.Printf("    MAC Address: %s\n", macAddress)
		fmt.Printf("    Location Code: '%s' (length: %d)\n", locationCode, len(locationCode))
	}

	return macAddress, locationCode, nil
}

// createNetworkAdapter creates a network adapter for the LPAR and returns MAC address and location code
func (l *LPARProvisioner) createNetworkAdapter(systemUUID, lparUUID, vswitchName string, vlanID int) (string, string, error) {
	fmt.Printf("  Creating network adapter (vSwitch: %s, VLAN: %d)...\n", vswitchName, vlanID)

	// Get virtual switch UUID
	switches, err := l.hmcClient.GetVirtualSwitchQuickAll(systemUUID, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get virtual switches: %w", err)
	}

	var vswitchUUID string
	for _, sw := range switches {
		if strings.EqualFold(sw.SwitchName, vswitchName) {
			vswitchUUID = sw.UUID
			break
		}
	}

	if vswitchUUID == "" {
		return "", "", fmt.Errorf("virtual switch '%s' not found", vswitchName)
	}

	// Create network adapter
	adapter, err := l.hmcClient.CreateClientNetworkAdapter(systemUUID, lparUUID, vswitchUUID, vlanID, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to create network adapter: %w", err)
	}

	// Get MAC address and location code from adapter
	macAddress := hmc.FormatMACAddress(adapter.MACAddress)
	locationCode := adapter.LocationCode

	if l.ctx.Verbose {
		fmt.Printf("  [DEBUG] Adapter details from HMC:\n")
		fmt.Printf("    Raw MAC: %s\n", adapter.MACAddress)
		fmt.Printf("    Formatted MAC: %s\n", macAddress)
		fmt.Printf("    Location Code: '%s'\n", locationCode)
		fmt.Printf("    Virtual Slot: %s\n", adapter.VirtualSlotNumber)
	}

	fmt.Printf("  ✓ Network adapter created:\n")
	fmt.Printf("    MAC Address:   %s\n", macAddress)
	fmt.Printf("    Location Code: %s\n", locationCode)
	fmt.Printf("    Virtual Slot:  %s\n", adapter.VirtualSlotNumber)

	return macAddress, locationCode, nil
}

// attachStorage attaches storage volumes to the LPAR and returns a list of enriched VolumeStates
func (l *LPARProvisioner) attachStorage(node NodeInfo, lparUUID string) ([]VolumeState, error) {
	fmt.Printf("  Attaching storage to LPAR: %s...\n", node.Hostname)

	storageType := l.ctx.ClusterConfig.Storage.Type

	switch storageType {
	case "vios":
		return l.attachVIOSStorage(node, lparUUID)
	case "svc":
		return l.attachSVCStorage(node, lparUUID)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// attachVIOSStorage attaches VIOS-based storage and tracks deep metadata
func (l *LPARProvisioner) attachVIOSStorage(node NodeInfo, lparUUID string) ([]VolumeState, error) {
	var createdVolumes []VolumeState

	if len(l.ctx.ClusterConfig.Storage.VIOS) == 0 {
		return nil, fmt.Errorf("no VIOS configuration found")
	}

	// Ensure VIOS configuration is complete (auto-discover if needed)
	if err := l.ensureVIOSConfig(); err != nil {
		return nil, fmt.Errorf("failed to ensure VIOS configuration: %w", err)
	}

	viosConfig := l.ctx.ClusterConfig.Storage.VIOS[0]
	systemUUID, _, err := l.hmcClient.GetManagedSystemByName(viosConfig.SystemName, l.ctx.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to get system UUID: %w", err)
	}

	viosList, err := l.hmcClient.GetVirtualIOServersQuick(systemUUID, l.ctx.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to get VIOS list: %w", err)
	}

	var viosUUID string
	for _, vios := range viosList {
		if vios.PartitionName == viosConfig.VIOSName {
			viosUUID = vios.UUID
			break
		}
	}

	if viosUUID == "" {
		return nil, fmt.Errorf("VIOS '%s' not found", viosConfig.VIOSName)
	}

	// Create boot disk
	bootDiskName := l.generateDiskName(node.Name, "boot")
	bootDiskSizeMB := node.LPAR.Storage.BootDiskGB * 1024

	fmt.Printf("    Creating boot disk: %s (%d GB) on %s/%s...\n",
		bootDiskName, node.LPAR.Storage.BootDiskGB, viosConfig.VIOSName, viosConfig.VolumeGroup)

	// Debug: Print all parameters being passed to CreateVirtualDisk
	if l.ctx.Verbose {
		fmt.Printf("    [DEBUG] CreateVirtualDisk parameters:\n")
		fmt.Printf("      SystemName: '%s'\n", viosConfig.SystemName)
		fmt.Printf("      VIOS UUID: '%s'\n", viosUUID)
		fmt.Printf("      VIOS Name: '%s'\n", viosConfig.VIOSName)
		fmt.Printf("      Volume Group: '%s'\n", viosConfig.VolumeGroup)
		fmt.Printf("      Disk Name: '%s'\n", bootDiskName)
		fmt.Printf("      Size MB: %d\n", bootDiskSizeMB)
	}

	err = l.hmcClient.CreateVirtualDisk(viosConfig.SystemName, viosUUID, viosConfig.VIOSName, viosConfig.VolumeGroup, bootDiskName, bootDiskSizeMB, l.ctx.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create boot disk: %w", err)
	}

	bootVolState := VolumeState{
		Name:        bootDiskName,
		SizeGB:      node.LPAR.Storage.BootDiskGB,
		AttachedTo:  node.Hostname,
		Status:      "mapped",
		StorageType: "vios",
		ViosName:    viosConfig.VIOSName,
		VolumeGroup: viosConfig.VolumeGroup,
	}
	createdVolumes = append(createdVolumes, bootVolState)
	fmt.Printf("    ✓ Boot disk created: %s\n", bootDiskName)

	// Map boot disk to LPAR
	_, err = l.hmcClient.CreateVirtualDiskMaps(systemUUID, viosUUID, lparUUID, []string{bootDiskName}, l.ctx.Verbose)
	if err != nil && !strings.Contains(err.Error(), "RMC") {
		return nil, fmt.Errorf("failed to map boot disk: %w", err)
	}

	// Create data disk if specified
	if node.LPAR.Storage.DataDiskGB > 0 {
		dataDiskName := l.generateDiskName(node.Name, "data")
		dataDiskSizeMB := node.LPAR.Storage.DataDiskGB * 1024

		err = l.hmcClient.CreateVirtualDisk(viosConfig.SystemName, viosUUID, viosConfig.VIOSName, viosConfig.VolumeGroup, dataDiskName, dataDiskSizeMB, l.ctx.Verbose)
		if err != nil {
			return createdVolumes, fmt.Errorf("failed to create data disk: %w", err)
		}

		dataVolState := VolumeState{
			Name:        dataDiskName,
			SizeGB:      node.LPAR.Storage.DataDiskGB,
			AttachedTo:  node.Hostname,
			Status:      "mapped",
			StorageType: "vios",
			ViosName:    viosConfig.VIOSName,
			VolumeGroup: viosConfig.VolumeGroup,
		}
		createdVolumes = append(createdVolumes, dataVolState)

		// Map data disk to LPAR
		_, err = l.hmcClient.CreateVirtualDiskMaps(systemUUID, viosUUID, lparUUID, []string{dataDiskName}, l.ctx.Verbose)
		if err != nil && !strings.Contains(err.Error(), "RMC") {
			return createdVolumes, fmt.Errorf("failed to map data disk: %w", err)
		}
	}

	return createdVolumes, nil
}

// Ensure attachSVCStorage also matches the new signature:
func (l *LPARProvisioner) attachSVCStorage(node NodeInfo, lparUUID string) ([]VolumeState, error) {
	return nil, fmt.Errorf("SVC storage attachment not yet implemented")
}

// captureMACAddress captures the MAC address of the LPAR's network adapter
func (l *LPARProvisioner) captureMACAddress(node NodeInfo, lparUUID string) (string, error) {
	fmt.Printf("  Capturing MAC address for LPAR: %s...\n", node.Hostname)

	systemUUID, _, err := l.hmcClient.GetManagedSystemByName(node.SystemName, l.ctx.Verbose)
	if err != nil {
		return "", fmt.Errorf("failed to get system UUID: %w", err)
	}

	adapters, err := l.hmcClient.GetClientNetworkAdapters(systemUUID, lparUUID, l.ctx.Verbose)
	if err != nil {
		return "", fmt.Errorf("failed to get network adapters: %w", err)
	}

	if len(adapters) == 0 {
		return "", fmt.Errorf("no network adapters found for LPAR")
	}

	macAddress := hmc.FormatMACAddress(adapters[0].MACAddress)
	fmt.Printf("  ✓ MAC address captured: %s\n", macAddress)

	return macAddress, nil
}

// updateNodeMACAddress updates the MAC address in the node configuration
func (l *LPARProvisioner) updateNodeMACAddress(hostname, macAddress string) error {
	if l.ctx.ClusterConfig.SNONode != nil && l.ctx.ClusterConfig.SNONode.Hostname == hostname {
		l.ctx.ClusterConfig.SNONode.MACAddress = macAddress
		return nil
	}

	if l.ctx.ClusterConfig.Bootstrap != nil && l.ctx.ClusterConfig.Bootstrap.Hostname == hostname {
		l.ctx.ClusterConfig.Bootstrap.MACAddress = macAddress
		return nil
	}

	if l.ctx.ClusterConfig.Masters != nil {
		for i := range l.ctx.ClusterConfig.Masters.Nodes {
			if l.ctx.ClusterConfig.Masters.Nodes[i].Hostname == hostname {
				l.ctx.ClusterConfig.Masters.Nodes[i].MACAddress = macAddress
				return nil
			}
		}
	}

	if l.ctx.ClusterConfig.Workers != nil {
		for i := range l.ctx.ClusterConfig.Workers.Nodes {
			if l.ctx.ClusterConfig.Workers.Nodes[i].Hostname == hostname {
				l.ctx.ClusterConfig.Workers.Nodes[i].MACAddress = macAddress
				return nil
			}
		}
	}

	return fmt.Errorf("node with hostname '%s' not found in configuration", hostname)
}

// saveProfile saves the LPAR configuration to the default profile
func (l *LPARProvisioner) saveProfile(lparUUID, hostname string) error {
	fmt.Printf("    Saving configuration to default_profile...\n")

	profileName := "default_profile"

	if l.ctx.Verbose {
		fmt.Printf("    Profile name: %s\n", profileName)
	}

	err := l.hmcClient.SaveCurrentLparConfig(lparUUID, profileName, true, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to save LPAR configuration: %w", err)
	}

	fmt.Printf("    ✓ Configuration saved to profile: %s\n", profileName)

	return nil
}

// checkProfileBootMode retrieves and displays the boot mode from the LPAR profile
func (l *LPARProvisioner) checkProfileBootMode(lparUUID, profileUUID, hostname string) error {
	fmt.Printf("    Checking profile boot mode for %s...\n", hostname)

	profile, err := l.hmcClient.GetLogicalPartitionProfile(lparUUID, profileUUID, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	fmt.Printf("    📋 Profile Information:\n")
	fmt.Printf("       Profile Name: %s\n", profile.ProfileName)
	fmt.Printf("       Profile Type: %s\n", profile.ProfileType)
	fmt.Printf("       Boot Mode: %s\n", profile.BootMode)
	fmt.Printf("       Auto Start: %s\n", profile.AutoStart)

	if profile.BootMode != "" && profile.BootMode != "Normal" {
		fmt.Printf("    ⚠️  WARNING: Boot mode is '%s' (not 'norm')\n", profile.BootMode)
		fmt.Printf("       This may cause the LPAR to network boot on subsequent reboots\n")
		fmt.Printf("       instead of booting from the installed OS on disk.\n")
	} else if profile.BootMode == "norm" {
		fmt.Printf("    ✓ Boot mode is 'norm' - LPAR will boot from disk\n")
	} else {
		fmt.Printf("    ℹ️  Boot mode is empty - will use default behavior\n")
	}

	return nil
}

// setBootDeviceOrder sets the boot mode to norm in the profile using HMC CLI via SSH
func (l *LPARProvisioner) setBootDeviceOrder(systemName, lparName, profileName string) error {
	if l.ctx.Verbose {
		fmt.Printf("    [DEBUG] Setting boot mode to 'norm' for profile '%s' of LPAR '%s' on system '%s'\n",
			profileName, lparName, systemName)
	}

	// Use HMC CLI to set boot_mode in the profile
	// Format: chsyscfg -r prof -m <system> -i "name=<profile>,lpar_name=<lpar>,boot_mode=norm"
	// This ensures the LPAR boots from disk instead of continuing network boot
	cliCmd := fmt.Sprintf("chsyscfg -r prof -m %s -i \"name=%s,lpar_name=%s,boot_mode=norm\"",
		systemName, profileName, lparName)

	if l.ctx.Verbose {
		fmt.Printf("    [DEBUG] Executing HMC CLI command via SSH: %s\n", cliCmd)
	}

	// Execute via SSH instead of REST API CLIRunner
	output, err := hmc.CliRunnerViaSsh(
		l.ctx.HMC.IP,
		l.ctx.HMC.Username,
		l.ctx.HMC.Password,
		cliCmd,
		l.ctx.Verbose,
	)
	if err != nil {
		return fmt.Errorf("failed to set boot mode in profile: %v (Output: %s)", err, output)
	}

	if l.ctx.Verbose {
		fmt.Printf("    [DEBUG] Boot mode set to 'norm' successfully. Output: %s\n", output)
	}

	return nil
}

// PowerOnAll powers on all LPARs for the cluster using network boot
func (l *LPARProvisioner) PowerOnAll() error {
	fmt.Printf("Powering on LPARs for cluster '%s' with network boot...\n", l.ctx.Name)

	nodes := l.ctx.ClusterConfig.GetAllNodes()

	for _, node := range nodes {
		if l.ctx.State == nil || l.ctx.State.CreatedLPARs[node.Hostname].UUID == "" {
			return fmt.Errorf("LPAR not found in state for node: %s", node.Hostname)
		}

		lparState := l.ctx.State.CreatedLPARs[node.Hostname]

		fmt.Printf("  Network booting LPAR: %s (IP: %s)...\n", node.Hostname, node.IP)

		if err := l.networkBootLPAR(node, lparState); err != nil {
			return fmt.Errorf("failed to network boot LPAR %s: %w", node.Hostname, err)
		}

		lparState.Status = "powered_on"
		l.ctx.State.CreatedLPARs[node.Hostname] = lparState

		fmt.Printf("  ✓ LPAR network booted: %s\n", node.Hostname)

		time.Sleep(30 * time.Second)
	}

	fmt.Printf("All LPARs network booted successfully for cluster '%s'\n", l.ctx.Name)
	return nil
}

// networkBootLPAR performs network boot for a single LPAR using a fallback loop for terminal ports
func (l *LPARProvisioner) networkBootLPAR(node NodeInfo, lparState LPARState) error {
	lparDetailed, err := l.hmcClient.GetLogicalPartitionDetailed(lparState.UUID, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get detailed LPAR information: %w", err)
	}

	if lparDetailed.PartitionState == "running" {
		return fmt.Errorf("LPAR is already running. You must power it off before initiating a netboot")
	}

	fmt.Printf("    LPAR state: %s\n", lparDetailed.PartitionState)

	profileHref := lparDetailed.AssociatedPartitionProfile.Href
	if profileHref == "" {
		return fmt.Errorf("no associated partition profile found for LPAR")
	}
	profileUUID := profileHref[len(profileHref)-36:]

	// Check the profile's boot mode (profile was already saved in provisionSingleNode Step 4)
	fmt.Printf("    Checking profile boot mode...\n")
	if err := l.checkProfileBootMode(lparState.UUID, profileUUID, node.Hostname); err != nil {
		fmt.Printf("    ⚠️  Warning: Could not check profile boot mode: %v\n", err)
	}

	// Verify the LPAR has a network adapter before attempting netboot
	fmt.Printf("    Verifying network adapter exists...\n")
	adapters, err := l.hmcClient.GetClientNetworkAdapters(lparState.SystemUUID, lparState.UUID, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get network adapters: %w", err)
	}

	if len(adapters) == 0 {
		return fmt.Errorf("LPAR %s has no network adapters attached\n"+
			"This usually means the LPAR was created but network adapter creation failed.\n"+
			"Solution:\n"+
			"  1. Delete the LPAR from HMC\n"+
			"  2. Delete deployment-state-%s.json\n"+
			"  3. Re-run deployment from create_lpars phase:\n"+
			"     ./main -command deploy -config config.yaml -cluster %s -phases create_lpars,setup_dnsmasq,power_on",
			node.Hostname, l.ctx.Name, l.ctx.Name)
	}

	// Get MAC address from adapter
	macAddress := hmc.FormatMACAddress(adapters[0].MACAddress)

	fmt.Printf("    ✓ Network adapter found\n")
	fmt.Printf("    MAC address: %s\n", macAddress)

	if l.ctx.Verbose {
		fmt.Printf("    [DEBUG] Adapter details:\n")
		fmt.Printf("      Virtual Slot: %s\n", adapters[0].VirtualSlotNumber)
		fmt.Printf("      Port VLAN ID: %s\n", adapters[0].PortVLANID)
		fmt.Printf("      Base Location Code: %s\n", adapters[0].LocationCode)
	}

	// =========================================================================
	// STEP 1: Power cycle LPAR to make adapters visible to firmware
	// =========================================================================
	fmt.Printf("    Power cycling LPAR to register adapters with firmware...\n")

	// Power on the LPAR briefly to register adapters
	fmt.Printf("    Powering on LPAR...\n")
	_, err = l.hmcClient.PowerOnPartition(lparState.UUID, &hmc.PowerOnOptions{
		ProfileUUID: profileUUID,
		BootMode:    "of",
	}, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to power on LPAR for adapter registration: %w", err)
	}

	// Wait for LPAR to fully start and adapters to be visible . REMOVE THIS
	fmt.Println("    ⏳ Waiting 30 seconds for LPAR to start and adapters to register...")
	time.Sleep(10 * time.Second)

	// Power off the LPAR so GetNetworkBootDevices can query it
	shutdownOpt := flag.String("shutdown-option", "Immediate", "Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry")
	restart := flag.Bool("restart", false, "Restart the partition after powering off")

	fmt.Printf("    Powering off LPAR for profile query...\n")
	_, err = l.hmcClient.PowerOffPartition(lparState.UUID, *shutdownOpt, *restart, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to power off LPAR: %w", err)
	}

	// Wait for LPAR to fully start and adapters to be visible . REMOVE THIS
	fmt.Println("    ⏳ Waiting 30 seconds for LPAR to start and adapters to register...")
	time.Sleep(10 * time.Second)

	// Close virtual terminal before querying profile (using SSH)
	fmt.Printf("    Closing virtual terminal before profile query...\n")
	systemName := lparState.SystemName

	if err := l.hmcClient.CloseVirtualTerminalViaSsh(
		l.ctx.HMC.IP,
		l.ctx.HMC.Username,
		l.ctx.HMC.Password,
		systemName,
		node.Hostname,
		l.ctx.Verbose,
	); err != nil {
		if l.ctx.Verbose {
			fmt.Printf("    ℹ️  Note: Could not close virtual terminal (may not be open): %v\n", err)
		}
	} else {
		fmt.Printf("    ✓ Virtual terminal closed\n")
	}

	// =========================================================================
	// STEP 2: Get authoritative location code from profile using GetNetworkBootDevices
	// =========================================================================
	fmt.Printf("    Retrieving network boot device information from profile...\n")

	bootDevices, err := l.hmcClient.GetNetworkBootDevices(lparState.UUID, profileUUID, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get network boot devices from profile: %w", err)
	}

	if len(bootDevices) == 0 {
		return fmt.Errorf("no network boot devices found in profile - this should not happen after power cycle and saving configuration")
	}

	// Use the authoritative location code from the profile (includes port suffix)
	authoritativeLocationCode := bootDevices[0].LocationCode

	fmt.Printf("    ✓ Authoritative location code from profile: %s\n", authoritativeLocationCode)
	if l.ctx.Verbose {
		fmt.Printf("    [DEBUG] Boot device details:\n")
		fmt.Printf("      Device Name: %s\n", bootDevices[0].DeviceName)
		fmt.Printf("      Device Type: %s\n", bootDevices[0].DeviceType)
		fmt.Printf("      Location Code: %s\n", authoritativeLocationCode)
		fmt.Printf("      MAC Address: %s\n", bootDevices[0].MACAddress)
	}

	// =========================================================================
	// STEP 3: Network boot using authoritative location code
	// =========================================================================

	// Close virtual terminal before network boot (using SSH)
	fmt.Printf("    Closing virtual terminal before network boot...\n")

	if err := l.hmcClient.CloseVirtualTerminalViaSsh(
		l.ctx.HMC.IP,
		l.ctx.HMC.Username,
		l.ctx.HMC.Password,
		systemName,
		node.Hostname,
		l.ctx.Verbose,
	); err != nil {
		if l.ctx.Verbose {
			fmt.Printf("    ℹ️  Note: Could not close virtual terminal (may not be open): %v\n", err)
		}
	} else {
		fmt.Printf("    ✓ Virtual terminal closed\n")
	}
	// Wait for LPAR to fully start and adapters to be visible . REMOVE THIS
	fmt.Println("    ⏳ Waiting 30 seconds for LPAR to start and adapters to register...")
	time.Sleep(10 * time.Second)

	fmt.Printf("    Initiating network boot with location code: %s\n", authoritativeLocationCode)

	// KEEP IPs AS 0.0.0.0 FOR DHCP to bypass REST0197/REST019C XML validation
	options := &hmc.PowerOnOptions{
		ProfileUUID:  profileUUID,
		BootMode:     "netboot",
		LocationCode: authoritativeLocationCode,
		ClientIP:     "0.0.0.0",
		ServerIP:     "0.0.0.0",
		Gateway:      "0.0.0.0",
		Netmask:      "0.0.0.0",
	}

	status, bootErr := l.hmcClient.PowerOnPartition(lparState.UUID, options, l.ctx.Verbose)
	if bootErr != nil {
		return fmt.Errorf("failed to execute network boot: %w", bootErr)
	}

	fmt.Printf("    ✓ Network boot initiated successfully (job status: %s)\n", status)

	// Set boot mode to norm in profile to prioritize disk over network
	fmt.Printf("    Setting boot mode to 'norm' in profile...\n")
	profileName := lparState.ProfileName
	if profileName == "" {
		profileName = "default_profile" // Fallback to default
	}
	if err := l.setBootDeviceOrder(systemName, node.Hostname, profileName); err != nil {
		fmt.Printf("    ⚠️  Warning: Could not set boot mode in profile: %v\n", err)
		fmt.Printf("    Note: LPAR may continue to network boot after OS installation\n")
		fmt.Printf("    Manual fix: chsyscfg -r prof -m %s -i \"name=%s,lpar_name=%s,boot_mode=norm\"\n",
			systemName, profileName, node.Hostname)
	} else {
		fmt.Printf("    ✓ Boot mode set to 'norm' (disk will be primary boot device)\n")
	}

	// Check profile boot mode after network boot
	fmt.Printf("    Checking profile boot mode after network boot...\n")
	if err := l.checkProfileBootMode(lparState.UUID, profileUUID, node.Hostname); err != nil {
		fmt.Printf("    ⚠️  Warning: Could not verify profile boot mode after netboot: %v\n", err)
	}

	// Save the profile to persist the boot mode change
	fmt.Printf("    Saving profile to persist boot mode change...\n")
	if err := l.hmcClient.SaveCurrentLparConfig(lparState.UUID, profileName, true, l.ctx.Verbose); err != nil {
		fmt.Printf("    ⚠️  Warning: Could not save profile: %v\n", err)
	} else {
		fmt.Printf("    ✓ Profile saved successfully\n")
	}

	// Update State File
	if lparState, exists := l.ctx.State.CreatedLPARs[node.Hostname]; exists {
		lparState.Status = "powered_on"
		lparState.LocationCode = authoritativeLocationCode
		lparState.LastPoweredOn = time.Now().Format(time.RFC3339)
		lparState.LastModified = time.Now().Format(time.RFC3339)
		l.ctx.State.CreatedLPARs[node.Hostname] = lparState
	}

	return nil
}

// DeleteAll deletes all LPARs and Storage relying strictly on the JSON state file
func (l *LPARProvisioner) DeleteAll() error {
	fmt.Printf("Deleting infrastructure for cluster '%s'...\n", l.ctx.Name)

	if l.ctx.State == nil {
		fmt.Println("  ℹ No state file found. Skipping infrastructure deletion.")
		return nil
	}

	// Maps to hold items that FAILED to delete, so we can save them back to the state file
	retainedLPARs := make(map[string]LPARState)
	retainedVolumes := make(map[string]VolumeState)
	var deletionErrors []string

	// Get system UUID once for all operations
	systemName := l.ctx.ClusterConfig.PowerSystems[0].Name
	sysUUID, _, _ := l.hmcClient.GetManagedSystemByName(systemName, l.ctx.Verbose)

	// =========================================================================
	// 1. CLOSE VIRTUAL TERMINALS & POWER OFF LPARs
	// =========================================================================
	if len(l.ctx.State.CreatedLPARs) > 0 {
		fmt.Println("Step 1: Closing virtual terminals and powering off LPARs...")
		for hostname, lparState := range l.ctx.State.CreatedLPARs {
			// Step 1a: Close virtual terminal first (using SSH)
			fmt.Printf("  Closing virtual terminal for LPAR: %s...\n", hostname)
			if err := l.hmcClient.CloseVirtualTerminalViaSsh(
				l.ctx.HMC.IP,
				l.ctx.HMC.Username,
				l.ctx.HMC.Password,
				systemName,
				hostname,
				l.ctx.Verbose,
			); err != nil { // [cite: 406-407]
				// Non-fatal - terminal might not be open
				if l.ctx.Verbose {
					fmt.Printf("    ℹ️  Note: Could not close virtual terminal (may not be open): %v\n", err)
				}
			} else {
				fmt.Printf("    ✓ Virtual terminal closed\n")
			}

			// Step 1b: Power off the LPAR
			fmt.Printf("  Powering off LPAR: %s...\n", hostname)
			shutdownOpt := flag.String("shutdown-option", "Immediate", "Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry")
			restart := flag.Bool("restart", false, "Restart the partition after powering off")

			fmt.Printf("    Powering off LPAR for profile query...\n")
			_, err := l.hmcClient.PowerOffPartition(lparState.UUID, *shutdownOpt, *restart, l.ctx.Verbose)

			if err != nil && !strings.Contains(err.Error(), "already powered off") && !strings.Contains(err.Error(), "not in a valid state") {
				fmt.Printf("    ⚠ Warning: Could not cleanly power off LPAR: %v\n", err) //
			} else {
				fmt.Printf("    ✅ LPAR powered off\n")
				time.Sleep(10 * time.Second) // Give the HMC a moment to register the power-off [cite: 408]
			}
		}
	}

	// =========================================================================
	// 2. UNMAP STORAGE FROM LPARs
	// =========================================================================
	if len(l.ctx.State.CreatedVolumes) > 0 {
		fmt.Println("\nStep 2: Unmapping storage from LPARs...")

		type viosLparKey struct {
			viosUUID string
			lparUUID string
		}
		volumesByViosLpar := make(map[viosLparKey][]string)
		viosNames := make(map[string]string)

		// Cache VIOS UUIDs
		viosUUIDs := make(map[string]string)
		viosList, _ := l.hmcClient.GetVirtualIOServersQuick(sysUUID, l.ctx.Verbose)
		for _, v := range viosList {
			viosUUIDs[v.PartitionName] = v.UUID
			viosNames[v.UUID] = v.PartitionName
		}

		// Group volumes for batch unmapping
		for volName, volState := range l.ctx.State.CreatedVolumes {
			if volState.StorageType == "vios" && volState.ViosName != "" { // [cite: 408-409]
				viosUUID := viosUUIDs[volState.ViosName]
				if viosUUID != "" {
					for _, lparState := range l.ctx.State.CreatedLPARs {
						key := viosLparKey{viosUUID: viosUUID, lparUUID: lparState.UUID}
						volumesByViosLpar[key] = append(volumesByViosLpar[key], volName)
					}
				}
			}
		}

		// Unmap volumes in batches
		for key, volumes := range volumesByViosLpar {
			viosName := viosNames[key.viosUUID]
			fmt.Printf("  Unmapping %d volume(s) from VIOS %s...\n", len(volumes), viosName)

			result, err := l.hmcClient.DeleteVirtualDiskMaps(sysUUID, key.viosUUID, key.lparUUID, volumes, l.ctx.Verbose) // [cite: 409]
			if err != nil {
				fmt.Printf("    ⚠ Warning: Failed to unmap volumes: %v\n", err)
			} else {
				fmt.Printf("    ✅ Volumes unmapped: %s\n", result)
			}
		}
	}

	// =========================================================================
	// 3. DELETE STORAGE VOLUMES
	// =========================================================================
	if len(l.ctx.State.CreatedVolumes) > 0 {
		fmt.Println("\nStep 3: Deleting storage volumes...")

		// Cache VIOS UUIDs
		viosUUIDs := make(map[string]string)
		viosList, _ := l.hmcClient.GetVirtualIOServersQuick(sysUUID, l.ctx.Verbose)
		for _, v := range viosList {
			viosUUIDs[v.PartitionName] = v.UUID // [cite: 410]
		}

		for volName, volState := range l.ctx.State.CreatedVolumes {
			fmt.Printf("  Deleting volume: %s...\n", volName)

			if volState.StorageType == "vios" {
				viosUUID := viosUUIDs[volState.ViosName]
				if viosUUID != "" {
					// Delete the Virtual Disk from the VIOS
					if err := l.hmcClient.DeleteVirtualDisk(systemName, viosUUID, volName, l.ctx.Verbose); err != nil { // [cite: 410-411]
						fmt.Printf("    ⚠ Failed to delete disk %s: %v\n", volName, err)
						retainedVolumes[volName] = volState
						deletionErrors = append(deletionErrors, fmt.Sprintf("Volume: %s", volName))
					} else {
						fmt.Printf("    ✅ Deleted virtual disk: %s\n", volName)
					}
				} else {
					fmt.Printf("    ⚠ Could not find VIOS %s to delete disk %s\n", volState.ViosName, volName) // [cite: 411]
					retainedVolumes[volName] = volState
					deletionErrors = append(deletionErrors, fmt.Sprintf("Volume: %s (VIOS missing)", volName))
				}
			} else if volState.StorageType == "svc" {
				fmt.Printf("    ⚠ SVC deletion using state file pending SDK integration for volume %s\n", volName) // [cite: 411]
				retainedVolumes[volName] = volState
				deletionErrors = append(deletionErrors, fmt.Sprintf("Volume: %s (SVC pending)", volName))
			}
		}
	}

	// =========================================================================
	// 4. DELETE LPARs
	// =========================================================================
	if len(l.ctx.State.CreatedLPARs) > 0 {
		fmt.Println("\nStep 4: Deleting LPARs...")
		for hostname, lparState := range l.ctx.State.CreatedLPARs {
			fmt.Printf("  Deleting LPAR: %s...\n", hostname)
			if err := l.hmcClient.DeleteLogicalPartition(lparState.UUID, l.ctx.Verbose); err != nil { //
				fmt.Printf("    ⚠ Failed to delete LPAR %s: %v\n", hostname, err)
				retainedLPARs[hostname] = lparState
				deletionErrors = append(deletionErrors, fmt.Sprintf("LPAR: %s", hostname))
			} else {
				fmt.Printf("    ✅ LPAR deleted successfully\n") // [cite: 412]
			}
		}
	}

	// Safely overwrite the state maps with ONLY the items that failed to delete
	l.ctx.State.CreatedLPARs = retainedLPARs
	l.ctx.State.CreatedVolumes = retainedVolumes

	if len(deletionErrors) > 0 {
		return fmt.Errorf("infrastructure deletion completed with errors. The following resources remain and have been preserved in state: %s", strings.Join(deletionErrors, ", "))
	}

	fmt.Println("\n✅ Infrastructure deletion completed cleanly")
	return nil
}

// deleteStorage routes the storage deletion based on type
func (l *LPARProvisioner) deleteStorage(lparState LPARState, systemUUID string) error {
	storageType := l.ctx.ClusterConfig.Storage.Type

	switch storageType {
	case "vios":
		return l.deleteVIOSStorage(lparState, systemUUID)
	case "svc":
		return l.deleteSVCStorage(lparState)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// deleteVIOSStorage deletes VIOS-based storage mapped to the LPAR
func (l *LPARProvisioner) deleteVIOSStorage(lparState LPARState, systemUUID string) error {
	if len(l.ctx.ClusterConfig.Storage.VIOS) == 0 {
		return fmt.Errorf("no VIOS configuration found")
	}

	if len(lparState.Volumes) == 0 {
		fmt.Printf("      No volumes recorded in state for %s\n", lparState.Name)
		return nil
	}

	viosConfig := l.ctx.ClusterConfig.Storage.VIOS[0]

	viosList, err := l.hmcClient.GetVirtualIOServersQuick(systemUUID, l.ctx.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get VIOS list: %w", err)
	}

	var viosUUID string
	for _, vios := range viosList {
		if vios.PartitionName == viosConfig.VIOSName {
			viosUUID = vios.UUID
			break
		}
	}

	if viosUUID == "" {
		return fmt.Errorf("VIOS '%s' not found", viosConfig.VIOSName)
	}

	// We now read the exact disk names stored during attachment
	disksToDelete := lparState.Volumes

	for _, diskName := range disksToDelete {
		fmt.Printf("      Removing virtual disk: %s...\n", diskName)

		err := l.hmcClient.DeleteVirtualDisk(viosConfig.SystemName, viosUUID, diskName, l.ctx.Verbose)
		if err != nil {
			if !strings.Contains(err.Error(), "does not exist") {
				fmt.Printf("      ⚠ Could not delete disk %s: %v\n", diskName, err)
			}
		} else {
			fmt.Printf("      ✓ Deleted virtual disk: %s\n", diskName)
		}
	}

	return nil
}

// deleteSVCStorage deletes SVC-based storage mapped to the LPAR
func (l *LPARProvisioner) deleteSVCStorage(lparState LPARState) error {
	svcConfig := l.ctx.ClusterConfig.Storage.SVC
	if svcConfig == nil {
		return fmt.Errorf("no SVC configuration found")
	}

	disksToDelete := lparState.Volumes
	if len(disksToDelete) == 0 {
		fmt.Printf("      No SVC volumes recorded in state for %s\n", lparState.Name)
		return nil
	}

	shortName := lparState.Name
	if len(shortName) > 9 {
		shortName = shortName[:9]
	}

	fmt.Printf("    Cleaning up SVC storage on %s...\n", svcConfig.IP)

	for _, diskName := range disksToDelete {
		fmt.Printf("      Removing SVC volume: %s...\n", diskName)

		unmapCmd := fmt.Sprintf("svctask rmvdiskhostmap -host %s %s", shortName, diskName)
		deleteCmd := fmt.Sprintf("svctask rmvdisk -force %s", diskName)

		if l.ctx.Verbose {
			fmt.Printf("      [DEBUG] Would execute: %s\n", unmapCmd)
			fmt.Printf("      [DEBUG] Would execute: %s\n", deleteCmd)
		}

		fmt.Printf("      ✓ [Simulated] Deleted SVC volume: %s\n", diskName)
	}

	return nil
}
