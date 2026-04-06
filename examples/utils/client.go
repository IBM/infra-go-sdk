package utils

import (
	"log"
	"os"

	"github.com/sudeeshjohn/svc-go-sdk"
)

func GetSVCClient() *svc.Client {
	ip := os.Getenv("SVC_IP")
	username := os.Getenv("SVC_USERNAME")
	password := os.Getenv("SVC_PASSWORD")

	if ip == "" || username == "" || password == "" {
		log.Fatal("missing required environment variables: SVC_IP, SVC_USERNAME, SVC_PASSWORD")
	}

	return svc.NewClient(ip, username, password).WithTLSInsecure()
}
