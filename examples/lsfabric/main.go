package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/mkumatag/svc-go-sdk"
)

func main() {
	client := svc.NewClient("REDACTED_SVC_IP<==", "REDACTED_SVC_USER<==", "REDACTED_SVC_PASS<==").WithTLSInsecure()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	// List all fabric logins
	logins, err := client.Lsfabric()
	if err != nil {
		log.Fatalf("Lsfabric error: %v", err)
	}

	if len(logins) == 0 {
		fmt.Println("No fabric logins found")
	} /*else {
		fmt.Println("All Fabric Logins:")
		for _, login := range logins {
			fmt.Printf("Remote WWPN: %s, Host Name: %s, Local WWPN: %s, Status: %s\n", login.RemoteWWPN, login.HostName, login.LocalWWPN, login.State)

	}*/
	// Define the host parameters
	host := svc.Host{
		Name:     "host1",
		Fcwwpn:   []string{"100000620B42EB0A", "100000620B42EB09"},
		Type:     "generic",
		Protocol: "scsi",
	}
	// Check if any WWPN is already associated with a host
	for _, wwpn := range host.Fcwwpn {
		existingHost, err := client.GetHostByWWPN(wwpn)
		if err == nil {
			fmt.Printf("WWPN %s is already associated with host: %s. Skipping creation.\n", wwpn, existingHost)
			return // Exit with success if already exists
		} else if !strings.Contains(err.Error(), "not found") {
			log.Fatalf("GetHostByWWPN error: %v", err)
		}
	}
}
