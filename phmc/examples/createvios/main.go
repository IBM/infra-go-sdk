package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	hmc "github.com/IBM/infra-go-sdk/phmc"
)

func main() {
	// =========================================================================
	// 1. SET UP CONTEXT & GRACEFUL CANCELLATION (Ctrl+C)
	// =========================================================================
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()


	// =========================================================================
	// 2. FLAGS & VALIDATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address (Required)")
	username := flag.String("hmc-user", "", "HMC username (Required)")
	password := flag.String("hmc-pass", "", "HMC password (Required)")
	sysName := flag.String("system-name", "", "Managed System Name (Required)")
	
	viosName := flag.String("vios-name", "", "Name of the new Virtual I/O Server (Required)")
	dedicatedProc := flag.Bool("dedicated-proc", false, "Use dedicated processors instead of shared")
	
	// Typical VIOS Sizing Defaults
	desMem := flag.Int("mem", 8192, "Desired Memory in MB (Default: 8192)")
	desCpus := flag.Float64("cpus", 1.0, "Desired Processing Units (Default: 1.0)")
	desVcpus := flag.Int("vcpus", 2, "Desired Virtual Processors (Default: 2)")

	maxVirtualSlots := flag.Int("max-slots", 500, "Maximum Virtual I/O Slots")
	sharingMode := flag.String("sharing-mode", "uncapped", "Sharing Mode: capped, uncapped, share idle procs")
	uncappedWeight := flag.Int("weight", 128, "Uncapped Weight (0-255)")

	verbose := flag.Bool("verbose", false, "Enable verbose XML and HTTP output")
	flag.Parse()
	_ = verbose


	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" || *viosName == "" {
		fmt.Println("Usage: createvios -hmc-ip <ip> -hmc-user <user> -hmc-pass <pass> -system-name <sys> -vios-name <name>")
		log.Fatal("Missing required arguments.")
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	log.Printf("Logging into HMC: ip=%v", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)


	if err := restClient.Login(ctx, *username, *password, *verbose); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer func() {
		log.Println("Closing HMC Session...")
		restClient.Logoff(ctx)
	}()

	log.Printf("Resolving System: system=%v", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	// --- PRE-FLIGHT EXISTENCE CHECK ---
	log.Println("Verifying if VIOS already exists...")
	viosList, err := restClient.GetVirtualIOServersQuick(ctx, sysUUID, *verbose)
	if err == nil {
		for _, vios := range viosList {
			if strings.EqualFold(vios.PartitionName, *viosName) {
				fmt.Println("\n=========================================================================")
				log.Printf("✅ Virtual I/O Server already exists. Skipping creation.: vios_name=%v uuid=%v", *viosName, vios.UUID)
				fmt.Println("=========================================================================")
				os.Exit(0) // Idempotent Exit
			}
		}
	}

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		log.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. BUILD REQUEST & EXECUTE
	// =========================================================================
	req := hmc.CreateViosRequest{
		Name:             *viosName,
		MinMem:           4096,          // 4GB Minimum Base
		DesiredMem:       *desMem,
		MaxMem:           *desMem * 4,   // 4x Scale
		MinProcUnits:     0.1,
		DesiredProcUnits: *desCpus,
		MaxProcUnits:     *desCpus * 4,
		MinVcpus:         1,
		DesiredVcpus:     *desVcpus,
		MaxVcpus:         *desVcpus * 4,
		SharingMode:      *sharingMode,
		UncappedWeight:   *uncappedWeight,
		MaxVirtualSlots:  *maxVirtualSlots,
		DedicatedProc:    *dedicatedProc,
	}

	log.Printf("Provisioning Virtual I/O Server...: vios=%v mem_mb=%v cpus=%v", *viosName, req.DesiredMem, req.DesiredProcUnits)

	newUUID, err := restClient.CreateVirtualIOServer(ctx, sysUUID, req, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			log.Fatal("Operation aborted by user (Ctrl+C)")
		}
		log.Fatal("Failed to provision VIOS")
	}

	fmt.Println("\n=========================================================================")
	log.Printf("SUCCESS: Virtual I/O Server Created!: vios_name=%v uuid=%v", *viosName, newUUID)
	fmt.Println("=========================================================================")
}

// Made with Bob
