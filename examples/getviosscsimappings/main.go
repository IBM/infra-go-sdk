package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

type viosMappingResult struct {
	ViosName     string
	ViosUUID     string
	ViosState    string
	MappingCount int
	Mappings     []hmc.VirtualSCSIMapping
	Error        error
}

// formatBool converts a boolean value to a user-friendly "Yes"/"No" string
func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func main() {
	// Command line flags
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	hmcUser := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	hmcPass := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	viosFilter := flag.String("vios-filter", "", "Filter by specific VIOS name (optional)")

	flag.Parse()

	if *hmcPass == "" {
		log.Fatal("Error: hmc-pass is required")
	}

	// =========================================================================
	// STEP 1: Login to HMC
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*hmcUser, *hmcPass, *verbose); err != nil {
		log.Fatalf("HMC Login failed: %v", err)
	}
	defer restClient.Logoff()

	fmt.Printf("✓ Successfully logged into HMC at %s\n", *hmcIP)

	// =========================================================================
	// STEP 2: Get Managed System UUID
	// =========================================================================
	systemUUID, system, err := restClient.GetManagedSystemByName(*sysName, *verbose)
	if err != nil {
		log.Fatalf("Failed to get managed system: %v", err)
	}

	fmt.Printf("✓ Found Managed System: %s (UUID: %s)\n", system.SystemName, systemUUID)

	// =========================================================================
	// STEP 3: Get All VIOS Instances
	// =========================================================================
	viosList, err := restClient.GetVirtualIOServers(systemUUID, *verbose)
	if err != nil {
		log.Fatalf("Failed to get VIOS list: %v", err)
	}

	if len(viosList) == 0 {
		log.Fatalf("No VIOS found on system '%s'", *sysName)
	}

	fmt.Printf("✓ Found %d VIOS instance(s) on system\n", len(viosList))

	// Apply VIOS filter if specified
	// Apply VIOS filter if specified, otherwise process all VIOS
	if *viosFilter != "" {
		var filteredViosList []hmc.VirtualIOServerDetailed
		for _, v := range viosList {
			if v.PartitionName == *viosFilter {
				filteredViosList = append(filteredViosList, v)
			}
		}
		if len(filteredViosList) == 0 {
			log.Fatalf("VIOS '%s' not found on system '%s'", *viosFilter, *sysName)
		}
		viosList = filteredViosList
		fmt.Printf("✓ Filtered to VIOS: %s\n", *viosFilter)
	} else {
		fmt.Printf("✓ Processing all %d VIOS instance(s)\n", len(viosList))
	}

	// =========================================================================
	// STEP 4: Get SCSI Mappings for Each VIOS
	// =========================================================================
	var allResults []viosMappingResult

	for _, vios := range viosList {
		fmt.Printf("\n📋 Fetching SCSI mappings for VIOS '%s' (State: %s)...\n",
			vios.PartitionName, vios.PartitionState)

		mappings, err := restClient.GetViosSCSIMappings(vios.PartitionUUID, *verbose)
		
		result := viosMappingResult{
			ViosName:  vios.PartitionName,
			ViosUUID:  vios.PartitionUUID,
			ViosState: vios.PartitionState,
			Mappings:  mappings,
			Error:     err,
		}

		if err != nil {
			fmt.Printf("   ⚠️  Error fetching mappings: %v\n", err)
			result.MappingCount = 0
		} else {
			result.MappingCount = len(mappings)
			fmt.Printf("   ✓ Found %d SCSI mapping(s)\n", len(mappings))
		}

		allResults = append(allResults, result)
	}

	// =========================================================================
	// STEP 5: Display Results
	// =========================================================================
	if *jsonOutput {
		// Output as JSON
		jsonData, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}
		fmt.Println(string(jsonData))
	} else {
		// Human-readable output
		displayAllResults(allResults)
	}
}

