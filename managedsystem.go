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

// GetManagedSystemQuick retrieves quick details of a specific managed system by UUID
func (c *HmcRestClient) GetManagedSystemQuick(systemUUID string, verbose bool) (*ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/quick", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick managed system details for UUID %s, URL: %s", systemUUID, url)
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
		hmcLogger.Printf("GetManagedSystemQuick response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetManagedSystemQuick response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("Get of Managed System failed. Response code: %d", resp.StatusCode)
		}
		return nil, nil
	}

	// Parse JSON response
	var system ManagedSystemQuick
	if err := json.Unmarshal(body, &system); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Found managed system: Name=%s, UUID=%s", system.SystemName, system.UUID)
	}

	return &system, nil
}

// GetManagedSystem fetches the managed system UUID and details by name
func (c *HmcRestClient) GetManagedSystemByName(systemName string, verbose bool) (string, *etree.Element, error) {
	if systemName == "" {
		return "", nil, fmt.Errorf("systemName cannot be empty")
	}
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/search/(SystemName=='%s')", c.hmcIP, systemName)
	if verbose {
		hmcLogger.Printf("Fetching managed system for name: %s, URL: %s", systemName, url)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetManagedSystem response status: %s", resp.Status)
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

	if verbose {
		hmcLogger.Printf("GetManagedSystem response body:\n%s", string(body))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(string(body)); err != nil {
		return "", nil, fmt.Errorf("failed to parse XML response: %v", err)
	}

	uuidElem := doc.FindElement("//AtomID")
	if uuidElem == nil {
		return "", nil, fmt.Errorf("AtomID not found in response")
	}
	uuid := uuidElem.Text()

	msElem := doc.FindElement("//ManagedSystem")
	if msElem == nil {
		return "", nil, fmt.Errorf("ManagedSystem not found in response")
	}

	return uuid, msElem, nil
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

// GetManagedSystemsQuick retrieves the quick list of managed systems as a JSON-parsed slice
func (c *HmcRestClient) GetManagedSystemsQuick(verbose bool) ([]ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/quick/All", c.hmcIP)
	if verbose {
		hmcLogger.Printf("Fetching quick managed systems, URL: %s", url)
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
		hmcLogger.Printf("GetManagedSystemsQuick response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetManagedSystemsQuick response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("Get of Managed Systems failed. Response code: %d", resp.StatusCode)
		}
		return nil, nil
	}

	// Parse JSON response
	var systems []ManagedSystemQuick
	if err := json.Unmarshal(body, &systems); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return systems, nil
}

// GetIOAdapters retrieves all physical IO adapters for a managed system.
func (c *HmcRestClient) GetManagedSystemInfo(systemUUID string, verbose bool) ([]*etree.Element, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)
    if verbose {
        hmcLogger.Printf("Fetching IO adapters for managed system UUID %s, URL: %s", systemUUID, url)
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

    if verbose {
        hmcLogger.Printf("GetIOAdapters response status: %s", resp.Status)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    if verbose {
        hmcLogger.Printf("GetIOAdapters response body:\n%s", string(body))
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
    }

    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    //adapters := doc.FindElements("//ManagedSystem")
	 adapters := doc.FindElements("//IOAdapters/IOAdapterChoice")
    if verbose {
        hmcLogger.Printf("Found %d IO adapters for managed system %s", len(adapters), systemUUID)
    }

    return adapters, nil
}

