package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/sudeeshjohn/svc-go-sdk" // Adjust if your package path differs
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	volumeName := "test_volume2"
	fmt.Printf("Searching for volume: %s...\n", volumeName)

	// Use the new targeted function instead of listing all and looping client-side
	foundVolume, err := client.LsVdiskByName(volumeName)
	if err != nil {
		// Catch the specific CMMVC5754E error from the IBM API
		if strings.Contains(err.Error(), "CMMVC5754E") {
			fmt.Printf("❌ No disk found with name: %s (CMMVC5754E)\n", volumeName)
		} else {
			log.Fatalf("LsVdiskByName error: %v", err)
		}
	} else {
		// Volume was found! Convert FCMapCount safely.
		fcMapCount, _ := strconv.Atoi(foundVolume.FCMapCount)
		if foundVolume.FCMapCount == "" {
			fcMapCount = 0
		}

		fmt.Printf("✅ Successfully retrieved disk with name: %s\n", foundVolume.Name)
		fmt.Printf("Details: MdiskGrp: %s, Capacity: %s, Status: %s, Type: %s, FC Map Count: %d, UID: %s\n",
			foundVolume.MdiskGrpName, foundVolume.Capacity, foundVolume.Status, foundVolume.Type, fcMapCount, foundVolume.VdiskUID)
	}

	/* // ==========================================
	// Example: Listing ALL volumes if needed
	// ==========================================
	volumes, err := client.LsVdisk()
	if err != nil {
		log.Fatalf("LsVdisk error: %v", err)
	}

	if len(volumes) == 0 {
		fmt.Println("No volumes found")
	} else {
		fmt.Printf("\nFound %d total volumes:\n", len(volumes))
		for _, vol := range volumes {
			fcMapCount, _ := strconv.Atoi(vol.FCMapCount)
			if vol.FCMapCount == "" {
				fcMapCount = 0
			}
			fmt.Printf("Name: %s, MdiskGrp: %s, Capacity: %s, Status: %s, Type: %s, FC Map Count: %d, Volume UID: %s\n",
				vol.Name, vol.MdiskGrpName, vol.Capacity, vol.Status, vol.Type, fcMapCount, vol.VdiskUID)
		}
	} 
	*/
}