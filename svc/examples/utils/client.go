package utils

import (
	"os"

	"github.com/IBM/infra-go-sdk/svc"
)

func GetSVCClient() *svc.Client {
	ip := os.Getenv("SVC_IP")
	username := os.Getenv("SVC_USERNAME")
	password := os.Getenv("SVC_PASSWORD")

	if ip == "" || username == "" || password == "" {
		logger := svc.NewDefaultLogger()
		logger.Error("missing required environment variables: SVC_IP, SVC_USERNAME, SVC_PASSWORD")
		os.Exit(1)
	}

	client := svc.NewClient(ip, username, password).WithTLSInsecure()
	
	// Automatically turn on debug if requested via env var
	if os.Getenv("SVC_DEBUG") == "true" {
		client = client.WithDebug()
	}

	return client
}