package main

import (
	"fmt"
	"log"

	"github.com/beevik/etree"
	hmc "github.com/sudeeshjohn/PowerHMC"
)

func main() {
	hmcIP      := "192.0.2.1"
	username   := "REDACTED_HMC_USER<=="
	password   := "REDACTED_HMC_PASS<=="
	systemUUID := "49672f05-253d-30bc-ae09-ecd76cb410ce"
	verbose    := false 

	restClient := hmc.NewHmcRestClient(hmcIP)

	fmt.Printf("Connecting to HMC at %s...\n", hmcIP)
	if err := restClient.Login(username, password, verbose); err != nil {
		log.Fatalf("Logon failed: %v", err)
	}

	defer func() {
		if err := restClient.Logoff(); err != nil {
			log.Printf("Logoff failed: %v", err)
		} else {
			fmt.Println("Logged off successfully.")
		}
	}()

	// Retrieve partition properties
	partitionData, err := restClient.GetLogicalPartitions(systemUUID, verbose)
	
	// FIX 1: Check the error immediately
	if err != nil {
		log.Fatalf("API Error: %v", err)
	}

	// FIX 2: Check if partitionData is nil before calling .Copy()
	if partitionData == nil {
		log.Fatal("Error: Received nil partition data from HMC. Check if System UUID is correct.")
	}

	doc := etree.NewDocument()
	doc.SetRoot(partitionData.Copy())

	// FIX 3: Check if the document actually has a root before processing
	root := doc.Root()
	if root == nil {
		log.Fatal("Error: XML document has no root element.")
	}

	// List LPARs
	entries := doc.FindElements("//Entry")
	if len(entries) == 0 {
		fmt.Println("No partitions found for this Managed System.")
		return
	}

	fmt.Printf("\nFound %d partitions:\n", len(entries))
	for i, entry := range entries {
		name := entry.FindElement(".//PartitionName")
		id := entry.FindElement(".//PartitionID")
		state := entry.FindElement(".//PartitionState")

		nStr, iStr, sStr := "Unknown", "Unknown", "Unknown"
		if name != nil { nStr = name.Text() }
		if id != nil { iStr = id.Text() }
		if state != nil { sStr = state.Text() }

		fmt.Printf("%d. [ID: %s] %-20s Status: %s\n", i+1, iStr, nStr, sStr)
	}
}
