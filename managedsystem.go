package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beevik/etree"
)

// GetManagedSystemQuick retrieves a surgical JSON summary of a single system.
func (c *HmcRestClient) GetManagedSystemQuick(systemUUID string, verbose bool) (*ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/quick", c.hmcIP, systemUUID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Ensure we get the raw JSON object

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC error %d: %s", resp.StatusCode, string(body))
	}

	var system ManagedSystemQuick
	if err := json.Unmarshal(body, &system); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exhaustive JSON: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Successfully captured all elements for: %s", system.SystemName)
	}

	return &system, nil
}

// GetManagedSystemByName fetches the managed system UUID and comprehensive details by its friendly name.
func (c *HmcRestClient) GetManagedSystemByName(systemName string, verbose bool) (string, *ManagedSystemDetailed, error) {
	if systemName == "" {
		return "", nil, fmt.Errorf("systemName cannot be empty")
	}

	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/search/(SystemName=='%s')", c.hmcIP, systemName)
	if verbose {
		hmcLogger.Printf("Fetching comprehensive managed system for name: %s, URL: %s", systemName, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	// Using atom+xml to ensure we get the proper feed/entry wrapper that the search endpoint returns
	req.Header.Set("Accept", "application/atom+xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetManagedSystemByName response status: %s", resp.Status)
	}

	if resp.StatusCode == 204 {
		if verbose {
			hmcLogger.Printf("No managed system found for name: %s", systemName)
		}
		return "", nil, nil // No content found
	}
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// 1. Strip the namespaces using your helper
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// 2. Extract the UUID from the AtomID field
	uuidElem := doc.FindElement("//AtomID")
	if uuidElem == nil {
		return "", nil, fmt.Errorf("AtomID not found in response")
	}
	uuid := uuidElem.Text()

	// 3. Extract ONLY the core ManagedSystem element
	msElem := doc.FindElement("//ManagedSystem")
	if msElem == nil {
		return "", nil, fmt.Errorf("ManagedSystem not found in response")
	}

	// 4. Serialize the isolated element back to bytes
	msDoc := etree.NewDocument()
	msDoc.SetRoot(msElem.Copy())
	msBytes, err := msDoc.WriteToBytes()
	if err != nil {
		return "", nil, fmt.Errorf("failed to serialize isolated ManagedSystem element: %v", err)
	}

	// 5. Unmarshal directly into our exhaustive Go struct
	var detailedSystem ManagedSystemDetailed
	if err := xml.Unmarshal(msBytes, &detailedSystem); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal XML into ManagedSystemDetailed struct: %v", err)
	}

	if verbose {
		hmcLogger.Printf("✅ Successfully resolved System '%s' to UUID: %s and parsed exhaustive configuration.", systemName, uuid)
	}

	return uuid, &detailedSystem, nil
}

// GetMaximumPartitions retrieves the MaximumPartitions for a system by UUID
func (c *HmcRestClient) GetMaximumPartitions(systemUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var system System
	if err := xml.Unmarshal(body, &system); err != nil {
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if system.MaxPartitions == "" {
		return "", fmt.Errorf("MaximumPartitions not found for system %s", systemUUID)
	}

	return system.MaxPartitions, nil
}

// GetManagedSystems retrieves the list of managed systems as an XML document
func (c *HmcRestClient) GetManagedSystems(verbose bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem", c.hmcIP)
	if verbose {
		hmcLogger.Printf("Fetching managed systems, URL: %s", url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=ManagedSystem")

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
		hmcLogger.Printf("GetManagedSystems response status: %s", resp.Status)
	}

	// Handle 204 No Content
	if resp.StatusCode == http.StatusNoContent {
		if verbose {
			hmcLogger.Printf("No managed systems found (204 No Content)")
		}
		return nil, nil
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetManagedSystems response body:\n%s", string(body))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	managedSystems := doc.FindElement("//ManagedSystem")
	if managedSystems == nil {
		return nil, fmt.Errorf("ManagedSystem element not found in response")
	}

	return managedSystems, nil
}

// GetManagedSystemsQuickAll fetches all systems using the high-performance JSON endpoint.
func (c *HmcRestClient) GetManagedSystemQuickAll(verbose bool) ([]ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/quick/All", c.hmcIP)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Request JSON explicitly

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HMC error (%s): %s", resp.Status, string(body))
	}

	var systems []ManagedSystemQuick
	if err := json.NewDecoder(resp.Body).Decode(&systems); err != nil {
		return nil, fmt.Errorf("failed to decode Quick/All JSON: %v", err)
	}

	return systems, nil
}

// GetManagedSystem retrieves the comprehensive, deeply parsed XML details of a Managed System.
func (c *HmcRestClient) GetManagedSystem(systemUUID string, verbose bool) (*ManagedSystemDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching comprehensive XML details for managed system UUID %s...", systemUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// 1. Strip the namespaces using the existing helper to make unmarshaling clean
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// 2. Extract ONLY the core ManagedSystem element (bypassing the <entry> atom wrapper)
	msElem := doc.FindElement("//ManagedSystem")
	if msElem == nil {
		return nil, fmt.Errorf("ManagedSystem root element not found in XML response")
	}

	// 3. Serialize the isolated element back to bytes
	msDoc := etree.NewDocument()
	msDoc.SetRoot(msElem.Copy())
	msBytes, err := msDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize isolated ManagedSystem element: %v", err)
	}

	// 4. Unmarshal directly into our comprehensive Go struct
	var detailedSystem ManagedSystemDetailed
	if err := xml.Unmarshal(msBytes, &detailedSystem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML into ManagedSystemDetailed struct: %v", err)
	}

	if verbose {
		hmcLogger.Printf("✅ Successfully parsed comprehensive details for System: %s", detailedSystem.SystemName)
	}

	return &detailedSystem, nil
}