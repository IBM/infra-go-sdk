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
		log.Fatal("Usage: mkfcconsistgrp -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	group := svc.FlashCopyConsistGroup{
		Name:       "test_fcgrp",
		AutoDelete: false,
	}

	log.Printf("Creating FlashCopy consistency group...: name=%v", group.Name)

	if err := client.Mkfcconsistgrp(ctx,group); err != nil {
		log.Printf("Mkfcconsistgrp error: error=%v", err)
		os.Exit(1)
	}

	log.Printf("✅ Successfully created FlashCopy consistency group: name=%v", group.Name)
}