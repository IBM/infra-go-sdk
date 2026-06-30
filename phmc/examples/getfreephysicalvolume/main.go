package main

import (
	"flag"
	"context"
	"log"

	hmc "github.com/IBM/infra-go-sdk/phmc"
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
)

func main() {
	hmcIP    := flag.String("hmc-ip",    "", "HMC IP address")
	username := flag.String("hmc-user",  "", "HMC username")
	password := flag.String("hmc-pass",  "", "HMC password")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()

	hmcIPVal    := *hmcIP
	usernameVal := *username
	passwordVal := *password

	verbose := true
	viosUUID := "0625F241-08C9-461D-9FA6-B46620D6FDB1"

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

	// Get free physical volumes for the VIOS
	pvList, err := restClient.GetFreePhyVolume(viosUUID)
	if err != nil {
		// Log the error and assume no volumes are available
		if verbose {
			log.Printf("Error getting free physical volumes for VIOS %v: %v", pvList, err)
		}
		pvList = []hmc.PhysicalVolume{} // Treat as no volumes found
	}

}
