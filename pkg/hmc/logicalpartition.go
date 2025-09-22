package hmc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// GetLogicalPartition retrieves the details of a logical partition by name or UUID
func (c *HmcRestClient) GetLogicalPartition(systemUUID, partitionName, partitionUUID string, verbose bool) (string, *etree.Element, error) {
	var lparUUID string

	// If partitionUUID is not provided, find it using partitionName
	if partitionUUID == "" && partitionName != "" {
		lparList, err := c.GetLogicalPartitionsQuick(systemUUID, verbose)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch logical partitions: %v", err)
		}
		if lparList == nil {
			if verbose {
				hmcLogger.Printf("No logical partitions found for system UUID %s", systemUUID)
			}
			return "", nil, nil
		}

		for _, lpar := range lparList {
			if lpar.PartitionName == partitionName {
				lparUUID = lpar.UUID
				if verbose {
					hmcLogger.Printf("Found partition %s with UUID %s", partitionName, lparUUID)
				}
				break
			}
		}

		if lparUUID == "" {
			if verbose {
				hmcLogger.Printf("Partition %s not found on system UUID %s", partitionName, systemUUID)
			}
			return "", nil, nil
		}
	} else if partitionUUID != "" {
		lparUUID = partitionUUID
	} else {
		return "", nil, fmt.Errorf("either partitionName or partitionUUID must be provided")
	}

	// Fetch partition details
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching logical partition details for UUID %s, URL: %s", lparUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetLogicalPartition response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetLogicalPartition response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("Get of Logical Partition failed. Response code: %d", resp.StatusCode)
		}
		return "", nil, nil
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	partitionElem := doc.FindElement("//LogicalPartition")
	if partitionElem == nil {
		return "", nil, fmt.Errorf("LogicalPartition element not found in response")
	}

	return lparUUID, partitionElem, nil
}

// PowerOnPartition powers on a logical partition and returns the job response
func (c *HmcRestClient) PowerOnPartition(systemUUID, lparUUID, profileUUID, keylock, iIPLsource, osType string, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/do/PowerOn", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Powering on partition UUID %s on system UUID %s, URL: %s", lparUUID, systemUUID, url)
	}

	// Define operation details
	reqdOperation := map[string]string{
		"OperationName": "PowerOn",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Build job parameters
	jobParams := map[string]string{
		"force":    "false",
		"novsi":    "true",
		"bootmode": "norm",
	}

	if profileUUID != "" {
		jobParams["LogicalPartitionProfile"] = profileUUID
	}

	if keylock != "" {
		if keylock == "normal" {
			keylock = "norm"
		}
		jobParams["keylock"] = keylock
	}

	if osType == "OS400" && iIPLsource != "" {
		jobParams["iIPLsource"] = iIPLsource
	}

	// Create XML payload using createJobRequestPayload
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("PowerOn job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("PowerOnPartition response status: %s", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("PowerOnPartition response body:\n%s", string(respBody))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse XML response (assuming XML response despite Accept: application/json)
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract job ID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Monitor job status
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}

// PowerOffPartition powers off a logical partition and returns the job response
func (c *HmcRestClient) PowerOffPartition(systemUUID, lparUUID, shutdownOption string, restart bool, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/do/PowerOff", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Powering off partition UUID %s on system UUID %s, URL: %s", lparUUID, systemUUID, url)
	}

	// Define operation details
	reqdOperation := map[string]string{
		"OperationName": "PowerOff",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Determine immediate and operation based on shutdownOption
	var immediate, operation string
	switch shutdownOption {
	case "Delayed":
		immediate = "false"
		operation = "shutdown"
	case "Immediate":
		immediate = "true"
		operation = "shutdown"
	case "OperatingSystem":
		immediate = "false"
		operation = "osshutdown"
	case "OSImmediate":
		immediate = "true"
		operation = "osshutdown"
	case "Dump":
		immediate = "false"
		operation = "dumprestart"
		restart = false // Override restart as per Python logic
	case "DumpRetry":
		immediate = "false"
		operation = "retrydump"
		restart = false // Override restart as per Python logic
	default:
		return nil, fmt.Errorf("invalid shutdownOption: %s, must be one of Delayed, Immediate, OperatingSystem, OSImmediate, Dump, DumpRetry", shutdownOption)
	}

	// Build job parameters
	jobParams := map[string]string{
		"immediate": immediate,
		"operation": operation,
		"restart":   fmt.Sprintf("%t", restart),
	}

	// Create XML payload using createJobRequestPayload
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("PowerOff job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("PowerOffPartition response status: %s", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("PowerOffPartition response body:\n%s", string(respBody))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract job ID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Monitor job status
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}

// GetLogicalPartitions retrieves the list of logical partitions for a managed system as an XML document
func (c *HmcRestClient) GetLogicalPartitions(systemUUID string, verbose bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition?group=Advanced", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching logical partitions for system UUID %s, URL: %s", systemUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=LogicalPartition")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetLogicalPartitions response status: %s", resp.Status)
	}

	// Check for non-200 status codes

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		if verbose {
			hmcLogger.Printf("Get of Logical Partitions failed. Response code: %d", resp.StatusCode)
		}
		return nil, nil
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetLogicalPartitions response body:\n%s", string(body))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	logicalPartitions := doc.FindElement("//LogicalPartition")
	if logicalPartitions == nil {
		return nil, fmt.Errorf("LogicalPartition element not found in response")
	}

	return logicalPartitions, nil
}

