package main

import (
	"log"
	"context"
	"flag"
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
	_ = verbose

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: mkvdiskhostmap -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()
	
	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	hostName := "ltc09u31-vios1"
	volName := "test_volume3" 

	log.Printf("Attempting to unmap volume from host...: volume=%v host=%v", volName, hostName)

	err := client.Rmvdiskhostmap(ctx,hostName, volName)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "CMMVC6071E") {
			log.Println("✅ Volume is already unmapped from this host. Nothing to do.")
		} else if strings.Contains(errStr, "CMMVC5754E") {
			log.Printf("Host or volume doesn't exist: volume=%v host=%v", volName, hostName)
		} else {
			log.Printf("Failed to unmap: error=%v", err)
			os.Exit(1)
		}
	} else {
		log.Printf("✅ Successfully unmapped volume: volume=%v host=%v", volName, hostName)
	}
}