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

	// Create a FlashCopyMapping instance
	mapping := svc.FlashCopyMapping{
		Name:        "test_fcmap",
		Source:      "295", //Soure volume ID
		Target:      "224", //Target Volume ID
		CopyRate:    150,   //Specifies the copy rate. The rate value can be 0 - 150;attribute value:141 - 150, Data copied/sec:2 GB, 256 KB grains/sec:8192,64 KB grains/sec:32768
		GrainSize:   256,
		Incremental: true,
		AutoDelete:  true, //Specifies that a mapping is to be deleted when the background copy completes
		//ConsistGrp:  "test_fcgrp",
	}

	// Create the FlashCopy mapping
	if err := client.Mkfcmap(mapping); err != nil {
		log.Fatalf("Mkfcmap error: %v", err)
	} else {
		fmt.Printf("Successfully created FlashCopy mapping with name: %s\n", mapping.Name)
	}
}
