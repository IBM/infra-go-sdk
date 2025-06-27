package hmc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
)

// LogonRequest represents the XML payload for HMC logon
type LogonRequest struct {
	XMLName       xml.Name `xml:"LogonRequest"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	XMLNS         string   `xml:"xmlns,attr"`
	XMLNSMC       string   `xml:"xmlns:mc,attr"`
	UserID        string   `xml:"UserID"`
	Password      string   `xml:"Password"`
}

// LogonResponse represents the XML response for HMC logon
type LogonResponse struct {
	XMLName xml.Name `xml:"LogonResponse"`
	Session string   `xml:"X-API-Session"`
}

// AtomFeed represents the Atom feed structure for PartitionTemplate
type AtomFeed struct {
	XMLName xml.Name         `xml:"http://www.w3.org/2005/Atom feed"`
	Entries []PartitionEntry `xml:"entry"`
}

// PartitionEntry represents a single PartitionTemplate entry in the feed
type PartitionEntry struct {
	XMLName           xml.Name          `xml:"entry"`
	ID                string            `xml:"id"`
	PartitionTemplate PartitionTemplate `xml:"content>PartitionTemplateSummary"`
}

// PartitionTemplate represents the PartitionTemplateSummary content
type PartitionTemplate struct {
	XMLName xml.Name `xml:"http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/ PartitionTemplateSummary"`
	AtomID  string   `xml:"Metadata>Atom>AtomID"`
	Name    string   `xml:"partitionTemplateName"`
	Content string   `xml:",innerxml"` // Capture full XML content for GetPartitionTemplate
}

// Logger with prefix for HMC operations
var hmcLogger = log.New(log.Writer(), "[HMC] ", log.LstdFlags)

// Logon performs the logon operation to the HMC REST API
func Logon(client *http.Client, hmcIP, username, password string, verbose bool) (string, error) {
	// Create XML payload
	payload := LogonRequest{
		SchemaVersion: "V1_0",
		XMLNS:         "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		XMLNSMC:       "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		UserID:        username,
		Password:      password,
	}
	xmlData, err := xml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("XML marshal failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Sending logon request to https://%s/rest/api/web/Logon", hmcIP)
		hmcLogger.Printf("Logon request payload:\n%s", string(xmlData))
	}

	// Create PUT request
	url := fmt.Sprintf("https://%s/rest/api/web/Logon", hmcIP)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(xmlData))
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.SetBasicAuth(username, password)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Logon response status: %s", resp.Status)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Logon response body:\n%s", string(body))
	}

	var logonResp LogonResponse
	if err := xml.Unmarshal(body, &logonResp); err != nil {
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	return logonResp.Session, nil
}

// Logoff performs the logoff operation from the HMC REST API
func Logoff(client *http.Client, hmcIP, session string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/web/Logon", hmcIP)
	if verbose {
		hmcLogger.Printf("Sending logoff request to %s", url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.Header.Set("Authorization", "Basic Og==")
	req.Header.Set("X-API-Session", session)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Logoff response status: %s", resp.Status)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("logoff failed with status: %s", resp.Status)
	}
	return nil
}

// GetPartitionTemplateID retrieves the AtomID for a partition template by name
func GetPartitionTemplateID(client *http.Client, hmcIP, session, name string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate?draft=false&detail=table", hmcIP)
	if verbose {
		hmcLogger.Printf("Requesting template ID for name: %s, URL: %s", name, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", session)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Raw response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Parsed %d entries from response", len(feed.Entries))
		for i, entry := range feed.Entries {
			hmcLogger.Printf("Entry %d: Name=%s, AtomID=%s", i+1, entry.PartitionTemplate.Name, entry.PartitionTemplate.AtomID)
		}
	}

	for _, entry := range feed.Entries {
		if entry.PartitionTemplate.Name == name {
			if entry.PartitionTemplate.AtomID == "" {
				return "", fmt.Errorf("no AtomID found for template name: %s", name)
			}
			if verbose {
				hmcLogger.Printf("Found template: Name=%s, AtomID=%s", name, entry.PartitionTemplate.AtomID)
			}
			return entry.PartitionTemplate.AtomID, nil
		}
	}

	return "", fmt.Errorf("template with name %s not found", name)
}

// GetPartitionTemplate retrieves the full PartitionTemplate XML by UUID or name
func GetPartitionTemplate(client *http.Client, hmcIP, session, uuid, name string, verbose bool) (string, error) {
	if uuid == "" && name != "" {
		var err error
		uuid, err = GetPartitionTemplateID(client, hmcIP, session, name, verbose)
		if err != nil {
			return "", fmt.Errorf("failed to get template UUID for name %s: %v", name, err)
		}
	}

	if uuid == "" {
		return "", fmt.Errorf("either uuid or name must be provided")
	}

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", hmcIP, uuid)
	if verbose {
		hmcLogger.Printf("Requesting template details for UUID: %s, URL: %s", uuid, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", session)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Response status for template details: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Raw response body for template details:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var template PartitionTemplate
	if err := xml.Unmarshal(body, &template); err != nil {
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Parsed template: AtomID=%s, Content:\n%s", template.AtomID, template.Content)
	}

	return template.Content, nil
}

// ListPartitionTemplateIDs retrieves all PartitionTemplate AtomIDs
func ListPartitionTemplateIDs(client *http.Client, hmcIP, session string, verbose bool) ([]string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate?draft=false&detail=table", hmcIP)
	if verbose {
		hmcLogger.Printf("Requesting partition template IDs, URL: %s", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", session)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Raw response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Parsed %d entries from response", len(feed.Entries))
		for i, entry := range feed.Entries {
			hmcLogger.Printf("Entry %d: Name=%s, AtomID=%s", i+1, entry.PartitionTemplate.Name, entry.PartitionTemplate.AtomID)
		}
	}

	ids := make([]string, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		if entry.PartitionTemplate.AtomID != "" {
			ids = append(ids, entry.PartitionTemplate.AtomID)
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no partition template IDs found")
	}

	if verbose {
		hmcLogger.Printf("Found %d template IDs: %v", len(ids), ids)
	}

	return ids, nil
}