func displayAllResults(results []viosMappingResult) {
	totalMappings := 0
	totalPhysical := 0
	totalVirtualDisk := 0
	totalOptical := 0

	for _, result := range results {
		if result.Error != nil {
			continue
		}

		fmt.Printf("\n\n")
		fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
		fmt.Printf("VIOS: %s (UUID: %s, State: %s)\n", result.ViosName, result.ViosUUID, result.ViosState)
		fmt.Printf("═══════════════════════════════════════════════════════════════════\n")

		if len(result.Mappings) == 0 {
			fmt.Printf("\nNo SCSI mappings found for this VIOS.\n")
			continue
		}

		physicalCount := 0
		virtualDiskCount := 0
		opticalCount := 0

		for i, mapping := range result.Mappings {
			// Extract LPAR UUID from URI for display
			lparUUID := extractUUIDFromURI(mapping.AssociatedLogicalPartition.Href)

			fmt.Printf("\n───────────────────────────────────────────────────────────────────\n")
			fmt.Printf("Mapping #%d\n", i+1)
			fmt.Printf("───────────────────────────────────────────────────────────────────\n")

			// LPAR Information
			fmt.Printf("\n🖥️  LPAR Information:\n")
			fmt.Printf("   LPAR UUID:      %s\n", lparUUID)
			fmt.Printf("   URI:            %s\n", mapping.AssociatedLogicalPartition.Href)

			// Client Adapter (LPAR side)
			fmt.Printf("\n📡 Client Adapter (LPAR side):\n")
			fmt.Printf("   Adapter Type:   %s\n", mapping.ClientAdapter.AdapterType)
			fmt.Printf("   Slot Number:    %d\n", mapping.ClientAdapter.VirtualSlotNumber)
			fmt.Printf("   Location Code:  %s\n", mapping.ClientAdapter.LocationCode)
			fmt.Printf("   DRC Name:       %s\n", mapping.ClientAdapter.DynamicReconfigurationConnectorName)
			fmt.Printf("   Remote Slot:    %d\n", mapping.ClientAdapter.RemoteSlotNumber)

			// Server Adapter (VIOS side)
			fmt.Printf("\n🔌 Server Adapter (VIOS side):\n")
			fmt.Printf("   Adapter Name:   %s\n", mapping.ServerAdapter.AdapterName)
			fmt.Printf("   Adapter Type:   %s\n", mapping.ServerAdapter.AdapterType)
			fmt.Printf("   Slot Number:    %d\n", mapping.ServerAdapter.VirtualSlotNumber)
			fmt.Printf("   Location Code:  %s\n", mapping.ServerAdapter.LocationCode)
			fmt.Printf("   Backing Device: %s\n", mapping.ServerAdapter.BackingDeviceName)
			fmt.Printf("   DRC Name:       %s\n", mapping.ServerAdapter.DynamicReconfigurationConnectorName)

			// Storage Information - Determine storage type
			fmt.Printf("\n💾 Storage Information:\n")
			
			// Determine storage type based on which field is populated
			storageType := ""
			if mapping.Storage.PhysicalVolume.VolumeName != "" {
				storageType = "PhysicalVolume"
			} else if mapping.Storage.VirtualOpticalMedia.MediaName != "" {
				storageType = "VirtualOpticalMedia"
			} else if mapping.Storage.VirtualDisk.DiskName != "" {
				storageType = "VirtualDisk"
			}
			fmt.Printf("   Storage Type:   %s\n", storageType)

			switch storageType {
			case "PhysicalVolume":
				physicalCount++
				pv := mapping.Storage.PhysicalVolume
				fmt.Printf("   Volume Name:    %s\n", pv.VolumeName)
				fmt.Printf("   Capacity:       %v\n", pv.VolumeCapacity)
				fmt.Printf("   Volume State:   %s\n", pv.VolumeState)
				fmt.Printf("   Volume UDID:    %s\n", pv.VolumeUniqueID)
				fmt.Printf("   Description:    %s\n", pv.Description)
				fmt.Printf("   Location Code:  %s\n", pv.LocationCode)
				// Display backing storage types
				fmt.Printf("   FC Backed:      %s\n", formatBool(pv.IsFibreChannelBacked))
				fmt.Printf("   iSCSI Backed:   %s\n", formatBool(pv.IsISCSIBacked))
				fmt.Printf("   Reserve Policy: %s\n", pv.ReservePolicy)
				if pv.StorageLabel != "" {
					fmt.Printf("   Storage Label:  %s\n", pv.StorageLabel)
				}

			case "VirtualOpticalMedia":
				opticalCount++
				opt := mapping.Storage.VirtualOpticalMedia
				fmt.Printf("   Media Name:     %s\n", opt.MediaName)
				fmt.Printf("   Media UDID:     %s\n", opt.MediaUDID)
				fmt.Printf("   Mount Type:     %s\n", opt.MountType)
				fmt.Printf("   Size:           %s\n", opt.Size)

			case "VirtualDisk":
				virtualDiskCount++
				vd := mapping.Storage.VirtualDisk
				if vd.DiskName != "" {
					fmt.Printf("   Disk Name:      %s\n", vd.DiskName)
				}
				if vd.DiskCapacity != "" {
					fmt.Printf("   Capacity:       %s\n", vd.DiskCapacity)
				}
				if vd.UniqueDeviceID != "" {
					fmt.Printf("   Volume UDID:    %s\n", vd.UniqueDeviceID)
				}
			}

			// Target Device - Determine device type
			fmt.Printf("\n🎯 Target Device:\n")
			deviceType := ""
			targetName := ""
			lunAddress := ""
			uniqueID := ""
			
			if mapping.TargetDevice.PhysicalVolumeVirtualTargetDevice.TargetName != "" {
				deviceType = "PhysicalVolumeVirtualTargetDevice"
				targetName = mapping.TargetDevice.PhysicalVolumeVirtualTargetDevice.TargetName
				lunAddress = mapping.TargetDevice.PhysicalVolumeVirtualTargetDevice.LogicalUnitAddress
				uniqueID = mapping.TargetDevice.PhysicalVolumeVirtualTargetDevice.UniqueDeviceID
			} else if mapping.TargetDevice.LogicalVolumeVirtualTargetDevice.TargetName != "" {
				deviceType = "LogicalVolumeVirtualTargetDevice"
				targetName = mapping.TargetDevice.LogicalVolumeVirtualTargetDevice.TargetName
				lunAddress = mapping.TargetDevice.LogicalVolumeVirtualTargetDevice.LogicalUnitAddress
				uniqueID = mapping.TargetDevice.LogicalVolumeVirtualTargetDevice.UniqueDeviceID
			} else if mapping.TargetDevice.VirtualOpticalTargetDevice.TargetName != "" {
				deviceType = "VirtualOpticalTargetDevice"
				targetName = mapping.TargetDevice.VirtualOpticalTargetDevice.TargetName
				lunAddress = mapping.TargetDevice.VirtualOpticalTargetDevice.LogicalUnitAddress
				uniqueID = mapping.TargetDevice.VirtualOpticalTargetDevice.UniqueDeviceID
			}
			
			fmt.Printf("   Device Type:    %s\n", deviceType)
			fmt.Printf("   Target Name:    %s\n", targetName)
			fmt.Printf("   LUN Address:    %s\n", lunAddress)
			if uniqueID != "" {
				fmt.Printf("   Unique ID:      %s\n", uniqueID)
			}
		}

		// VIOS Summary
		fmt.Printf("\n───────────────────────────────────────────────────────────────────\n")
		fmt.Printf("📊 VIOS '%s' Summary:\n", result.ViosName)
		fmt.Printf("───────────────────────────────────────────────────────────────────\n")
		fmt.Printf("Total Mappings:        %d\n", len(result.Mappings))
		fmt.Printf("Physical Volumes:      %d\n", physicalCount)
		fmt.Printf("Virtual Disks:         %d\n", virtualDiskCount)
		fmt.Printf("Virtual Optical Media: %d\n", opticalCount)

		totalMappings += len(result.Mappings)
		totalPhysical += physicalCount
		totalVirtualDisk += virtualDiskCount
		totalOptical += opticalCount
	}

	// Overall Summary (if multiple VIOS)
	if len(results) > 1 {
		fmt.Printf("\n\n")
		fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
		fmt.Printf("📊 Overall Summary (All VIOS)\n")
		fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
		fmt.Printf("Total VIOS Instances:  %d\n", len(results))
		fmt.Printf("Total Mappings:        %d\n", totalMappings)
		fmt.Printf("Physical Volumes:      %d\n", totalPhysical)
		fmt.Printf("Virtual Disks:         %d\n", totalVirtualDisk)
		fmt.Printf("Virtual Optical Media: %d\n", totalOptical)
		fmt.Println()
	}
}

// extractUUIDFromURI extracts the UUID from an HMC REST API URI
func extractUUIDFromURI(uri string) string {
	// URI format: https://hmc-ip/rest/api/uom/LogicalPartition/uuid
	// We want to extract the last part after the final '/'
	if uri == "" {
		return ""
	}

	// Find the last '/' and extract everything after it
	for i := len(uri) - 1; i >= 0; i-- {
		if uri[i] == '/' {
			return uri[i+1:]
		}
	}

	return ""
}

// Made with Bob
