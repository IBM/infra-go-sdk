package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
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

	viosName := flag.String("vios-name", "", "Name of the Virtual I/O Server to delete (Required)")
	force := flag.Bool("force", false, "Acknowledge this is a destructive action (Required for safety)")
	verbose := flag.Bool("verbose", false, "Enable verbose XML and HTTP output")

	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose


	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" || *viosName == "" {
		fmt.Println("Usage: deletevios -hmc-ip <ip> -hmc-user <user> -hmc-pass <pass> -system-name <sys> -vios-name <name> -force")
		log.Fatal("Missing required arguments.")
	}

	if !*force {
		log.Fatal("Safety Lock: You must provide the -force flag to confirm VIOS deletion.")
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	log.Printf("Logging into HMC: ip=%v", *hmcIP)
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)


	if err := restClient.Login(ctx, *username, *password); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer func() {
		log.Println("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	log.Printf("Resolving System: system=%v", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	// --- PRE-FLIGHT RESOLUTION & IDEMPOTENCY CHECK ---
	log.Println("Verifying if VIOS exists and checking power state...")
	viosList, err := restClient.GetVirtualIOServersQuick(ctx, sysUUID)
	if err != nil {
		log.Fatal("Failed to fetch VIOS inventory")
	}

	var targetViosUUID string
	var targetViosState string

	for _, vios := range viosList {
		if strings.EqualFold(vios.PartitionName, *viosName) {
			targetViosUUID = vios.UUID
			targetViosState = vios.PartitionState
			break
		}
	}

	// Idempotent Exit
	if targetViosUUID == "" {
		fmt.Println("\n=========================================================================")
		log.Printf("✅ Virtual I/O Server not found. No action needed.: vios_name=%v", *viosName)
		fmt.Println("=========================================================================")
		os.Exit(0)
	}

	// Power State Validation
	if !strings.EqualFold(targetViosState, "Not Activated") && !strings.EqualFold(targetViosState, "not activated") {
		log.Fatal("VIOS is currently running. You must power it off before deletion.")
	}

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		log.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE DELETION
	// =========================================================================
	log.Printf("Executing permanent Virtual I/O Server deletion...: vios=%v uuid=%v", *viosName, targetViosUUID)

	err = restClient.DeleteVirtualIOServer(ctx, sysUUID, targetViosUUID)
	if err != nil {
		if ctx.Err() != nil {
			log.Fatal("Operation aborted by user (Ctrl+C)")
		}
		log.Fatal("Failed to delete VIOS")
	}

	fmt.Println("\n=========================================================================")
	log.Printf("🗑️  SUCCESS: Virtual I/O Server Deleted!: vios_name=%v", *viosName)
	fmt.Println("=========================================================================")
}

