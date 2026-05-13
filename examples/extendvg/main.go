package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	hmc "github.com/sudeeshjohn/powerhmc-go"
)

func main() {
	// =========================================================================
	// 1. SET UP CONTEXT & GRACEFUL CANCELLATION (Ctrl+C)
	// =========================================================================
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

	// =========================================================================
	// 2. FLAGS & VALIDATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name (Required)")
	
	viosName := flag.String("vios-name", "", "Target VIOS Name (Required)")
	vgName := flag.String("vg-name", "", "Name of the Volume Group to extend (Required)")
	pvs := flag.String("pvs", "", "Comma-separated list of Physical Volumes to add (e.g. hdisk5,hdisk6) (Required)")

	verbose := flag.Bool("verbose", false, "Enable verbose XML and HTTP output")
	flag.Parse()

	if *verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // InfoLevel
	}

	if *password == "" || *sysName == "" || *viosName == "" || *vgName == "" || *pvs == "" {
		fmt.Println("Usage: extendvg -system-name <sys> -vios-name <vios> -vg-name <vg> -pvs <hdiskX,hdiskY>")
		cliLogger.Fatal("Missing required arguments.")
	}

	// =========================================================================
	// 3. AUTHENTICATION & RESOLUTION
	// =========================================================================
	cliLogger.Info("Logging into HMC", "ip", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)

	if *verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(ctx, *username, *password, *verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer func() {
		cliLogger.Info("Closing HMC Session...")
		restClient.Logoff(ctx)
	}()

	cliLogger.Debug("Resolving System", "system", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", *sysName, "error", err)
	}

	cliLogger.Debug("Resolving VIOS", "vios", *viosName)
	viosUUID, err := hmc.GetViosID(ctx, restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		cliLogger.Fatal("VIOS not found on system", "vios", *viosName, "system", *sysName)
	}

	// Split and clean the requested physical volume list
	pvList := strings.Split(*pvs, ",")

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE EXTENSION
	// =========================================================================
	cliLogger.Info("Initiating Volume Group Extension", "vios", *viosName, "vg", *vgName, "targets", len(pvList))

	err = restClient.ExtendVolumeGroup(ctx, *sysName, viosUUID, *viosName, *vgName, pvList, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
		}
		cliLogger.Fatal("Failed to extend Volume Group", "error", err)
	}

	fmt.Println("\n=========================================================================")
	cliLogger.Info("SUCCESS: Volume Group Extended!", "vg", *vgName, "added_pvs", pvList)
	fmt.Println("=========================================================================")
}

// Made with Bob
