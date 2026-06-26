package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	
	viosName := flag.String("vios-name", "ltc09u31-vios1", "Target VIOS (Required)")
	fileNames := flag.String("file-names", "/mnt/f43.iso", "Comma-separated list of ISO file paths on the VIOS filesystem (e.g., /mnt/file1.iso,/mnt/file2.iso)")
	mediaPrefix := flag.String("media-prefix", fmt.Sprintf("ocp_%d", time.Now().Unix()), "Prefix for media names (will append _1, _2, etc. for multiple files)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *viosName == "" || *fileNames == "" {
		log.Fatal("Error: hmc-pass, vios-name, and file-names are required.")
	}

	// =========================================================================
	// AUTHENTICATION & RESOLUTION
	// =========================================================================
	fmt.Printf("Logging into HMC at %s...\n", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("HMC Logon failed: %v", err)
	}
	defer restClient.Logoff(context.Background())

	system, _, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || system.UUID == "" {
		log.Fatalf("❌ System '%s' not found: %v", *sysName, err)
	}

	viosUUID, err := hmc.GetViosID(context.Background(), restClient, system.UUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatalf("❌ VIOS '%s' not found.", *viosName)
	}

	// =========================================================================
	// PARSE FILE NAMES AND BUILD MEDIA MAP
	// =========================================================================
	// Split comma-separated file names
	fileList := strings.Split(*fileNames, ",")
	for i := range fileList {
		fileList[i] = strings.TrimSpace(fileList[i])
	}
	
	// Build media files map
	mediaFiles := make(map[string]string)
	for i, filePath := range fileList {
		if filePath == "" {
			continue
		}
		
		// Generate media name: if single file, use prefix as-is; if multiple, append base filename
		var mediaName string
		if len(fileList) == 1 {
			mediaName = *mediaPrefix
		} else {
			// Extract base filename without extension for better naming
			baseName := filepath.Base(filePath)
			ext := filepath.Ext(baseName)
			nameWithoutExt := strings.TrimSuffix(baseName, ext)
			mediaName = fmt.Sprintf("%s_%s", *mediaPrefix, nameWithoutExt)
		}
		
		mediaFiles[mediaName] = filePath
		_ = i // Suppress unused variable warning
	}
	
	if len(mediaFiles) == 0 {
		log.Fatal("Error: No valid file names provided.")
	}

	// =========================================================================
	// EXECUTE NATIVE REST ISO IMPORT
	// =========================================================================
	fmt.Printf("\n🚀 Attempting to import %d ISO file(s) into the Media Repository...\n", len(mediaFiles))
	for name, file := range mediaFiles {
		fmt.Printf("   - Media: '%s' from file: '%s'\n", name, file)
	}

	results, err := restClient.AddVirtualOpticalMedia(context.Background(), viosUUID, mediaFiles, *verbose)
	if err != nil {
		log.Printf("⚠️  Warning: Some or all media additions failed: %v", err)
	}

	// Display results
	fmt.Println("\n📊 Results:")
	successCount := 0
	failCount := 0
	
	for mediaName, mediaErr := range results {
		if mediaErr == nil {
			fmt.Printf("   ✅ '%s': Successfully imported\n", mediaName)
			successCount++
		} else {
			fmt.Printf("   ❌ '%s': Failed - %v\n", mediaName, mediaErr)
			failCount++
		}
	}
	
	fmt.Printf("\n💿 Summary: %d succeeded, %d failed out of %d total\n", successCount, failCount, len(results))
	
	if failCount > 0 {
		log.Fatal("❌ Some media additions failed. See results above.")
	}
}
