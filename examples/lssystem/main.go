package main

import (
	"fmt"
	"log"

	"github.com/sudeeshjohn/svc-go-sdk/examples/utils"
	// Adjust if your package path differs
)

func main() {

	client := utils.GetSVCClient()

	if err := client.Authenticate(); err != nil {
		log.Fatalf("auth error: %v", err)
	}
	fmt.Println("✅ Authenticated")

	systemInfo, err := client.Lssystem()
	if err != nil {
		log.Fatalf("lssystem error: %v", err)
	}
	fmt.Printf("System: %+v\n", systemInfo)
}
