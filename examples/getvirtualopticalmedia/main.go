package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS")
	
	// Flags for testing targeted query and deletion
	mediaName := flag.String("media-name", "sno-0-agent-1776731165", "Specific media name to query (e.g., 'rhel9.iso')")
	deleteMedia := flag.Bool("delete", false, "Set to true to delete the specified -media-name")
	
	debug := flag.Bool("debug", false, "Enable debug output")
	flag.Parse()

	if *password == "" || *sysName == "" || *viosName == "" {
		log.Fatal("Error: hmc-pass, system-name, and vios-name are required.")
	}

	if *deleteMedia && *mediaName == "" {
		log.Fatal("Error: -delete=true requires -media-name to be specified.")
	}

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *debug); err != nil {
		log.Fatalf("❌ HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	fmt.Println("=========================================================================")
	fmt.Printf(" 💿 Virtual Optical Media Repository Test: %s\n", *viosName)
	fmt.Println("=========================================================================")

	// =========================================================================
	// TEST 1: GET ALL MEDIA (Plural)
	// =========================================================================
	fmt.Printf("\n[Test 1] Fetching ALL Virtual Optical Media...\n")
	
	allMedia, err := restClient.GetVirtualOpticalMedias(context.Background(), *sysName, *viosName, *debug)
	if err != nil {
		log.Fatalf("❌ Failed to fetch all media: %v", err)
	}

	if len(allMedia) == 0 {
		fmt.Println("   ℹ️  No virtual optical media found in repository.")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "   MEDIA NAME\tSIZE (MB)\tMOUNT TYPE")
		fmt.Fprintln(w, "   ----------\t---------\t----------")
		for _, m := range allMedia {
			fmt.Fprintf(w, "   %s\t%s\t%s\n", m.MediaName, m.Size, m.MountType)
		}
		w.Flush()
		fmt.Printf("\n   ✅ Total Media Found: %d\n", len(allMedia))
	}

	// =========================================================================
	// TEST 2: GET SPECIFIC MEDIA (Singular)
	// =========================================================================
	if *mediaName != "" {
		fmt.Printf("\n[Test 2] Querying specific media '%s'...\n", *mediaName)
		
		media, err := restClient.GetVirtualOpticalMedia(context.Background(), *sysName, *viosName, *mediaName, *debug)
		if err != nil {
			fmt.Printf("   ⚠️  Media not found or error occurred: %v\n", err)
		} else {
			fmt.Printf("   ✅ Successfully retrieved details for '%s':\n", media.MediaName)
			fmt.Printf("      - Size:       %s MB\n", media.Size)
			fmt.Printf("      - Mount Type: %s\n", media.MountType)
			
			// =========================================================================
			// TEST 3: DELETE MEDIA
			// =========================================================================
			if *deleteMedia {
				fmt.Printf("\n[Test 3] Deleting media '%s'...\n", *mediaName)
				
				err := restClient.DeleteVirtualOpticalMedia(context.Background(), *sysName, *viosName, *mediaName, *debug)
				if err != nil {
					log.Fatalf("   ❌ Failed to delete media: %v", err)
				}
				
				fmt.Printf("   🗑️  Successfully deleted media '%s' from the repository!\n", *mediaName)
			} else {
				fmt.Printf("\n   💡 Note: Pass '-delete=true' if you want to test deleting this file.\n")
			}
		}
	} else {
		fmt.Println("\n💡 Note: Provide a '-media-name=\"filename.iso\"' flag to test querying specific details.")
	}
	
	fmt.Println("\n=========================================================================")
}