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

	// Create a VolumeHostMap instance
	mapping := svc.VolumeHostMap{
		Host:  "host1",        // Host name or ID
		SCSI:  "1",            // Optional SCSI LUN ID
		Force: true,           // Optional force flag
		VDisk: "test_volume2", // Volume name or ID
	}

	// Create the volume to host mapping
	if err := client.Mkvdiskhostmap(mapping); err != nil {
		log.Fatalf("Mkvdiskhostmap error: %v", err)
	} else {
		fmt.Printf("Successfully created volume host mapping for volume: %s to host: %s\n", mapping.VDisk, mapping.Host)
	}
}
