package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Create a VolumeRemove instance
	volumeName := "test_volume3"
	removeVolume := svc.VolumeRemove{
		Force:              true,  // -force flag: removes even if mapped or part of FC map
		RemoveHostMappings: false, // SDK-specific logic
	}

	fmt.Printf("Attempting to delete volume: %s...\n", volumeName)

	// Delete the volume
	if err := client.Rmvdisk(volumeName, removeVolume); err != nil {
		errStr := err.Error()
		// Catch the "object does not exist" error to make this idempotent
		if strings.Contains(errStr, "CMMVC5754E") || strings.Contains(errStr, "CMMVC5804E") {
			fmt.Printf("✅ Volume '%s' is already deleted (or does not exist). Nothing to do.\n", volumeName)
		} else {
			// Fail loudly for actual errors (e.g., volume is in an active FlashCopy and Force wasn't enough)
			log.Fatalf("❌ Rmvdisk error: %v", err)
		}
	} else {
		fmt.Printf("✅ Successfully deleted volume: %s\n", volumeName)
	}
}