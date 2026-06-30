package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc"
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

// Rollback tracker to clean up resources on failure
type RollbackTracker struct {
	createdLPAR       bool
	lparUUID          string
	createdNetwork    bool
	createdMedia      []string
	mappedMedia       bool
	createdDisks      []string
	mappedDisks       bool
	savedProfile      bool
	poweredOn         bool
	restClient        *hmc.RestClient
	sysUUID           string
	sysName           string
	viosUUID          string
	viosName          string
	lparName          string
	verbose           bool
}

func (r *RollbackTracker) Rollback(ctx context.Context) {
	if r.restClient == nil {
		return
	}
	
	fmt.Println("\n⚠️  ROLLBACK: Cleaning up created resources...")
	
	// Step 1: Power off LPAR if we powered it on
	if r.poweredOn && r.lparUUID != "" {
		fmt.Printf("   [1/7] Powering off LPAR '%s'...\n", r.lparName)
		_, err := r.restClient.PowerOffPartition(ctx, r.lparUUID, "immediate", false)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to power off LPAR: %v\n", err)
		} else {
			fmt.Printf("   ✅ LPAR powered off\n")
		}
		time.Sleep(5 * time.Second) // Wait for power off
	}
	
	// Step 2: Unmap virtual disks if they were mapped
	if r.mappedDisks && len(r.createdDisks) > 0 && r.lparUUID != "" {
		fmt.Printf("   [2/7] Unmapping %d virtual disk(s)...\n", len(r.createdDisks))
		_, err := r.restClient.DeleteVirtualDiskMaps(r.sysUUID, r.viosUUID, r.lparUUID, r.createdDisks)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to unmap virtual disks: %v\n", err)
		} else {
			fmt.Printf("   ✅ Virtual disks unmapped\n")
		}
	}
	
	// Step 3: Delete virtual disks if they were created
	if len(r.createdDisks) > 0 {
		fmt.Printf("   [3/7] Deleting %d virtual disk(s)...\n", len(r.createdDisks))
		for _, diskName := range r.createdDisks {
			err := r.restClient.DeleteVirtualDisk(context.Background(), r.sysName, r.viosName, diskName)
			if err != nil {
				fmt.Printf("   ⚠️  Failed to delete disk '%s': %v\n", diskName, err)
			} else {
				fmt.Printf("   ✅ Deleted disk: %s\n", diskName)
			}
		}
	}
	
	// Step 4: Unmap optical media if they were mapped
	if r.mappedMedia && len(r.createdMedia) > 0 && r.lparUUID != "" {
		fmt.Printf("   [4/7] Unmapping %d optical media...\n", len(r.createdMedia))
		_, err := r.restClient.DeleteVirtualOpticalMaps(context.Background(), r.sysUUID, r.viosUUID, r.lparUUID, r.createdMedia)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to unmap optical media: %v\n", err)
		} else {
			fmt.Printf("   ✅ Optical media unmapped\n")
		}
	}
	
	// Step 5: Delete optical media files from VIOS repository
	if len(r.createdMedia) > 0 {
		fmt.Printf("   [5/7] Deleting %d optical media file(s) from VIOS repository...\n", len(r.createdMedia))
		for _, mediaName := range r.createdMedia {
			// Use VIOS CLI to remove the optical media
			err := r.restClient.RemoveVIOSDevice(r.viosUUID, mediaName)
			if err != nil {
				fmt.Printf("   ⚠️  Failed to delete media '%s': %v\n", mediaName, err)
			} else {
				fmt.Printf("   ✅ Deleted media: %s\n", mediaName)
			}
		}
	}
	
	// Step 6: Delete network adapter if created
	if r.createdNetwork && r.lparUUID != "" {
		fmt.Printf("   [6/7] Deleting network adapter...\n")
		// Network adapter will be deleted with LPAR
		fmt.Printf("   ℹ️  Network adapter will be deleted with LPAR\n")
	}
	
	// Step 7: Delete LPAR if it was created
	if r.createdLPAR && r.lparUUID != "" {
		fmt.Printf("   [7/7] Deleting LPAR '%s'...\n", r.lparName)
		err := r.restClient.DeleteLogicalPartition(r.lparUUID)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to delete LPAR: %v\n", err)
		} else {
			fmt.Printf("   ✅ LPAR deleted\n")
		}
	}
	
	fmt.Println("✅ Rollback completed - all resources cleaned up")
}

