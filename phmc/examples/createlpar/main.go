package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc"
	svc "github.com/IBM/infra-go-sdk/svc"
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	// HMC Config
	hmcIP := flag.String("hmc-ip", "", "HMC IP address (Required)")
	username := flag.String("hmc-user", "", "HMC Username (Required)")
	password := flag.String("hmc-pass", "", "HMC Password (Required)")
	sysName := flag.String("system-name", "", "Managed System Name (Required)")
	lparName := flag.String("lpar-name", "", "Name for the new LPAR (Required)")
	osType := flag.String("os-type", "AIX/Linux", "OS type (AIX/Linux, OS400, Virtual IO Server)")

	// Networking Config
	vswitchName := flag.String("vswitch-name", "ETHERNET0(Default)", "Name of the Virtual Switch")
	vlanID := flag.Int("vlan-id", 0, "VLAN ID for the Client Network Adapter")

	// Storage Type Selection
	storageTypes := flag.String("storage", "physical,virtual,optical", "Comma-separated storage types: physical,virtual,optical")

	// SVC Config (for Physical Disk)
	svcIP := flag.String("svc-ip", "", "SVC IP address (Required if using physical storage)")
	svcUser := flag.String("svc-user", "", "SVC Username (Required if using physical storage)")
	svcPass := flag.String("svc-pass", "", "SVC Password (Required if using physical storage)")
	baseImageName := flag.String("base-image", "", "Base image name for FlashCopy (leave empty to create fresh volume without FlashCopy)")

	// Virtual Disk Config
	targetVios := flag.String("vios-name", "", "Target VIOS for virtual disk (Leave empty for auto-select)")
	targetVg := flag.String("vg-name", "auto_vg01", "Target Volume Group (Leave empty for auto-select)")
	virtualDiskName := flag.String("vdisk-name", "", "Name of the Virtual Disk (auto-generated if empty)")
	virtualDiskSize := flag.Int("vdisk-size", 51200, "Size of the Virtual Disk in Megabytes")

	// Optical Media Config
	mediaNamesStr := flag.String("media-names", "", "Comma-separated list of ISO files to map (e.g., 'rhel9.iso,aix73.iso'). Leave empty to skip optical mapping.")

	// Processor Configuration
	dedicatedProc := flag.Bool("dedicated-proc", false, "Use dedicated processors (default: shared)")
	
	// Power-on Configuration
	powerOn := flag.Bool("power-on", false, "Power on the LPAR after creation (default: false)")
	
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel() // Automatically cleans up the timer/goroutine the second the function exits

	// Parse storage types
	storageTypesMap := parseStorageTypes(*storageTypes)
	usePhysical := storageTypesMap["physical"]
	useVirtual := storageTypesMap["virtual"]
	useOptical := storageTypesMap["optical"]

	// Derive names from LPAR name with timestamp for uniqueness
	timestamp := time.Now().Unix() % 10000 // Last 4 digits of timestamp
	physicalVolumeName := fmt.Sprintf("%s_boot_%d", strings.ReplaceAll(*lparName, " ", "_"), timestamp)
	if *virtualDiskName == "" {
		// AIX/VIOS has 15-char limit for LV names, so use shorter format
		cleanLpar := strings.ReplaceAll(*lparName, " ", "_")
		if len(cleanLpar) > 8 {
			cleanLpar = cleanLpar[:8]
		}
		*virtualDiskName = fmt.Sprintf("%s_d%d", cleanLpar, timestamp)
	}

	log.Println("=========================================================================")
	log.Println(" 🚀 Starting PowerVM LPAR Provisioning")
	if usePhysical {
		log.Println("    ✓ Physical SAN Disk (via SVC FlashCopy)")
	}
	if useVirtual {
		log.Println("    ✓ Virtual Disk (Native VIOS LV)")
	}
	if useOptical && *mediaNamesStr != "" {
		log.Println("    ✓ Virtual Optical Media (ISO files)")
	}
	log.Println("=========================================================================")

	// =========================================================================
	// 1. AUTHENTICATION (HMC + SVC if needed)
	// =========================================================================
	log.Println("")
	log.Println("🔀 Phase 1: Authentication...")

	var restClient *hmc.RestClient
	var svcclient *svc.Client
	var wg sync.WaitGroup
	var hmcErr, svcErr error

	// Always authenticate to HMC
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[Auth-HMC] Connecting to HMC...")
		restClient = exutil.NewClient(*hmcIP, *debug, *debugFull)
		if err := restClient.Login(context.Background(), *username, *password); err != nil {
			hmcErr = fmt.Errorf("HMC login failed: %v", err)
			return
		}
		log.Println("[Auth-HMC] ✅ HMC Authentication Successful")
	}()

	// Authenticate to SVC only if physical storage is requested
	if usePhysical {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("[Auth-SVC] Connecting to SVC...")
			svcclient = svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
			if err := svcclient.Authenticate(context.Background()); err != nil {
				svcErr = fmt.Errorf("SVC authentication failed: %v", err)
				return
			}
			log.Println("[Auth-SVC] ✅ SVC Authentication Successful")
		}()
	}

	wg.Wait()

	// Check for authentication errors
	if hmcErr != nil {
		log.Fatalf("❌ %v", hmcErr)
	}
	if svcErr != nil {
		log.Fatalf("❌ %v", svcErr)
	}

	defer restClient.Logoff(context.Background())

	log.Println("✅ Authentication completed successfully")

	// =========================================================================
	// 2. SYSTEM RESOLUTION & VALIDATION
	// =========================================================================
	sysUUID := resolveSystemUUID(restClient, *sysName)
	ensureLparDoesNotExist(restClient, sysUUID, *lparName)

	// =========================================================================
	// 3. PARALLEL: CREATE LPAR || DISCOVER VIOS || RESOLVE VSWITCH
	// =========================================================================
	log.Println("")
	log.Println("🔀 Phase 2: 3-Way Parallel Operations (LPAR || VIOS || vSwitch)...")

	// Channel-based synchronization for better error handling
	type lparResult struct {
		details *hmc.LogicalPartitionDetailed
		uuid    string
	}
	lparResCh := make(chan lparResult, 1)
	lparErrCh := make(chan error, 1)
	
	type viosResult struct {
		wwpnMap map[string][]string
		uuidMap map[string]string
	}
	viosResCh := make(chan viosResult, 1)
	viosErrCh := make(chan error, 1)
	
	vswitchResCh := make(chan string, 1)
	vswitchErrCh := make(chan error, 1)

	parallelOps := 2 // LPAR + vSwitch
	if usePhysical {
		parallelOps = 3 // Add VIOS discovery
	}
	wg.Add(parallelOps)

	// Thread 1: Create LPAR
	go func() {
		defer wg.Done()
		log.Printf("[Thread-LPAR] Creating LPAR '%s'...", *lparName)
		var req hmc.CreateLparRequest
		if *dedicatedProc {
			// Dedicated processor configuration
			req = hmc.CreateLparRequest{
				Name:             *lparName,
				OsType:           *osType,
				MinMem:           153600,
				DesiredMem:       307200,
				MaxMem:           614400,
				MinProcUnits:     8,    // 2 dedicated processors
				DesiredProcUnits: 16,    // 4 dedicated processors
				MaxProcUnits:     32,    // 8 dedicated processors
				SharingMode:      "sre idle proces",
				DedicatedProc:    true,
			}
		} else {
			// Shared processor configuration (default)
			req = hmc.CreateLparRequest{
				Name:             *lparName,
				OsType:           *osType,
				MinMem:           2048,
				DesiredMem:       32768,
				MaxMem:           65536,
				MinProcUnits:     0.1,
				DesiredProcUnits: 1,
				MaxProcUnits:     2.0,
				MinVcpus:         1,
				DesiredVcpus:     2,
				MaxVcpus:         4,
				SharingMode:      "uncapped",
				DedicatedProc:    false,
			}
		}

		lparDetails, err := restClient.CreateLogicalPartition(sysUUID, req)
		if err != nil {
			lparErrCh <- fmt.Errorf("LPAR creation failed: %v", err)
			return
		}
		lparUUID := lparDetails.MetadataID
		log.Printf("[Thread-LPAR] ✅ LPAR Created! UUID: %s", lparUUID)
		
		// Early unlock: Send result immediately for network/storage operations
		lparResCh <- lparResult{details: lparDetails, uuid: lparUUID}
	}()

	// Thread 2: Discover VIOS WWPNs (only if physical storage needed)
	if usePhysical {
		go func() {
			defer wg.Done()
			log.Println("[Thread-VIOS] Discovering VIOS WWPNs...")
			wwpnMap, uuidMap, err := getViosWwpnMap(restClient, sysUUID)
			if err != nil {
				viosErrCh <- fmt.Errorf("VIOS discovery failed: %v", err)
				return
			}
			log.Println("[Thread-VIOS] ✅ VIOS Discovery Complete.")
			viosResCh <- viosResult{wwpnMap: wwpnMap, uuidMap: uuidMap}
		}()
	}

	// Thread 3: Resolve Virtual Switch
	go func() {
		defer wg.Done()
		log.Printf("[Thread-vSwitch] Resolving Virtual Switch '%s'...", *vswitchName)
		switches, err := restClient.GetVirtualSwitchQuickAll(context.Background(), sysUUID)
		if err != nil {
			vswitchErrCh <- fmt.Errorf("failed to retrieve Virtual Switches: %v", err)
			return
		}

		var foundUUID string
		for _, s := range switches {
			if strings.EqualFold(s.SwitchName, *vswitchName) {
				foundUUID = s.UUID
				break
			}
		}
		if foundUUID == "" {
			vswitchErrCh <- fmt.Errorf("virtual Switch '%s' not found", *vswitchName)
			return
		}
		log.Printf("[Thread-vSwitch] ✅ Virtual Switch Resolved: %s", foundUUID)
		vswitchResCh <- foundUUID
	}()

	wg.Wait()

	// Collect results using select statements for cleaner error handling
	var lparRes lparResult
	select {
	case err := <-lparErrCh:
		log.Fatalf("❌ LPAR Thread Failed: %v", err)
	case lparRes = <-lparResCh:
	}
	
	var viosWwpnMap map[string][]string
	var viosUuidMap map[string]string
	if usePhysical {
		select {
		case err := <-viosErrCh:
			log.Fatalf("❌ VIOS Thread Failed: %v", err)
		case viosRes := <-viosResCh:
			viosWwpnMap = viosRes.wwpnMap
			viosUuidMap = viosRes.uuidMap
		}
	}
	
	var vswitchUUID string
	select {
	case err := <-vswitchErrCh:
		log.Fatalf("❌ vSwitch Thread Failed: %v", err)
	case vswitchUUID = <-vswitchResCh:
	}

	log.Println("✅ All parallel operations completed successfully")
	
	// Extract LPAR details from result
	lparDetails := lparRes.details
	lparUUID := lparRes.uuid

	// =========================================================================
	// 4. PARALLEL: ATTACH NETWORK || PROVISION STORAGE
	// =========================================================================
	log.Println("")
	log.Println("🔀 Phase 3: Parallel Operations (Network || Storage)...")
	
	networkResCh := make(chan *hmc.ClientNetworkAdapter, 1)
	networkErrCh := make(chan error, 1)
	
	// Thread: Attach Network Adapter
	go func() {
		log.Printf("[Thread-Network] Attaching VLAN %d to LPAR...", *vlanID)
		adapter, err := restClient.CreateClientNetworkAdapter(context.Background(), sysUUID, lparUUID, vswitchUUID, *vlanID)
		if err != nil {
			networkErrCh <- fmt.Errorf("failed to add network adapter: %v", err)
			return
		}
		log.Printf("[Thread-Network] ✅ Network Adapter Attached.")
		log.Printf("[Thread-Network]    Adapter UUID: %s", adapter.UUID)
		log.Printf("[Thread-Network]    MAC Address: %s", hmc.FormatMACAddress(adapter.MACAddress))
		log.Printf("[Thread-Network]    Virtual Slot: %s", adapter.VirtualSlotNumber)
		log.Printf("[Thread-Network]    Location Code: %s", adapter.LocationCode)
		networkResCh <- adapter
	}()

	// Storage provisioning runs in parallel with network attachment
	type physicalStorageResult struct {
		targetVol        *svc.Vdisk
		selectedViosName string
		selectedViosUUID string
		diskName         string
	}

	type virtualStorageResult struct {
		viosUUID string
		viosName string
	}

	physicalResCh := make(chan physicalStorageResult, 1)
	physicalErrCh := make(chan error, 1)
	virtualResCh := make(chan virtualStorageResult, 1)
	virtualErrCh := make(chan error, 1)

	storageOps := 0
	if usePhysical {
		storageOps++
	}
	if useVirtual {
		storageOps++
	}

	if storageOps > 0 {
		wg.Add(storageOps)

		// Thread: Provision Physical SAN Disk (if requested)
		if usePhysical {
			go func() {
				defer wg.Done()
				log.Println("[Thread-Physical] Starting SVC storage provisioning...")

				targetVol, selectedViosName, err := provisionSVCStorage(context.Background(), svcclient, *baseImageName, viosWwpnMap, physicalVolumeName)
				if err != nil {
					physicalErrCh <- fmt.Errorf("failed to provision SVC storage: %v", err)
					return
				}

				selectedViosUUID := viosUuidMap[selectedViosName]

				log.Printf("[Thread-Physical] Running ConfigDevice (cfgdev) to scan for the new SVC LUN...")
				if err := restClient.ConfigDevice(ctx, selectedViosUUID, ""); err != nil {
					physicalErrCh <- fmt.Errorf("failed to run cfgdev: %v", err)
					return
				}

				log.Printf("[Thread-Physical] Locating new physical volume matching SVC UID: %s...", targetVol.VdiskUID)
				diskName, err := identifyFreeVolume(ctx, restClient, selectedViosUUID, selectedViosName, targetVol.VdiskUID)
				if err != nil {
					physicalErrCh <- fmt.Errorf("failed to identify free volume: %v", err)
					return
				}
				log.Printf("[Thread-Physical] ✅ Matched SVC LUN to VIOS Disk: %s", diskName)

				physicalResCh <- physicalStorageResult{
					targetVol:        targetVol,
					selectedViosName: selectedViosName,
					selectedViosUUID: selectedViosUUID,
					diskName:         diskName,
				}
			}()
		}

		// Thread: Provision Virtual Disk (if requested)
		if useVirtual {
			go func() {
				defer wg.Done()
				log.Printf("[Thread-Virtual] Discovering optimal Volume Group for %d MB virtual disk...", *virtualDiskSize)

				viosUUID, viosName, err := provisionVirtualDisk(restClient, *sysName, sysUUID, *virtualDiskName, *targetVios, *targetVg, *virtualDiskSize)
				if err != nil {
					virtualErrCh <- fmt.Errorf("failed to provision virtual disk: %v", err)
					return
				}

				log.Printf("[Thread-Virtual] ✅ Virtual Disk '%s' Provisioned on VIOS '%s'.", *virtualDiskName, viosName)
				virtualResCh <- virtualStorageResult{viosUUID: viosUUID, viosName: viosName}
			}()
		}

		wg.Wait()
	}

	// =========================================================================
	// 5. SYNCHRONIZATION: COLLECT ALL RESULTS
	// =========================================================================
	log.Println("")
	log.Println("🔗 Phase 4: Synchronizing parallel operations...")
	
	// Wait for network attachment
	select {
	case err := <-networkErrCh:
		log.Fatalf("❌ Network Thread Failed: %v", err)
	case <-networkResCh:
	}
	
	// Collect storage results
	var physicalStorage physicalStorageResult
	var virtualStorage virtualStorageResult
	var storageViosUUID, storageViosName string // For optical media mapping

	if usePhysical {
		select {
		case err := <-physicalErrCh:
			log.Fatalf("❌ Physical Storage Thread Failed: %v", err)
		case physicalStorage = <-physicalResCh:
			storageViosUUID = physicalStorage.selectedViosUUID
			storageViosName = physicalStorage.selectedViosName
		}
	}

	if useVirtual {
		select {
		case err := <-virtualErrCh:
			log.Fatalf("❌ Virtual Storage Thread Failed: %v", err)
		case virtualStorage = <-virtualResCh:
			// Virtual storage takes precedence for optical media if both exist
			storageViosUUID = virtualStorage.viosUUID
			storageViosName = virtualStorage.viosName
		}
	}

	log.Println("✅ All parallel operations completed successfully")

	// =========================================================================
	// 6. MAP DISKS TO LPAR
	// =========================================================================
	log.Println("")
	log.Println("[HMC] Phase 5: Mapping storage to LPAR...")

	// Map Physical Disk (if provisioned)
	if usePhysical {
		log.Printf("Attaching Physical Disk '%s' to LPAR '%s'...: physicalStorage.diskName=%v", *lparName)
		mappingUUID1, err := restClient.CreatePhysicalVolumeMaps(sysUUID, physicalStorage.selectedViosUUID, lparUUID, []string{physicalStorage.diskName})
		if err != nil {
			log.Fatalf("[HMC] Physical Storage Mapping Failed: %v", err)
		}

		if mappingUUID1 == "SUCCESS_WITH_RMC_WARNING" {
			log.Println("✅ Physical disk mapped successfully! (Ignored expected RMC warning for offline LPAR)")
		} else {
			log.Println("✅ Physical disk mapped successfully!")
		}
	}

	// Map Virtual Disk (if provisioned)
	if useVirtual {
		log.Printf("Attaching Virtual Disk '%s' to LPAR '%s'...: *virtualDiskName=%v", *lparName)
		mappingUUID2, err := restClient.CreateVirtualDiskMaps(sysUUID, virtualStorage.viosUUID, lparUUID, []string{*virtualDiskName})
		if err != nil {
			log.Fatalf("[HMC] Virtual Storage Mapping Failed: %v", err)
		}

		if mappingUUID2 == "SUCCESS_WITH_RMC_WARNING" {
			log.Println("✅ Virtual disk mapped successfully! (Ignored expected RMC warning for offline LPAR)")
		} else {
			log.Println("✅ Virtual disk mapped successfully!")
		}
	}

	// =========================================================================
	// 7. MAP OPTICAL MEDIA (OPTIONAL)
	// =========================================================================
	var opticalMediaCount int
	if useOptical && *mediaNamesStr != "" && storageViosUUID != "" {
		log.Println("")
		log.Println("[HMC] Phase 6: Mapping Virtual Optical Media...")
		
		// Parse comma-separated media names
		mediaNames := strings.Split(*mediaNamesStr, ",")
		for i := range mediaNames {
			mediaNames[i] = strings.TrimSpace(mediaNames[i])
		}
		
		log.Printf("Attaching %d Virtual Optical Media to LPAR '%s'...: len(mediaNames)=%v", *lparName)
		mappingUUID3, err := restClient.CreateVirtualOpticalMaps(context.Background(), sysUUID, storageViosUUID, lparUUID, mediaNames)
		if err != nil {
			log.Printf("⚠️  Warning: Virtual Optical Media mapping failed:: %v", err)
		} else {
			if mappingUUID3 == "SUCCESS_WITH_RMC_WARNING" {
				log.Println("✅ Virtual optical media mapped successfully! (Ignored expected RMC warning for offline LPAR)")
			} else {
				log.Println("✅ Virtual optical media mapped successfully!")
			}
			opticalMediaCount = len(mediaNames)
		}
	}

	// =========================================================================
	// 8. SAVE CONFIGURATION & POWER ON THE LPAR
	// =========================================================================
	// Use the default profile name from the LPAR details
	profileName := lparDetails.DefaultProfileName
	log.Println("")
	log.Printf("Phase 7: Saving configuration to profile '%s'...: %v", profileName)
	if err := restClient.SaveCurrentLparConfig(context.Background(), lparUUID, profileName, true); err != nil {
		log.Fatalf("[HMC] Failed to save LPAR configuration: %v", err)
	}
	log.Println("✅ Configuration permanently saved to profile.")

	log.Println("")
	log.Printf("Phase 8: Powering on LPAR '%s'...: %v", *lparName)

	// Extract profile UUID from the AssociatedPartitionProfile href (already available from CreateLogicalPartition)
	profileHref := lparDetails.AssociatedPartitionProfile.Href
	if profileHref == "" {
		log.Fatalf("[HMC] No associated partition profile found for LPAR")
	}
	
	// Extract UUID from href (last 36 characters)
	if len(profileHref) < 36 {
		log.Fatalf("[HMC] Invalid profile href format: %s", profileHref)
	}
	profileUUID := profileHref[len(profileHref)-36:]
	

	// Power on LPAR if requested
	lparStatus := "READY (not powered on)"
	if *powerOn {
		log.Println("\n⚡ Powering on LPAR...")
		// Create PowerOnOptions
		options := &hmc.PowerOnOptions{
			ProfileUUID: profileUUID,
			Keylock:     "normal",
			OSType:      *osType,
		}
		if _, err := restClient.PowerOnPartition(ctx, lparUUID, options); err != nil {
			log.Fatalf("[HMC] Failed to PowerOn Partition: %v", err)
		}
		lparStatus = "BOOTING"
	}

	log.Println("")
	log.Println("=========================================================================")
	log.Printf(" 🎉 SUCCESS: PowerVM Provisioning Complete!")
	log.Printf("    - LPAR Name      : %s is %s", *lparName, lparStatus)
	log.Printf("    - Network        : Attached to %s (VLAN %d)", *vswitchName, *vlanID)
	if usePhysical {
		log.Printf("    - Physical Disk  : SVC Vol '%s' (%s) via %s", physicalStorage.targetVol.Name, physicalStorage.diskName, physicalStorage.selectedViosName)
	}
	if useVirtual {
		log.Printf("    - Virtual Disk   : Native LV '%s' via %s", *virtualDiskName, virtualStorage.viosName)
	}
	if opticalMediaCount > 0 {
		log.Printf("    - Optical Media  : %d ISO(s) mounted via %s", opticalMediaCount, storageViosName)
	}
	log.Println("=========================================================================")
}