// GetLogicalPartitionQuick retrieves quick details of a specific logical partition by UUID
func (c *HmcRestClient) GetLogicalPartitionQuick(partitionUUID string, verbose bool) (*LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, partitionUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick logical partition details for UUID %s, URL: %s", partitionUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetLogicalPartitionQuick response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetLogicalPartitionQuick response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("Get of Logical Partition failed. Response code: %d", resp.StatusCode)
		}
		return nil, nil
	}

	// Parse JSON response
	var partition LogicalPartitionQuick
	if err := json.Unmarshal(body, &partition); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Found logical partition: Name=%s, UUID=%s", partition.PartitionName, partition.UUID)
	}

	return &partition, nil
}

// QuickGetPartition retrieves quick properties of a partition as a map
func (c *HmcRestClient) QuickGetPartition(lparUUID string, verbose bool) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick partition properties for UUID %s, URL: %s", lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("QuickGetPartition response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("QuickGetPartition response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var partitionProps map[string]interface{}
	if err := json.Unmarshal(body, &partitionProps); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return partitionProps, nil
}

// GetLogicalPartitionsQuick retrieves the quick list of logical partitions for a system
func (c *HmcRestClient) GetLogicalPartitionsQuick(systemUUID string, verbose bool) ([]LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/quick/All", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick logical partitions for system UUID %s, URL: %s", systemUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionsQuick response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionsQuick response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil // No partitions found
		}
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	var lparList []LogicalPartitionQuick
	if err := json.Unmarshal(body, &lparList); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return lparList, nil
}

// GetPartitionProfile retrieves the UUID of a partition profile for a logical partition
func (c *HmcRestClient) GetPartitionProfile(lparUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/LogicalPartitionProfile/quick/All", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching partition profile for partition UUID %s, URL: %s", lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=LogicalPartitionProfile")
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionProfile response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != err {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetPartitionProfile response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var profiles []PartitionProfileQuick
	if err := json.Unmarshal(body, &profiles); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Check if any profiles were found
	if len(profiles) == 0 {
		return "", fmt.Errorf("no partition profiles found for partition UUID %s", lparUUID)
	}

	// Return the UUID of the first profile
	profileUUID := profiles[0].UUID
	if profileUUID == "" {
		return "", fmt.Errorf("profile UUID not found in response for partition UUID %s", lparUUID)
	}

	if verbose {
		hmcLogger.Printf("Found partition profile %s with UUID %s", profiles[0].ProfileName, profileUUID)
	}

	return profileUUID, nil
}

// LPAR_TEMPLATE_NS is the namespace for PartitionTemplate as used in the Python code
const LPAR_TEMPLATE_NS = `PartitionTemplate xmlns="http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2"`

// GetClientNetworkAdapter retrieves the ClientNetworkAdapter details for a partition
func (c *HmcRestClient) GetClientNetworkAdapter(systemUUID, lparUUID string, verbose bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching ClientNetworkAdapter for system UUID %s, partition UUID %s, URL: %s", systemUUID, lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml;type=ClientNetworkAdapter")
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetClientNetworkAdapter response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetClientNetworkAdapter response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	clientNetworkAdapter := doc.FindElement("//ClientNetworkAdapter")
	if clientNetworkAdapter == nil {
		return nil, fmt.Errorf("ClientNetworkAdapter element not found in response")
	}

	return clientNetworkAdapter, nil
}
