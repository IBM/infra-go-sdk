package main

import (
	"fmt"
	"log"

	"github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// Create a Volume instance
	volume := svc.Volume{
		Name:       "test_volume2",
		MdiskGrp:   "0",
		Size:       120,
		Unit:       "gb",
		RSize:      "2%",
		Warning:    "80%",
		AutoExpand: true,
		GrainSize:  256,
	}

	// Create the volume using Mkvdisk
	if err := client.Mkvdisk(volume); err != nil {
		log.Fatalf("Mkvdisk error: %v", err)
	} else {
		fmt.Printf("Successfully created disk with name: %s\n", volume.Name)
	}
}