// =========================================================================
// HELPER FUNCTIONS
// =========================================================================

// parseStorageTypes parses comma-separated storage types into a map
func parseStorageTypes(storageStr string) map[string]bool {
	result := make(map[string]bool)
	types := strings.Split(storageStr, ",")
	for _, t := range types {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "physical" || t == "virtual" || t == "optical" {
			result[t] = true
		}
	}
	return result
}

func resolveSystemUUID(restClient *hmc.RestClient, systemName string) string {
	systems, err := restClient.GetManagedSystemQuickAll(context.Background())
	if err != nil {
		log.Fatalf("[HMC] Failed to get managed systems: %v", err)
	}
	for _, system := range systems {
		if strings.EqualFold(system.SystemName, systemName) {
			if false {
				log.Printf("Resolved Managed System UUID: %s: %v", system.UUID)
			}
			return system.UUID
		}
	}
	log.Fatalf("[HMC] Managed system '%s' not found.", systemName)
	return ""
}

func ensureLparDoesNotExist(restClient *hmc.RestClient, systemUUID, vmName string) {
	if false {
		log.Printf("Verifying LPAR name '%s' is unique...: %v", vmName)
	}
	_, existingUUID, err := restClient.GetLogicalPartitionByName(context.Background(), systemUUID, vmName)
	if err == nil && existingUUID != "" {
		log.Fatalf("[HMC] Error: LPAR with name '%s' already exists (UUID: %s)", vmName, existingUUID)
	}
}

