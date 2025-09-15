package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/mkumatag/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// List all volumes
	volumes, err := client.LsVdisk()
	if err != nil {
		log.Fatalf("LsVdisk error: %v", err)
	}

	volumeName := "test_volume2"
	var foundVolume *svc.VolumeInfo
	for _, vol := range volumes {
		if vol.Name == volumeName {
			foundVolume = &vol
			break
		}
	}

	if foundVolume != nil {
		fcMapCount, _ := strconv.Atoi(foundVolume.FCMapCount)
		if foundVolume.FCMapCount == "" {
			fcMapCount = 0
		}
		fmt.Printf("Successfully retrieved disk with name: %s\n", foundVolume.Name)
		fmt.Printf("Details: MdiskGrp: %s, Capacity: %s, Status: %s, Type: %s, FC Map Count: %d, UID: %s\n",
			foundVolume.MDiskGrpName, foundVolume.Capacity, foundVolume.Status, foundVolume.Type, fcMapCount, foundVolume.VdiskUID)
	} else {
		fmt.Printf("No disk found with name: %s\n", volumeName)
	}

	/* if len(volumes) == 0 {
		fmt.Println("No volumes found")
	} else {
		fmt.Println("\nAll Volumes:")
		for _, vol := range volumes {
			// Convert FCMapCount to int for display, default to 0 if empty or invalid
			fcMapCount, _ := strconv.Atoi(vol.FCMapCount)
			if vol.FCMapCount == "" {
				fcMapCount = 0
			}
			fmt.Printf("Name: %s, MdiskGrp: %s, Capacity: %s, Status: %s, Type: %s, FC Map Count: %d, Volume UID: %s\n",
				vol.Name, vol.MDiskGrpName, vol.Capacity, vol.Status, vol.Type, fcMapCount, vol.VdiskUID)
		}
	} */
}
