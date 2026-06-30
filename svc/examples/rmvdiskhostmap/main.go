package main

import (
	"log"
	"context"
	"flag"
	"os"

	"github.com/IBM/infra-go-sdk/svc"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()
	_ = verbose

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: rmvdiskhostmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}


	ctx := context.Background()

	
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	mapping := svc.VolumeHostMap{
		Host:  "host1",
		SCSI:  "1",
		Force: true,
		VDisk: "test_volume2",
	}

	log.Printf("Mapping volume to host...: volume=%v host=%v", mapping.VDisk, mapping.Host)

	if err := client.Mkvdiskhostmap(ctx,mapping); err != nil {
		log.Printf("Mkvdiskhostmap error: error=%v", err)
		os.Exit(1)
	}
	
	log.Printf("✅ Successfully created volume host mapping: volume=%v host=%v", mapping.VDisk, mapping.Host)
}
