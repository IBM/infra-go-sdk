package main

import (
	"fmt"
	"log"

	"example.com/svc-demo/utils"
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
