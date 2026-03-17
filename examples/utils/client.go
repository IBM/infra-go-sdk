package utils

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sudeeshjohn/svc-go-sdk"
)

func GetSVCClient() *svc.Client {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ip := os.Getenv("SVC_IP")
	username := os.Getenv("SVC_USERNAME")
	password := os.Getenv("SVC_PASSWORD")
	return svc.NewClient(ip, username, password).WithTLSInsecure()
}
