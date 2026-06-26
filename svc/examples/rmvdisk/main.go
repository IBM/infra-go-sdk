package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/IBM/infra-go-sdk/svc"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "", "SVC IP address (required)")
	svcUser := flag.String("svc-user", "", "SVC username (required)")
	svcPass := flag.String("svc-pass", "", "SVC password (required)")
	flag.Parse()

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: rmvdisk -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	_ = verbose // reserved for future use

	ctx := context.Background()
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Fatalf("Authentication error: %v", err)
	}

	volumeName := "test_volume3"
	removeVolume := svc.VolumeRemove{Force: true, RemoveHostMappings: false}

	log.Printf("Attempting to delete volume: %s", volumeName)

	if err := client.Rmvdisk(ctx, volumeName, removeVolume); err != nil {
		if strings.Contains(err.Error(), "CMMVC5754E") || strings.Contains(err.Error(), "CMMVC5804E") {
			log.Printf("Volume is already deleted (or does not exist). Nothing to do: volume=%s", volumeName)
		} else {
			log.Fatalf("Rmvdisk error: %v", err)
		}
	} else {
		log.Printf("Successfully deleted volume: %s", volumeName)
	}
	os.Exit(0)
}
