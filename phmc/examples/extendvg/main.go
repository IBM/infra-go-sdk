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
	
	viosName := flag.String("vios-name", "", "Target VIOS Name (Required)")
	vgName := flag.String("vg-name", "", "Name of the Volume Group to extend (Required)")
	pvs := flag.String("pvs", "", "Comma-separated list of Physical Volumes to add (e.g. hdisk5,hdisk6) (Required)")

	verbose := flag.Bool("verbose", false, "Enable verbose XML and HTTP output")
	debug     := flag.Bool("debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
	debugFull := flag.Bool("debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
	flag.Parse()
	_ = verbose


	if *hmcIP == "" || *username == "" || *password == "" || *sysName == "" || *viosName == "" || *vgName == "" || *pvs == "" {
		fmt.Println("Usage: extendvg -hmc-ip <ip> -hmc-user <user> -hmc-pass <pass> -system-name <sys> -vios-name <vios> -vg-name <vg> -pvs <hdiskX,hdiskY>")
		log.Fatal("Missing required arguments.")
	}

	// =========================================================================
	// 3. AUTHENTICATION & RESOLUTION
	// =========================================================================
	log.Printf("Logging into HMC: ip=%v", *hmcIP)
	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)


	if err := restClient.Login(ctx, *username, *password); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer func() {
		log.Println("Closing HMC Session...")
		restClient.Logoff(ctx)
	}()

	log.Printf("Resolving System: system=%v", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	log.Printf("Resolving VIOS: vios=%v", *viosName)
	viosUUID, err := hmc.GetViosID(ctx, restClient, sysUUID, *viosName)
	if err != nil || viosUUID == "" {
		log.Fatal("VIOS not found on system")
	}

	// Split and clean the requested physical volume list
	pvList := strings.Split(*pvs, ",")

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		log.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE EXTENSION
	// =========================================================================
	log.Printf("Initiating Volume Group Extension: vios=%v vg=%v targets=%v", *viosName, *vgName, len(pvList))

	err = restClient.ExtendVolumeGroup(ctx, *sysName, viosUUID, *viosName, *vgName, pvList)
	if err != nil {
		if ctx.Err() != nil {
			log.Fatal("Operation aborted by user (Ctrl+C)")
		}
		log.Fatal("Failed to extend Volume Group")
	}

	fmt.Println("\n=========================================================================")
	log.Printf("SUCCESS: Volume Group Extended!: vg=%v added_pvs=%v", *vgName, pvList)
	fmt.Println("=========================================================================")
}

// Made with Bob