func getViosWwpnMap(restClient *hmc.RestClient, systemUUID string) (map[string][]string, map[string]string, error) {
	viosList, err := restClient.GetVirtualIOServers(systemUUID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch VIOS details: %v", err)
	}

	viosWwpnMap := make(map[string][]string)
	viosUuidMap := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range viosList {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v := viosList[idx]
			var wwpns []string
			// Extract WWPNs from nested structure
			for _, profileSlot := range v.PartitionIOConfiguration.ProfileIOSlots {
				fcAdapter := profileSlot.AssociatedIOSlot.RelatedIOAdapter.PhysicalFibreChannelAdapter
				for _, port := range fcAdapter.PhysicalFibreChannelPorts {
					if port.WWPN != "" {
						wwpns = append(wwpns, strings.ToUpper(port.WWPN))
					}
				}
			}
			if len(wwpns) > 0 {
				mu.Lock()
				viosWwpnMap[v.PartitionName] = wwpns
				viosUuidMap[v.PartitionName] = v.PartitionUUID
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if len(viosWwpnMap) == 0 {
		return nil, nil, fmt.Errorf("no Fibre Channel WWPNs found on any VIOS")
	}
	return viosWwpnMap, viosUuidMap, nil
}

func provisionSVCStorage(ctx context.Context, svcclient *svc.Client, baseImageName string, viosWwpnMap map[string][]string, volumeName string) (*svc.Vdisk, string, error) {

	var selectedViosName string
	var selectedHostName string
	var selectedWWPNs []string
	hostExists := false

	fabricLogins, err := svcclient.Lsfabric(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch fabric logins: %v", err)
	}

	wwpnToHostMap := make(map[string]string)
	for _, login := range fabricLogins {
		wwpnToHostMap[strings.ToUpper(login.RemoteWWPN)] = login.HostName
	}

	for viosName, wwpns := range viosWwpnMap {
		for _, wwpn := range wwpns {
			if hostName, found := wwpnToHostMap[strings.ToUpper(wwpn)]; found {
				if false {
					log.Printf("✅ Match Found! VIOS '%s' is mapped to SVC Host '%s': viosName=%v", hostName)
				}
				selectedViosName = viosName
				selectedHostName = hostName
				selectedWWPNs = wwpns
				hostExists = true
				break
			}
		}
		if hostExists {
			break
		}
	}

	if !hostExists {
		if false {
			log.Println("[SVC] ⚠️ No matching SVC host found. Preparing to create a new host mapping...")
		}
		for viosName, wwpns := range viosWwpnMap {
			selectedViosName = viosName
			selectedHostName = viosName
			selectedWWPNs = wwpns
			break
		}
		newHost := svc.Host{
			Name:     selectedHostName,
			Fcwwpn:   selectedWWPNs,
			Type:     "generic",
			Protocol: "scsi",
		}
		if err := svcclient.Mkhost(ctx, newHost); err != nil && !strings.Contains(err.Error(), "already exists") {
			return nil, "", fmt.Errorf("mkhost error: %v", err)
		}
	}

	targetHost, err := svcclient.LshostByTarget(ctx, selectedHostName)
	if err != nil {
		return nil, "", fmt.Errorf("error finding host %s: %v", selectedHostName, err)
	}

	grainSize := 256
	volume := svc.Volume{
		Name:       volumeName,
		MdiskGrp:   "0",
		Size:       120,
		Unit:       "gb",
		RSize:      "2%",
		Warning:    "80%",
		AutoExpand: true,
		GrainSize:  &grainSize,
	}
	if err := svcclient.Mkvdisk(ctx, volume); err != nil {
		return nil, "", fmt.Errorf("mkvdisk error: %v", err)
	}

	targetVol, err := svcclient.LsVdiskByName(ctx, volume.Name)
	if err != nil {
		return nil, "", fmt.Errorf("error finding target volume %s: %v", volume.Name, err)
	}

	// Perform FlashCopy only if baseImageName is provided
	if baseImageName != "" {
		if false {
			log.Printf("Creating FlashCopy from base image '%s'...: %v", baseImageName)
		}
		
		sourceVol, err := svcclient.LsVdiskByName(ctx, baseImageName)
		if err != nil {
			return nil, "", fmt.Errorf("error finding source volume %s: %v", baseImageName, err)
		}
		if false {
			log.Printf("Creating FlashCopy from base image ID '%s'...: %v", sourceVol.ID)
		}

		copyRate := 150
		fcGrainSize := 256
		fcmapping := svc.FlashCopyMapping{
			Name:        fmt.Sprintf("fcmap_%d", time.Now().Unix()),
			Source:      sourceVol.ID,
			Target:      targetVol.ID,
			CopyRate:    &copyRate,
			GrainSize:   &fcGrainSize,
			Incremental: true,
			AutoDelete:  true,
		}
		if err := svcclient.Mkfcmap(ctx, fcmapping); err != nil {
			return nil, "", fmt.Errorf("mkfcmap error: %v", err)
		}

		fmapping := svc.FlashCopyMappingStart{ID: fcmapping.Name, Prep: true, Restore: true}
		if err := svcclient.Startfcmap(ctx, fmapping); err != nil {
			return nil, "", fmt.Errorf("startfcmap error: %v", err)
		}
		
		if false {
			log.Println("✅ FlashCopy completed successfully")
		}
	} else {
		if false {
			log.Println("Creating fresh volume without FlashCopy...")
		}
	}

	mapping := svc.VolumeHostMap{Host: targetHost.ID, Force: true, VDisk: volume.Name}
	if err := svcclient.Mkvdiskhostmap(ctx, mapping); err != nil {
		return nil, "", fmt.Errorf("mkvdiskhostmap error: %v", err)
	}

	return targetVol, selectedViosName, nil
}

func identifyFreeVolume(ctx context.Context, restClient *hmc.RestClient, viosUUID string, viosName string, VdiskUID string) (string, error) {
	pvList, err := restClient.GetFreePhyVolume(viosUUID)
	if err != nil {
		pvList = []hmc.PhysicalVolume{}
	}

	for _, pv := range pvList {
		if strings.Contains(pv.VolumeUniqueID, VdiskUID) {
			return pv.VolumeName, nil
		}
	}
	if len(pvList) == 0 {
		return "", fmt.Errorf("no free physical volumes found on VIOS %s", viosName)
	}
	return "", fmt.Errorf("volume with UID %s not found on VIOS", VdiskUID)
}

// provisionVirtualDisk performs Smart Capacity Discovery to find the best VG, then creates the disk.
func provisionVirtualDisk(restClient *hmc.RestClient, sysName, sysUUID, diskName, targetVios, targetVg string, diskSizeMB int) (string, string, error) {
	requiredGB := float64(diskSizeMB) / 1024.0

	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID)
	if err != nil || len(viosList) == 0 {
		return "", "", fmt.Errorf("failed to fetch VIOS instances for system")
	}

	var finalViosUUID, finalViosName, finalVgName string
	var usingRootVgFallback bool

	for _, vios := range viosList {
		// Filter by VIOS if provided
		if targetVios != "" && !strings.EqualFold(vios.PartitionName, targetVios) {
			continue
		}

		vgList, err := restClient.GetVolumeGroups(context.Background(), vios.UUID)
		if err != nil {
			continue
		}

		for _, vg := range vgList {
			// Filter by VG if provided
			if targetVg != "" && !strings.EqualFold(vg.GroupName, targetVg) {
				continue
			}

			// Ensure no naming collision exists in this VG
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

			freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
			if parseErr != nil {
				continue
			}

			// Capacity Check
			if freeSpaceGB >= requiredGB {
				if targetVg == "" {
					// Smart selection: Avoid rootvg if possible
					if strings.ToLower(vg.GroupName) == "rootvg" {
						if finalVgName == "" {
							finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
							usingRootVgFallback = true
						}
					} else {
						// Found a perfect Data VG match
						finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
						usingRootVgFallback = false
						break
					}
				} else {
					// Explicit match requested
					finalViosUUID, finalViosName, finalVgName = vios.UUID, vios.PartitionName, vg.GroupName
					break
				}
			}
		}
		if finalVgName != "" && !usingRootVgFallback {
			break
		}
	}

	if finalVgName == "" {
		return "", "", fmt.Errorf("could not find a Volume Group with %.2f GB of free space", requiredGB)
	}

	// Create the disk via the Smart CLI Wrapper
	err = restClient.CreateVirtualDisk(context.Background(), sysName, finalViosUUID, finalViosName, finalVgName, diskName, diskSizeMB)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Virtual Disk via CLI: %v", err)
	}

	return finalViosUUID, finalViosName, nil
}

// Made with Bob
