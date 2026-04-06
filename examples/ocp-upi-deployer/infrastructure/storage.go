package infrastructure

import (
	"fmt"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

// discoverVIOSAndVolumeGroup automatically discovers an active VIOS and its volume group
// when they are not specified in the configuration
func (l *LPARProvisioner) discoverVIOSAndVolumeGroup(systemName string) (string, string, error) {
	if l.ctx.Verbose {
		fmt.Println("    Auto-discovering VIOS and volume group...")
	}

	// Get system UUID
	systemUUID, _, err := l.hmcClient.GetManagedSystemByName(systemName, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get system UUID: %w", err)
	}

	// Get all VIOS servers
	viosList, err := l.hmcClient.GetVirtualIOServersQuick(systemUUID, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get VIOS list: %w", err)
	}

	if len(viosList) == 0 {
		return "", "", fmt.Errorf("no VIOS servers found on system %s", systemName)
	}

	// Find an active VIOS (state = "running")
	var activeVIOS *hmc.VIOSQuick
	for i := range viosList {
		if viosList[i].PartitionState == "running" && viosList[i].RMCState == "active" {
			activeVIOS = &viosList[i]
			break
		}
	}

	// If no active VIOS with RMC active, try just running state
	if activeVIOS == nil {
		for i := range viosList {
			if viosList[i].PartitionState == "running" {
				activeVIOS = &viosList[i]
				break
			}
		}
	}

	if activeVIOS == nil {
		return "", "", fmt.Errorf("no active VIOS found on system %s (checked %d VIOS servers)", systemName, len(viosList))
	}

	viosName := activeVIOS.PartitionName
	viosUUID := activeVIOS.UUID

	if l.ctx.Verbose {
		fmt.Printf("    ✓ Found active VIOS: %s (UUID: %s, State: %s, RMC: %s)\n",
			viosName, viosUUID, activeVIOS.PartitionState, activeVIOS.RMCState)
	}

	// Get volume groups from the active VIOS
	volumeGroups, err := l.hmcClient.GetVolumeGroups(viosUUID, l.ctx.Verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get volume groups from VIOS %s: %w", viosName, err)
	}

	if len(volumeGroups) == 0 {
		return "", "", fmt.Errorf("no volume groups found on VIOS %s", viosName)
	}

	// Find the first volume group with available space
	// IMPORTANT: Skip 'rootvg' as it's the VIOS system volume group and should not be used for client LPARs
	var selectedVG *hmc.VolumeGroup
	for i := range volumeGroups {
		vg := &volumeGroups[i]
		// Skip rootvg - it's reserved for VIOS system use
		if vg.GroupName == "rootvg" {
			if l.ctx.Verbose {
				fmt.Printf("    ⚠️  Skipping 'rootvg' (VIOS system volume group)\n")
			}
			continue
		}
		// Check if volume group has free space (FreeSpace != "0")
		if vg.FreeSpace != "" && vg.FreeSpace != "0" {
			selectedVG = vg
			break
		}
	}

	// If no suitable VG found, return error instead of using rootvg
	if selectedVG == nil {
		return "", "", fmt.Errorf("no suitable volume group found on VIOS %s (rootvg is reserved for VIOS system use). Please create a volume group for client storage", viosName)
	}

	volumeGroupName := selectedVG.GroupName

	if l.ctx.Verbose {
		fmt.Printf("    ✓ Selected volume group: %s (Free: %s MB, Capacity: %s MB)\n",
			volumeGroupName, selectedVG.FreeSpace, selectedVG.GroupCapacity)
	}

	fmt.Printf("    ✓ Auto-discovered: VIOS=%s, VolumeGroup=%s\n", viosName, volumeGroupName)

	return viosName, volumeGroupName, nil
}

// ensureVIOSConfig ensures VIOS configuration is complete by auto-discovering missing values
// and validates that rootvg is not used for client storage
func (l *LPARProvisioner) ensureVIOSConfig() error {
	if len(l.ctx.ClusterConfig.Storage.VIOS) == 0 {
		return fmt.Errorf("no VIOS configuration found")
	}

	viosConfig := &l.ctx.ClusterConfig.Storage.VIOS[0]

	// Check if we need discovery
	needsDiscovery := viosConfig.VIOSName == "" || viosConfig.VolumeGroup == ""

	if !needsDiscovery {
		// Both VIOS and VG are specified - just use them
		if l.ctx.Verbose {
			fmt.Printf("    Using configured VIOS: %s, VolumeGroup: %s\n",
				viosConfig.VIOSName, viosConfig.VolumeGroup)
		}
		return nil
	}

	// Need to discover VIOS and/or volume group
	fmt.Println("    VIOS name or volume group not specified in configuration")

	// If VIOS is specified but VG is empty, discover VG for this specific VIOS
	if viosConfig.VIOSName != "" && viosConfig.VolumeGroup == "" {
		fmt.Printf("    Discovering volume group for VIOS: %s\n", viosConfig.VIOSName)

		systemUUID, _, err := l.hmcClient.GetManagedSystemByName(viosConfig.SystemName, l.ctx.Verbose)
		if err != nil {
			return fmt.Errorf("failed to get system UUID: %w", err)
		}

		// Get VIOS UUID
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
			return fmt.Errorf("VIOS '%s' not found on system %s", viosConfig.VIOSName, viosConfig.SystemName)
		}

		// Discover VG for this specific VIOS
		discoveredVG, err := l.discoverVolumeGroupForVIOS(viosUUID, viosConfig.VIOSName)
		if err != nil {
			return err
		}

		viosConfig.VolumeGroup = discoveredVG
		fmt.Printf("    ✓ Auto-configured volume group: %s\n", discoveredVG)
	} else {
		// Both are empty - full auto-discovery
		discoveredVIOSName, discoveredVG, err := l.discoverVIOSAndVolumeGroup(viosConfig.SystemName)
		if err != nil {
			return fmt.Errorf("failed to auto-discover VIOS configuration: %w", err)
		}

		if viosConfig.VIOSName == "" {
			viosConfig.VIOSName = discoveredVIOSName
			fmt.Printf("    ✓ Auto-configured VIOS name: %s\n", discoveredVIOSName)
		}

		if viosConfig.VolumeGroup == "" {
			viosConfig.VolumeGroup = discoveredVG
			fmt.Printf("    ✓ Auto-configured volume group: %s\n", discoveredVG)
		}
	}

	// Verify the configuration was actually updated
	fmt.Printf("    [DEBUG] Final VIOS config after discovery: VIOSName='%s', VolumeGroup='%s'\n",
		viosConfig.VIOSName, viosConfig.VolumeGroup)

	return nil
}

// discoverVolumeGroupForVIOS discovers a suitable volume group for a specific VIOS
// Returns the first non-rootvg volume group with free space
func (l *LPARProvisioner) discoverVolumeGroupForVIOS(viosUUID, viosName string) (string, error) {
	// Get volume groups from the VIOS
	volumeGroups, err := l.hmcClient.GetVolumeGroups(viosUUID, l.ctx.Verbose)
	if err != nil {
		return "", fmt.Errorf("failed to get volume groups from VIOS %s: %w", viosName, err)
	}

	if len(volumeGroups) == 0 {
		return "", fmt.Errorf("no volume groups found on VIOS %s", viosName)
	}

	// Find first non-rootvg volume group with free space
	for _, vg := range volumeGroups {
		if vg.GroupName != "rootvg" && vg.FreeSpace != "0" {
			return vg.GroupName, nil
		}
	}

	// If we get here, only rootvg exists (validation should have caught this)
	return "", fmt.Errorf("no suitable volume group found on VIOS %s (only rootvg available)", viosName)
}
