package main

import (
	"flag"
	"context"
	"encoding/json"
	"fmt"
	"log"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	// --- Configuration ---
	hmcIP    := flag.String("hmc-ip",    "", "HMC IP address")
	username := flag.String("hmc-user",  "", "HMC username")
	password := flag.String("hmc-pass",  "", "HMC password")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()

	hmcIPVal    := *hmcIP
	usernameVal := *username
	passwordVal := *password

	sysName  := "" // Enter the System Name here
	lparName := ""       // Enter the Partition Name here
	verbose  := false

	// Initialize HmcRestClient
	restClient := exutil.NewClient(hmcIPVal, *debug, *debugFull)

	// Logon
	if verbose {
		log.Printf("Attempting to log on to HMC at %s with username %s", hmcIP, username)
	}
	if err := restClient.Login(context.Background(), usernameVal, passwordVal); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}
	defer func() {
		if err := restClient.Logoff(context.Background()); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else if verbose {
			log.Println("Logged off successfully")
		}
	}()

	// 1. Resolve System Name to UUID
	fmt.Printf("Step 1: Resolving System Name '%s'...\n", sysName)
	sysUUID, _, err := restClient.GetManagedSystemByName(context.Background(), sysName)
	if err != nil {
		log.Fatalf("Error resolving system name: %v", err)
	}
	if sysUUID == "" {
		log.Fatalf("System '%s' not found.", sysName)
	}

	// 2. Fetch all partitions for this system to find the target partition's UUID
	fmt.Printf("Step 2: Searching for Partition '%s'...\n", lparName)
	partitions, err := restClient.GetLogicalPartitionsQuickAll(context.Background(), sysUUID)
	if err != nil {
		log.Fatalf("Failed to fetch partitions: %v", err)
	}

	// 3. Find the matching partition UUID
	var targetLparUUID string
	for _, p := range partitions {
		if p.PartitionName == lparName {
			targetLparUUID = p.UUID
			break
		}
	}

	if targetLparUUID == "" {
		log.Fatalf("Partition '%s' not found on system '%s'.", lparName, sysName)
	}

	// 4. Fetch Quick details for the specific partition using your function
	fmt.Printf("Step 3: Fetching complete details for UUID: %s...\n\n", targetLparUUID)
	partition, err := restClient.GetLogicalPartitionQuick(targetLparUUID)
	if err != nil {
		log.Fatalf("Failed to retrieve partition properties: %v", err)
	}

	// 5. Print ALL details elegantly using JSON Marshaling
	// This takes your LogicalPartitionQuick struct and formats it cleanly
	prettyJSON, err := json.MarshalIndent(partition, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal partition details: %v", err)
	}

	fmt.Printf("--- Details for Partition: %s ---\n", partition.PartitionName)
	fmt.Println(string(prettyJSON))
}
