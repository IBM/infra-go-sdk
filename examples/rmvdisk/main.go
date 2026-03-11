package main

import (
	"fmt"
	"log"

	"github.com/mkumatag/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Create a VolumeRemove instance
	volumeName := "test_volume2"
	removeVolume := svc.VolumeRemove{
		Force:              true,  // Force deletion if mappings exist
		RemoveHostMappings: false, // Remove host mappings before deletion
	}

	// Delete the volume
	if err := client.Rmvdisk(volumeName, removeVolume); err != nil {
		log.Fatalf("Rmvdisk error: %v", err)
	} else {
		fmt.Printf("Successfully deleted volume with name: %s\n", volumeName)
	}
}
