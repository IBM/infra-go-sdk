package utils

import (
	"log"
	"os"

	"github.com/IBM/infra-go-sdk/svc"
)

func GetSVCClient() *svc.Client {
	ip := os.Getenv("SVC_IP")
	username := os.Getenv("SVC_USERNAME")
	password := os.Getenv("SVC_PASSWORD")

	if ip == "" || username == "" || password == "" {
		log.Println("missing required environment variables: SVC_IP, SVC_USERNAME, SVC_PASSWORD")
		os.Exit(1)
	}

	return svc.NewClient(ip, username, password).WithTLSInsecure()
}
