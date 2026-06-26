package main

import (
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

	if *svcIP == "" || *svcUser == "" || *svcPass == "" {
		log.Fatal("Usage: lsvdisk -svc-ip <ip> -svc-user <user> -svc-pass <pass>")
	}

	ctx := context.Background()

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
	if *verbose {
		client = client.WithDebug()
	}

	if err := client.Authenticate(ctx); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}

	volumeName := "test_volume2"
	client.Logger.Info("Searching for volume...", "volume", volumeName)

	foundVolume, err := client.LsVdiskByName(ctx,volumeName)
	if err != nil {
		if strings.Contains(err.Error(), "CMMVC5754E") {
			client.Logger.Warn("No disk found with name", "volume", volumeName)
		} else {
			client.Logger.Error("LsVdiskByName error", "error", err)
			os.Exit(1)
		}
	} else {
		fcMapCount, _ := strconv.Atoi(foundVolume.FCMapCount)
		client.Logger.Info("✅ Successfully retrieved disk", "name", foundVolume.Name)
		client.Logger.Debug("Disk Details",
			"mdisk_grp", foundVolume.MdiskGrpName,
			"capacity", foundVolume.Capacity,
			"status", foundVolume.Status,
			"type", foundVolume.Type,
			"fc_map_count", fcMapCount,
			"uid", foundVolume.VdiskUID,
		)
	}
}