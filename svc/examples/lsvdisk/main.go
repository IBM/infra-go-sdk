package main

import (
	"log"
	"context"
	"flag"
	"os"
	"strconv"
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
		log.Fatal("Usage: lsvdisk -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(ctx); err != nil {
		log.Printf("Authentication error: error=%v", err)
		os.Exit(1)
	}

	volumeName := "test_volume2"
	log.Printf("Searching for volume...: volume=%v", volumeName)

	foundVolume, err := client.LsVdiskByName(ctx,volumeName)
	if err != nil {
		if strings.Contains(err.Error(), "CMMVC5754E") {
			log.Printf("No disk found with name: volume=%v", volumeName)
		} else {
			log.Printf("LsVdiskByName error: error=%v", err)
			os.Exit(1)
		}
	} else {
		fcMapCount, _ := strconv.Atoi(foundVolume.FCMapCount)
		log.Printf("✅ Successfully retrieved disk: name=%v", foundVolume.Name)
		log.Printf("[DEBUG] Disk Details %v", "mdisk_grp", foundVolume.MdiskGrpName,
			"capacity", foundVolume.Capacity,
			"status", foundVolume.Status,
			"type", foundVolume.Type,
			"fc_map_count", fcMapCount,
			"uid", foundVolume.VdiskUID,)
	}
}
