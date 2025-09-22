package hmc

import "fmt"

// GetViosID retrieves the UUID of a Virtual I/O Server by its name using the provided rest client
func GetViosID(restClient *HmcRestClient, systemUUID, viosName string, verbose bool) (string, error) {
	viosList, err := restClient.GetVirtualIOServersQuick(systemUUID, verbose)
	fmt.Printf("VIOS List: %s\n", viosList)
	if err != nil {
		return "", fmt.Errorf("failed to get VIOSes: %v", err)
	}

	for _, vios := range viosList {
		if vios.PartitionName == viosName {
			return vios.UUID, nil
		}
	}

	return "", fmt.Errorf("VIOS %s not found", viosName)
}
