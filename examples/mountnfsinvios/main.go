package main

import (
	"flag"
	"fmt"
	"log"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	// =========================================================================
	// CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name")
	viosName := flag.String("vios-name", "ltc09u31-vios1", "VIOS partition name")
	
	// NFS Mount Parameters
	nfsServer := flag.String("nfs-server", "192.0.2.20", "NFS server IP or hostname")
	exportPath := flag.String("export-path", "/var/www/html/ocp", "NFS export path on server")
	mountPoint := flag.String("mount-point", "/mnt", "Local mount point on VIOS (under /tmp)")
	nfsVersion := flag.String("nfs-version", "3", "NFS version (3 or 4)")
	
	// Action to perform
	action := flag.String("action", "mount", "Action: mount or unmount")
	
	verbose := flag.Bool("verbose", true, "Enable verbose output")
	flag.Parse()

	// Validate required parameters
	if *password == "" || *sysName == "" || *viosName == "" {
		log.Fatal("❌ Error: hmc-pass, system-name, and vios-name are required")
	}

	if *action == "mount" && (*nfsServer == "" || *exportPath == "" || *mountPoint == "") {
		log.Fatal("❌ Error: For mount action, nfs-server, export-path, and mount-point are required")
	}

	if *action == "unmount" && *mountPoint == "" {
		log.Fatal("❌ Error: For unmount action, mount-point is required")
	}

	// Print banner
	fmt.Println("=========================================================================")
	fmt.Printf(" 🗂️  NFS Mount Management for VIOS\n")
	fmt.Printf("    - System: %s\n", *sysName)
	fmt.Printf("    - VIOS:   %s\n", *viosName)
	fmt.Printf("    - Action: %s\n", *action)
	if *action == "mount" {
		fmt.Printf("    - Server: %s:%s\n", *nfsServer, *exportPath)
		fmt.Printf("    - Mount:  %s\n", *mountPoint)
	} else if *action == "unmount" {
		fmt.Printf("    - Mount:  %s\n", *mountPoint)
	}
	fmt.Println("=========================================================================")

	// =========================================================================
	// AUTHENTICATION
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(*username, *password, *verbose); err != nil {
		log.Fatalf("❌ Login failed: %v", err)
	}
	defer restClient.Logoff()

	// =========================================================================
	// EXECUTE NFS OPERATION
	// =========================================================================
	var output string
	var err error

	switch *action {
	case "mount":
		fmt.Println("\n📌 Mounting NFS export...")
		output, err = hmc.MountNFS(restClient, *sysName, *viosName, *nfsServer, *exportPath, *mountPoint, *nfsVersion, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to mount NFS: %v", err)
		}
		fmt.Println("\n✅ NFS mounted successfully!")

	case "unmount":
		fmt.Println("\n📌 Unmounting NFS...")
		output, err = hmc.UnmountNFS(restClient, *sysName, *viosName, *mountPoint, *verbose)
		if err != nil {
			log.Fatalf("❌ Failed to unmount NFS: %v", err)
		}
		fmt.Println("\n✅ NFS unmounted successfully!")

	default:
		log.Fatalf("❌ Invalid action: %s. Valid actions are: mount, unmount", *action)
	}

	// Display command output
	if output != "" {
		fmt.Println("─────────────────────────────────────────────────────────────────────────")
		fmt.Println(output)
		fmt.Println("─────────────────────────────────────────────────────────────────────────")
	}

	fmt.Println("\n✅ Operation completed successfully!")
	fmt.Println("=========================================================================")
}

// Made with Bob