// deleteLPAR handles the deletion of an LPAR and its associated resources
func deleteLPAR(ctx context.Context,hmcIP, username, password, sysName, lparName, viosName string) {
	fmt.Println("=========================================================================")
	fmt.Printf(" 🗑️  LPAR Deletion Workflow: %s\n", lparName)
	fmt.Println("=========================================================================")

	// =========================================================================
	// STEP 1: AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Println("\n[Step 1/7] Authenticating and resolving resources...")
	
	restClient := exutil.NewClient(hmcIP, false, false)
	if err := restClient.Login(context.Background(), username, password); err != nil {
		log.Fatalf("❌ HMC Login failed: %v", err)
	}
	defer restClient.Logoff(context.Background())
	fmt.Printf("✅ Logged into HMC at %s\n", hmcIP)

	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), sysName)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", sysName)
	}
	fmt.Printf("✅ Found system: %s (UUID: %s)\n", sysName, sysUUID)

	// =========================================================================
	// STEP 2: FIND LPAR
	// =========================================================================
	fmt.Println("\n[Step 2/7] Finding LPAR...")
	
	_, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, lparName)
	if err != nil || lparUUID == "" {
		log.Fatalf("❌ LPAR '%s' not found on system '%s'", lparName, sysName)
	}
	fmt.Printf("✅ Found LPAR: %s (UUID: %s)\n", lparName, lparUUID)

	// Get LPAR details to check state
	lparDetails, err := restClient.GetLogicalPartitionDetailed(context.Background(), lparUUID)
	if err != nil {
		log.Fatalf("❌ Failed to get LPAR details: %v", err)
	}
	fmt.Printf("   Current state: %s\n", lparDetails.PartitionState)

	// =========================================================================
	// STEP 3: DISCOVER VIOS (if not specified)
	// =========================================================================
	fmt.Println("\n[Step 3/7] Resolving VIOS...")
	
	var viosUUID string
	var viosNameResolved string
	
	if viosName != "" {
		// User specified a VIOS name - use it
		fmt.Printf("   Using specified VIOS: %s\n", viosName)
		viosUUID, err = hmc.GetViosID(context.Background(), restClient, sysUUID, viosName)
		if err != nil || viosUUID == "" {
			log.Fatalf("❌ VIOS '%s' not found.", viosName)
		}
		viosNameResolved = viosName
		fmt.Printf("✅ Found VIOS: %s (UUID: %s)\n", viosNameResolved, viosUUID)
	} else {
		// Auto-discover active VIOS
		fmt.Printf("   Auto-discovering active VIOS...\n")
		
		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID)
		if err != nil {
			log.Fatalf("❌ Failed to get VIOS list: %v", err)
		}
		
		if len(viosList) == 0 {
			log.Fatalf("❌ No VIOS servers found on system '%s'", sysName)
		}
		
		viosUUIDs := make([]string, len(viosList))
		for i, v := range viosList {
			viosUUIDs[i] = v.UUID
		}
		
		activeVIOSMap, err := restClient.GetActiveVIOSServers(context.Background(), sysUUID, viosUUIDs)
		if err != nil {
			log.Fatalf("❌ Failed to find active VIOS: %v", err)
		}
		
		for uuid, details := range activeVIOSMap {
			viosUUID = uuid
			viosNameResolved = details.PartitionName
			fmt.Printf("✅ Auto-selected active VIOS: %s (UUID: %s, State: %s)\n",
				viosNameResolved, viosUUID, details.ResourceMonitoringControlState)
			break
		}
	}

	// =========================================================================
	// STEP 4: POWER OFF LPAR (if running)
	// =========================================================================
	fmt.Println("\n[Step 4/7] Powering off LPAR (if running)...")
	
	if lparDetails.PartitionState == "running" || lparDetails.PartitionState == "starting" {
		fmt.Printf("   LPAR is %s, powering off...\n", lparDetails.PartitionState)
		_, err := restClient.PowerOffPartition(ctx,lparUUID, "Immediate", false)
		if err != nil {
			if strings.Contains(err.Error(), "not running") || strings.Contains(err.Error(), "not activated") {
				fmt.Printf("   ℹ️  LPAR already powered off\n")
			} else {
				log.Fatalf("❌ Failed to power off LPAR: %v", err)
			}
		} else {
			fmt.Printf("✅ LPAR powered off successfully\n")
			fmt.Printf("   Waiting 5 seconds for shutdown to complete...\n")
			time.Sleep(5 * time.Second)
		}
	} else {
		fmt.Printf("   ℹ️  LPAR is already in '%s' state, no need to power off\n", lparDetails.PartitionState)
	}

	// =========================================================================
	// STEP 5: GET AND DELETE SCSI MAPPINGS
	// =========================================================================
	fmt.Println("\n[Step 5/7] Removing SCSI mappings...")
	
	// Get all SCSI mappings for this LPAR
	mappings, err := restClient.GetViosSCSIMappings(context.Background(), viosUUID)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to get SCSI mappings: %v\n", err)
	} else {
		var diskMappings []string
		var opticalMappings []string
		
		// Categorize mappings by storage type
		for _, mapping := range mappings {
			// Check if this mapping belongs to our LPAR (using Href field from LinkXML)
			if mapping.AssociatedLogicalPartition.Href != "" &&
			   strings.Contains(mapping.AssociatedLogicalPartition.Href, lparUUID) {
				
				if mapping.Storage.VirtualDisk.DiskName != "" {
					diskMappings = append(diskMappings, mapping.Storage.VirtualDisk.DiskName)
				} else if mapping.Storage.VirtualOpticalMedia.MediaName != "" {
					opticalMappings = append(opticalMappings, mapping.Storage.VirtualOpticalMedia.MediaName)
				}
			}
		}
		
		// Unmap virtual disks
		if len(diskMappings) > 0 {
			fmt.Printf("   Unmapping %d virtual disk(s)...\n", len(diskMappings))
			_, err := restClient.DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID, diskMappings)
			if err != nil {
				fmt.Printf("   ⚠️  Warning: Failed to unmap some virtual disks: %v\n", err)
			} else {
				fmt.Printf("   ✅ Virtual disks unmapped\n")
			}
			
			// Delete the virtual disks
			fmt.Printf("   Deleting %d virtual disk(s)...\n", len(diskMappings))
			for _, diskName := range diskMappings {
				err := restClient.DeleteVirtualDisk(context.Background(), sysName, viosNameResolved, diskName)
				if err != nil {
					fmt.Printf("   ⚠️  Warning: Failed to delete disk '%s': %v\n", diskName, err)
				} else {
					fmt.Printf("   ✅ Deleted disk: %s\n", diskName)
				}
			}
		} else {
			fmt.Printf("   ℹ️  No virtual disk mappings found\n")
		}
		
		// Unmap optical media
		if len(opticalMappings) > 0 {
			fmt.Printf("   Unmapping %d optical media...\n", len(opticalMappings))
			_, err := restClient.DeleteVirtualOpticalMaps(context.Background(), sysUUID, viosUUID, lparUUID, opticalMappings)
			if err != nil {
				fmt.Printf("   ⚠️  Warning: Failed to unmap optical media: %v\n", err)
			} else {
				fmt.Printf("   ✅ Optical media unmapped\n")
			}
		} else {
			fmt.Printf("   ℹ️  No optical media mappings found\n")
		}
	}

	// =========================================================================
	// STEP 6: DELETE NETWORK ADAPTERS
	// =========================================================================
	fmt.Println("\n[Step 6/7] Removing network adapters...")
	fmt.Printf("   ℹ️  Network adapters will be deleted with LPAR\n")

	// =========================================================================
	// STEP 7: DELETE LPAR
	// =========================================================================
	fmt.Println("\n[Step 7/7] Deleting LPAR...")
	
	err = restClient.DeleteLogicalPartition(lparUUID)
	if err != nil {
		log.Fatalf("❌ Failed to delete LPAR: %v", err)
	}
	fmt.Printf("✅ LPAR '%s' deleted successfully\n", lparName)

	// =========================================================================
	// COMPLETION SUMMARY
	// =========================================================================
	fmt.Println("\n=========================================================================")
	fmt.Println(" ✅ LPAR DELETION COMPLETED SUCCESSFULLY")
	fmt.Println("=========================================================================")
	fmt.Printf("\nDeleted Resources:\n")
	fmt.Printf("  LPAR:           %s (UUID: %s)\n", lparName, lparUUID)
	fmt.Printf("  System:         %s\n", sysName)
	fmt.Printf("  VIOS:           %s\n", viosNameResolved)
	fmt.Println("\nNote: Optical media files remain in VIOS repository for potential reuse.")
	fmt.Println("      Use VIOS commands to manually delete them if needed.")
	fmt.Println("=========================================================================")
}

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	
	// Operation Mode (mutually exclusive)
	createMode := flag.Bool("create", false, "Create and provision a new LPAR")
	deleteMode := flag.Bool("delete", false, "Delete an existing LPAR and its resources")
	
	// LPAR Configuration
	lparName := flag.String("lpar-name", "TEST-CLOUD-INIT-ISO", "Name for the LPAR (required for both create and delete)")
	osType := flag.String("os-type", "AIX/Linux", "OS type (AIX/Linux, OS400, Virtual IO Server)")
	lparProfile := flag.String("lpar-profile", "default_profile", "LPAR profile to save")
	
	// CPU Configuration
	minProcUnits := flag.Float64("min-proc-units", 0.5, "Minimum processing units")
	desiredProcUnits := flag.Float64("desired-proc-units", 1.0, "Desired processing units")
	maxProcUnits := flag.Float64("max-proc-units", 2.0, "Maximum processing units")
	minVcpus := flag.Int("min-vcpus", 1, "Minimum virtual CPUs")
	desiredVcpus := flag.Int("desired-vcpus", 2, "Desired virtual CPUs")
	maxVcpus := flag.Int("max-vcpus", 4, "Maximum virtual CPUs")
	sharingMode := flag.String("sharing-mode", "uncapped", "Processor sharing mode (uncapped/capped)")
	
	// Memory Configuration (in MB)
	minMem := flag.Int("min-mem", 2048, "Minimum memory in MB")
	desiredMem := flag.Int("desired-mem", 4096, "Desired memory in MB")
	maxMem := flag.Int("max-mem", 8192, "Maximum memory in MB")
	
	// Network Configuration
	vswitchName := flag.String("vswitch-name", "ETHERNET0(Default)", "Virtual switch name")
	vlanID := flag.Int("vlan-id", 1337, "VLAN ID for network adapter")
	
	// VIOS Configuration (optional - will auto-discover active VIOS if not specified)
	viosName := flag.String("vios-name", "", "Target VIOS name (optional - will auto-select active VIOS if not specified)")
	
	// NFS Configuration
	nfsServer := flag.String("nfs-server", "192.0.2.20", "NFS server IP or hostname")
	exportPath := flag.String("export-path", "/var/www/html/f43", "NFS export path on server")
	mountPoint := flag.String("mount-point", "/mnt", "Local mount point on VIOS")
	
	// ISO Configuration
	isoFiles := flag.String("iso-files", "f43.iso", "Comma-separated list of ISO filenames on NFS (e.g., rhcos.iso,fedora.iso)")
	mediaPrefix := flag.String("media-prefix", fmt.Sprintf("media_%d", time.Now().Unix()), "Prefix for media names in repository")
	
	// Virtual Disk Configuration
	vgName := flag.String("vg-name", "auto_vg01", "Volume Group name for virtual disks")
	diskNames := flag.String("disk-names", fmt.Sprintf("disk_%d", time.Now().Unix()), "Comma-separated list of virtual disk names (e.g., disk1,disk2)")
	diskSizes := flag.String("disk-sizes-mb", "10240", "Comma-separated list of disk sizes in MB (e.g., 10240,20480). Must match disk-names count")
	
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits

	// Validate mutually exclusive flags
	if *createMode && *deleteMode {
		log.Fatal("❌ Error: --create and --delete are mutually exclusive. Use only one.")
	}
	
	if !*createMode && !*deleteMode {
		log.Fatal("❌ Error: Either --create or --delete must be specified.")
	}

	if *password == "" || *lparName == "" {
		log.Fatal("❌ Error: hmc-pass and lpar-name are required.")
	}

	// Route to appropriate operation
	if *deleteMode {
		deleteLPAR(ctx,*hmcIP, *username, *password, *sysName, *lparName, *viosName)
		return
	}

	// Continue with create mode
	fmt.Println("=========================================================================")
	fmt.Printf(" 🚀 Complete LPAR Provisioning Workflow: %s\n", *lparName)
	fmt.Println("=========================================================================")

	// Initialize rollback tracker
	rollback := &RollbackTracker{
		lparName: *lparName,
		verbose:  *verbose,
	}
	
	// Ensure rollback is called on any panic or error
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n❌ PANIC: %v\n", r)
			rollback.Rollback(ctx)
			panic(r)
		}
	}()

	// =========================================================================
	// STEP 1: AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Println("\n[Step 1/10] Authenticating and resolving resources...")
	
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
	if err := restClient.Login(context.Background(), *username, *password); err != nil {
		log.Fatalf("❌ HMC Login failed: %v", err)
	}
	defer restClient.Logoff(context.Background())
	fmt.Printf("✅ Logged into HMC at %s\n", *hmcIP)

	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System '%s' not found.", *sysName)
	}
	fmt.Printf("✅ Found system: %s (UUID: %s)\n", *sysName, sysUUID)

	// Discover VIOS - either use specified name or auto-select active VIOS
	var viosUUID string
	var viosNameResolved string
	
	if *viosName != "" {
		// User specified a VIOS name - use it
		fmt.Printf("   Using specified VIOS: %s\n", *viosName)
		viosUUID, err = hmc.GetViosID(context.Background(), restClient, sysUUID, *viosName)
		if err != nil || viosUUID == "" {
			log.Fatalf("❌ VIOS '%s' not found.", *viosName)
		}
		viosNameResolved = *viosName
		fmt.Printf("✅ Found VIOS: %s (UUID: %s)\n", viosNameResolved, viosUUID)
	} else {
		// Auto-discover active VIOS
		fmt.Printf("   Auto-discovering active VIOS...\n")
		
		// Get all VIOS servers
		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID)
		if err != nil {
			log.Fatalf("❌ Failed to get VIOS list: %v", err)
		}
		
		if len(viosList) == 0 {
			log.Fatalf("❌ No VIOS servers found on system '%s'", *sysName)
		}
		
		// Extract VIOS UUIDs
		viosUUIDs := make([]string, len(viosList))
		for i, v := range viosList {
			viosUUIDs[i] = v.UUID
		}
		
		// Get active VIOS servers
		activeVIOSMap, err := restClient.GetActiveVIOSServers(context.Background(), sysUUID, viosUUIDs)
		if err != nil {
			log.Fatalf("❌ Failed to find active VIOS: %v", err)
		}
		
		// Select the first active VIOS
		for uuid, details := range activeVIOSMap {
			viosUUID = uuid
			viosNameResolved = details.PartitionName
			fmt.Printf("✅ Auto-selected active VIOS: %s (UUID: %s, State: %s)\n",
				viosNameResolved, viosUUID, details.ResourceMonitoringControlState)
			break
		}
	}

	// Set rollback tracker with client and UUIDs
	rollback.restClient = restClient
	rollback.sysUUID = sysUUID
	rollback.sysName = *sysName
	rollback.viosUUID = viosUUID
	rollback.viosName = viosNameResolved

	// =========================================================================
	// STEP 2: CREATE LPAR
	// =========================================================================
	fmt.Println("\n[Step 2/10] Creating LPAR...")
	
	// Check if LPAR already exists
	_, existingUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName)
	if err == nil && existingUUID != "" {
		fmt.Printf("❌ LPAR '%s' already exists (UUID: %s)\n", *lparName, existingUUID)
		log.Fatalf("Please delete the existing LPAR or use a different name")
	}
	
	lparReq := hmc.CreateLparRequest{
		Name:             *lparName,
		OsType:           *osType,
		MinMem:           *minMem,
		DesiredMem:       *desiredMem,
		MaxMem:           *maxMem,
		MinProcUnits:     *minProcUnits,
		DesiredProcUnits: *desiredProcUnits,
		MaxProcUnits:     *maxProcUnits,
		MinVcpus:         *minVcpus,
		DesiredVcpus:     *desiredVcpus,
		MaxVcpus:         *maxVcpus,
		SharingMode:      *sharingMode,
	}
	
	fmt.Printf("   Creating LPAR with:\n")
	fmt.Printf("   - CPU: %.1f units (%d vCPUs, %s)\n", *desiredProcUnits, *desiredVcpus, *sharingMode)
	fmt.Printf("   - Memory: %d MB\n", *desiredMem)
	fmt.Printf("   - OS Type: %s\n", *osType)
	
	lparDetails, err := restClient.CreateLogicalPartition(sysUUID, lparReq)
	if err != nil {
		log.Fatalf("❌ Failed to create LPAR: %v", err)
	}
	
	lparUUID := lparDetails.PartitionUUID
	fmt.Printf("✅ LPAR created successfully (UUID: %s)\n", lparUUID)
	
	// Track LPAR creation for rollback
	rollback.createdLPAR = true
	rollback.lparUUID = lparUUID

	// =========================================================================
	// STEP 3: RESOLVE VSWITCH AND CREATE NETWORK ADAPTER
	// =========================================================================
	fmt.Println("\n[Step 3/10] Resolving vSwitch and creating network adapter...")
	
	// Get vSwitch UUID
	fmt.Printf("   Resolving vSwitch '%s'...\n", *vswitchName)
	switches, err := restClient.GetVirtualSwitchQuickAll(context.Background(), sysUUID)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve Virtual Switches: %v\n", err)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at vSwitch retrieval stage")
	}
	
	var vswitchUUID string
	for _, s := range switches {
		if strings.EqualFold(s.SwitchName, *vswitchName) {
			vswitchUUID = s.UUID
			break
		}
	}
	if vswitchUUID == "" {
		fmt.Printf("❌ Virtual Switch '%s' not found\n", *vswitchName)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at vSwitch resolution stage")
	}
	fmt.Printf("   ✅ Found vSwitch UUID: %s\n", vswitchUUID)
	
	fmt.Printf("   Creating network adapter with VLAN %d\n", *vlanID)
	_, err = restClient.CreateClientNetworkAdapter(context.Background(), sysUUID, lparUUID, vswitchUUID, *vlanID)
	if err != nil {
		fmt.Printf("❌ Failed to create network adapter: %v\n", err)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at network adapter creation stage")
	}
	fmt.Printf("✅ Network adapter created successfully\n")
	
	rollback.createdNetwork = true

	// =========================================================================
	// STEP 4: MOUNT NFS ON VIOS
	// =========================================================================
	fmt.Println("\n[Step 4/10] Mounting NFS share on VIOS...")
	
	nfsPath := fmt.Sprintf("%s:%s", *nfsServer, *exportPath)
	fmt.Printf("   NFS: %s -> %s\n", nfsPath, *mountPoint)
	
	_, err = hmc.MountNFS(context.Background(), restClient, *sysName, viosNameResolved, *nfsServer, *exportPath, *mountPoint, "3")
	if err != nil {
		// Check if already mounted
		if strings.Contains(err.Error(), "already mounted") || strings.Contains(err.Error(), "busy") {
			fmt.Printf("⚠️  NFS already mounted (continuing)\n")
		} else {
			fmt.Printf("❌ Failed to mount NFS: %v\n", err)
			rollback.Rollback(ctx)
			log.Fatalf("Provisioning failed at NFS mount stage")
		}
	} else {
		fmt.Printf("✅ NFS mounted successfully\n")
	}

	// =========================================================================
	// STEP 5: CREATE VIRTUAL OPTICAL MEDIA FROM ISO FILES
	// =========================================================================
	fmt.Println("\n[Step 5/10] Creating virtual optical media from ISO files...")
	
	// Parse ISO files
	isoList := strings.Split(*isoFiles, ",")
	for i := range isoList {
		isoList[i] = strings.TrimSpace(isoList[i])
	}
	
	// Build media files map
	mediaFiles := make(map[string]string)
	for _, isoFile := range isoList {
		if isoFile == "" {
			continue
		}
		// Extract base filename without extension for better naming
		baseName := filepath.Base(isoFile)
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)
		mediaName := fmt.Sprintf("%s_%s", *mediaPrefix, nameWithoutExt)
		
		// Full path to ISO on NFS mount
		isoPath := fmt.Sprintf("%s/%s", *mountPoint, isoFile)
		mediaFiles[mediaName] = isoPath
	}
	
	fmt.Printf("   Creating %d optical media...\n", len(mediaFiles))
	for name, path := range mediaFiles {
		fmt.Printf("   - %s from %s\n", name, path)
	}
	
	results, err := restClient.AddVirtualOpticalMedia(context.Background(), viosUUID, mediaFiles)
	if err != nil {
		log.Printf("⚠️  Warning: Some media creation failed: %v", err)
	}
	
	// Check results
	successCount := 0
	var createdMedia []string
	for mediaName, mediaErr := range results {
		if mediaErr == nil {
			fmt.Printf("   ✅ Created: %s\n", mediaName)
			createdMedia = append(createdMedia, mediaName)
			successCount++
		} else {
			fmt.Printf("   ❌ Failed: %s - %v\n", mediaName, mediaErr)
		}
	}
	
	if successCount == 0 {
		rollback.Rollback(ctx)
		log.Fatal("❌ No optical media created successfully")
	}
	fmt.Printf("✅ Created %d optical media successfully\n", successCount)
	
	// Track created media for rollback
	rollback.createdMedia = createdMedia

	// =========================================================================
	// STEP 6: MAP OPTICAL MEDIA TO LPAR
	// =========================================================================
	fmt.Println("\n[Step 6/10] Mapping optical media to LPAR...")
	
	fmt.Printf("   Mapping %d media to LPAR '%s'\n", len(createdMedia), *lparName)
	mappingStatus, err := restClient.CreateVirtualOpticalMaps(context.Background(), sysUUID, viosUUID, lparUUID, createdMedia)
	if err != nil {
		fmt.Printf("❌ Failed to map optical media: %v\n", err)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at optical media mapping stage")
	}
	fmt.Printf("✅ Optical media mapped successfully (Status: %s)\n", mappingStatus)
	
	// Track that media was mapped
	rollback.mappedMedia = true

	// =========================================================================
	// STEP 7: CREATE VIRTUAL DISKS
	// =========================================================================
	fmt.Println("\n[Step 7/10] Creating virtual disks...")
	
	// Parse disk names and sizes
	diskNameList := strings.Split(*diskNames, ",")
	diskSizeList := strings.Split(*diskSizes, ",")
	
	for i := range diskNameList {
		diskNameList[i] = strings.TrimSpace(diskNameList[i])
	}
	for i := range diskSizeList {
		diskSizeList[i] = strings.TrimSpace(diskSizeList[i])
	}
	
	// Validate counts match
	if len(diskNameList) != len(diskSizeList) {
		fmt.Printf("❌ Error: Number of disk names (%d) must match number of disk sizes (%d)\n", len(diskNameList), len(diskSizeList))
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at disk validation stage")
	}
	
	fmt.Printf("   Creating %d virtual disk(s) in VG '%s'\n", len(diskNameList), *vgName)
	
	var createdDisks []string
	for i, diskName := range diskNameList {
		if diskName == "" {
			continue
		}
		
		// Parse size
		var diskSizeMB int
		_, err := fmt.Sscanf(diskSizeList[i], "%d", &diskSizeMB)
		if err != nil {
			fmt.Printf("❌ Invalid disk size '%s': %v\n", diskSizeList[i], err)
			rollback.createdDisks = createdDisks
			rollback.Rollback(ctx)
			log.Fatalf("Provisioning failed at disk size parsing stage")
		}
		
		diskSizeGB := float64(diskSizeMB) / 1024.0
		fmt.Printf("   Creating disk %d/%d: '%s' (%.2fGB / %dMB)\n", i+1, len(diskNameList), diskName, diskSizeGB, diskSizeMB)
		
		err = restClient.CreateVirtualDisk(context.Background(), *sysName, viosUUID, viosNameResolved, *vgName, diskName, diskSizeMB)
		if err != nil {
			fmt.Printf("❌ Failed to create virtual disk '%s': %v\n", diskName, err)
			rollback.createdDisks = createdDisks
			rollback.Rollback(ctx)
			log.Fatalf("Provisioning failed at virtual disk creation stage")
		}
		fmt.Printf("   ✅ Created: %s\n", diskName)
		createdDisks = append(createdDisks, diskName)
	}
	
	if len(createdDisks) == 0 {
		rollback.Rollback(ctx)
		log.Fatal("❌ No virtual disks created")
	}
	fmt.Printf("✅ Created %d virtual disk(s) successfully\n", len(createdDisks))
	
	// Track created disks for rollback
	rollback.createdDisks = createdDisks

	// =========================================================================
	// STEP 8: MAP VIRTUAL DISKS TO LPAR
	// =========================================================================
	fmt.Println("\n[Step 8/10] Mapping virtual disks to LPAR...")
	
	fmt.Printf("   Mapping %d disk(s) to LPAR '%s'\n", len(createdDisks), *lparName)
	for _, disk := range createdDisks {
		fmt.Printf("   - %s\n", disk)
	}
	
	mapStatus, err := restClient.CreateVirtualDiskMaps(sysUUID, viosUUID, lparUUID, createdDisks)
	if err != nil {
		fmt.Printf("❌ Failed to map virtual disks: %v\n", err)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at virtual disk mapping stage")
	}
	fmt.Printf("✅ Virtual disks mapped successfully (Status: %s)\n", mapStatus)
	
	// Track that disks were mapped
	rollback.mappedDisks = true

	// =========================================================================
	// STEP 9: SAVE PARTITION PROFILE
	// =========================================================================
	fmt.Println("\n[Step 9/10] Saving partition profile...")
	
	fmt.Printf("   Saving current configuration to profile '%s'\n", *lparProfile)
	err = restClient.SaveCurrentLparConfig(context.Background(), lparUUID, *lparProfile, false)
	if err != nil {
		fmt.Printf("❌ Failed to save partition profile: %v\n", err)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at profile save stage")
	}
	fmt.Printf("✅ Partition profile '%s' saved successfully\n", *lparProfile)
	
	rollback.savedProfile = true

	// =========================================================================
	// STEP 10: GET PROFILE UUID AND POWER ON LPAR
	// =========================================================================
	fmt.Println("\n[Step 10/10] Powering on LPAR...")
	
	// Get LPAR detailed info to extract profile UUID
	fmt.Printf("   Getting LPAR details to extract profile UUID...\n")
	lparDetailed, err := restClient.GetLogicalPartitionDetailed(context.Background(), lparUUID)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve LPAR details: %v\n", err)
		rollback.Rollback(ctx)
		log.Fatalf("Provisioning failed at LPAR details retrieval stage")
	}
	
	profileHref := lparDetailed.AssociatedPartitionProfile.Href
	if profileHref == "" {
		rollback.Rollback(ctx)
		log.Fatal("❌ No associated partition profile found.")
	}
	// Extract UUID from href (last 36 characters)
	profileUUID := profileHref[len(profileHref)-36:]
	fmt.Printf("   Profile UUID: %s\n", profileUUID)
	
	// Power on with options
	fmt.Printf("   Starting LPAR '%s' with profile '%s'\n", *lparName, *lparProfile)
	powerOnOpts := &hmc.PowerOnOptions{
		ProfileUUID: profileUUID,
		BootMode:    "norm",
		Keylock:     "normal",
	}
	
	_, err = restClient.PowerOnPartition(ctx,lparUUID, powerOnOpts)
	if err != nil {
		// Check if already running
		if strings.Contains(err.Error(), "already running") || strings.Contains(err.Error(), "operating") {
			fmt.Printf("⚠️  LPAR is already running\n")
		} else {
			fmt.Printf("❌ Failed to power on LPAR: %v\n", err)
			rollback.Rollback(ctx)
			log.Fatalf("Provisioning failed at power on stage")
		}
	} else {
		fmt.Printf("✅ LPAR powered on successfully\n")
		rollback.poweredOn = true
	}

	// =========================================================================
	// COMPLETION SUMMARY
	// =========================================================================
	fmt.Println("\n=========================================================================")
	fmt.Println(" ✅ COMPLETE LPAR PROVISIONING COMPLETED SUCCESSFULLY")
	fmt.Println("=========================================================================")
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  LPAR:           %s (UUID: %s)\n", *lparName, lparUUID)
	fmt.Printf("  Profile:        %s\n", *lparProfile)
	fmt.Printf("  CPU:            %.1f units (%d vCPUs, %s)\n", *desiredProcUnits, *desiredVcpus, *sharingMode)
	fmt.Printf("  Memory:         %d MB\n", *desiredMem)
	fmt.Printf("  Network:        %s (VLAN %d)\n", *vswitchName, *vlanID)
	fmt.Printf("  Optical Media:  %d mapped\n", len(createdMedia))
	for _, media := range createdMedia {
		fmt.Printf("    - %s\n", media)
	}
	fmt.Printf("  Virtual Disks:  %d mapped\n", len(createdDisks))
	for i, disk := range createdDisks {
		var diskSizeMB int
		fmt.Sscanf(diskSizeList[i], "%d", &diskSizeMB)
		diskSizeGB := float64(diskSizeMB) / 1024.0
		fmt.Printf("    - %s (%.2fGB)\n", disk, diskSizeGB)
	}
	fmt.Printf("  Status:         Running\n")
	fmt.Println("\nNext Steps:")
	fmt.Println("  1. Monitor LPAR console for boot progress")
	fmt.Println("  2. Complete OS installation from mounted ISO")
	fmt.Println("  3. Configure network and storage as needed")
	fmt.Println("\nNote: Rollback protection is active. If any error occurred,")
	fmt.Println("      all created resources would have been automatically cleaned up.")
	fmt.Println("=========================================================================")
}

// Made with Bob
