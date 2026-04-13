package main

import (
	"flag"
	"os"
	"strings"

	"github.com/sudeeshjohn/svc-go-sdk"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	svcIP := flag.String("svc-ip", "REDACTED_SVC_IP<==", "SVC IP address")
	svcUser := flag.String("svc-user", "REDACTED_SVC_USER<==", "SVC username")
	svcPass := flag.String("svc-pass", "REDACTED_SVC_PASS<==", "SVC password")
	flag.Parse()

	client := svc.NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()
	if *verbose {
		client = client.WithDebug()
	}

	if err := client.Authenticate(); err != nil {
		client.Logger.Error("Authentication error", "error", err)
		os.Exit(1)
	}

	hostName := "ltc09u31-vios1"
	volName := "test_volume3" 

	client.Logger.Info("Attempting to unmap volume from host...", "volume", volName, "host", hostName)

	err := client.Rmvdiskhostmap(hostName, volName)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "CMMVC6071E") {
			client.Logger.Info("✅ Volume is already unmapped from this host. Nothing to do.")
		} else if strings.Contains(errStr, "CMMVC5754E") {
			client.Logger.Warn("Host or volume doesn't exist", "volume", volName, "host", hostName)
		} else {
			client.Logger.Error("Failed to unmap", "error", err)
			os.Exit(1)
		}
	} else {
		client.Logger.Info("✅ Successfully unmapped volume", "volume", volName, "host", hostName)
	}
}