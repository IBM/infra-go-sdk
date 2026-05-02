package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" {
		log.Fatal("Error: hmc-ip, hmc-user, hmc-pass, and system-name are required.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())
	fmt.Println("✅ Successfully authenticated with HMC.")

	// =========================================================================
	// RESOLVE SYSTEM & VIOS UUIDS
	// =========================================================================
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ System %s not found: %v", *sysName, err)
	}

	viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, *verbose)
	if err != nil {
		log.Fatalf("❌ Failed to fetch VIOS instances: %v", err)
	}
	if len(viosList) == 0 {
		log.Fatalf("No Virtual I/O Servers found on system %s.", *sysName)
	}

	// =========================================================================
	// FETCH & PRINT VOLUME GROUPS
	// =========================================================================
	for _, vios := range viosList {
		fmt.Printf("\n===============================================================================\n")
		fmt.Printf(" VIOS: %s (UUID: %s)\n", vios.PartitionName, vios.UUID)
		fmt.Printf("===============================================================================\n")

		volumeGroups, err := restClient.GetVolumeGroups(context.Background(), vios.UUID, *verbose)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to fetch Volume Groups for %s: %v", vios.PartitionName, err)
			continue
		}

		if len(volumeGroups) == 0 {
			fmt.Println("  No Volume Groups found.")
			continue
		}

		fmt.Printf("  Found %d Volume Group(s):\n", len(volumeGroups))
		for i, vg := range volumeGroups {
			fmt.Printf("\n  📦 [%d] Volume Group: %s\n", i+1, vg.GroupName)
			fmt.Printf("      - UUID:             %s\n", vg.UUID)
			fmt.Printf("      - Capacity:         %s GB\n", vg.GroupCapacity) // Note: HMC reports capacity in GB here
			fmt.Printf("      - Free Space:       %s GB\n", vg.FreeSpace)
			fmt.Printf("      - Serial ID:        %s\n", vg.GroupSerialID)

			// Print Media Repository Info if it exists in this VG
			if vg.MediaRepositoryName != "" {
				fmt.Printf("      - Repository:       ✅ YES (%s | Size: %s GB)\n", vg.MediaRepositoryName, vg.MediaRepositorySize)
			} else {
				fmt.Printf("      - Repository:       ❌ NO\n")
			}

			// Print Virtual Disks (Logical Volumes)
			fmt.Printf("\n      💽 Virtual Disks / Logical Volumes (%d):\n", len(vg.VirtualDisks))
			if len(vg.VirtualDisks) == 0 {
				fmt.Println("         (None found)")
			} else {
				for j, vd := range vg.VirtualDisks {
					fmt.Printf("         %d.%d %-15s | Capacity: %-4s GB | UID: %s\n",
						i+1, j+1, vd.DiskName, vd.DiskCapacity, vd.UniqueDeviceID)
				}
			}

			// Print Physical Volumes
			fmt.Printf("\n      💿 Physical Volumes (%d):\n", len(vg.PhysicalVolumes))
			if len(vg.PhysicalVolumes) == 0 {
				fmt.Println("         (None found)")
			} else {
				for j, pv := range vg.PhysicalVolumes {
					fmt.Printf("         %d.%d %-10s | %-10d MB | State: %-8s | UID: %s\n",
						i+1, j+1, pv.VolumeName, pv.VolumeCapacity, pv.VolumeState, pv.UniqueDeviceID)
				}
			}

			// Print Virtual Optical Media
			fmt.Printf("\n      📀 Virtual Optical Media (%d):\n", len(vg.OpticalMedia))
			if len(vg.OpticalMedia) == 0 {
				fmt.Println("         (No optical media found in repository)")
			} else {
				for k, opt := range vg.OpticalMedia {
					fmt.Printf("         %d.%d %-40s | Mount: %-4s | Size: %s\n",
						i+1, k+1, opt.MediaName, opt.MountType, opt.Size)
				}
			}
			fmt.Println("      -------------------------------------------------------------------------")
		}
	}
}