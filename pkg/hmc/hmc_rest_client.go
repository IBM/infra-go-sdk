package hmc

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
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
	Content string   `xml:",innerxml"` // Capture full XML content
}

// System represents the ManagedSystem content
type System struct {
	XMLName       xml.Name `xml:"http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/ ManagedSystem"`
	MaxPartitions string   `xml:"MaximumPartitions"`
	SystemName    string   `xml:"SystemName"`
	SerialNumber  string   `xml:"MachineTypeModelAndSerialNumber>SerialNumber"`
}

// Logger with prefix for HMC operations
var hmcLogger = log.New(log.Writer(), "[HMC] ", log.LstdFlags)

// Logon performs the logon operation to the HMC REST API
func Logon(client *http.Client, hmcIP, username, password string, verbose bool) (string, error) {
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

	url := fmt.Sprintf("https://%s/rest/api/web/Logon", hmcIP)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(xmlData))
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.SetBasicAuth(username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Logon response status: %s", resp.Status)
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

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

// GetPartitionTemplate retrieves the full PartitionTemplate XML by UUID or name
func GetPartitionTemplate(client *http.Client, hmcIP, session, uuid, name string, verbose bool) (*etree.Element, error) {
	if uuid == "" && name != "" {
		var err error
		uuid, err = GetPartitionTemplateID(client, hmcIP, session, name, verbose)
		if err != nil {
			return nil, fmt.Errorf("failed to get template UUID for name %s: %v", name, err)
		}
	}

	if uuid == "" {
		return nil, fmt.Errorf("no template found for name %s", name)
	}

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", hmcIP, uuid)
	if verbose {
		hmcLogger.Printf("Requesting template details for UUID: %s, URL: %s", uuid, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Response status for template details: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Raw response body for template details:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("no root element in XML")
	}

	elements := root.FindElements("//PartitionTemplate")
	if len(elements) == 0 {
		return nil, fmt.Errorf("no PartitionTemplate found in response")
	}

	return elements[0], nil
}

// CopyPartitionTemplate copies a partition template from one name to another
func CopyPartitionTemplate(client *http.Client, hmcIP, session, fromName, toName string, verbose bool) error {
	if verbose {
		hmcLogger.Printf("Copying template from %s to %s", fromName, toName)
	}

	templateDoc, err := GetPartitionTemplate(client, hmcIP, session, "", fromName, verbose)
	if err != nil || templateDoc == nil {
		return fmt.Errorf("failed to fetch source template %s: %v", fromName, err)
	}

	nameElements := templateDoc.FindElements("//partitionTemplateName")
	if len(nameElements) == 0 {
		return fmt.Errorf("partitionTemplateName not found in template")
	}
	nameElements[0].SetText(toName)

	templateNamespace := `PartitionTemplate xmlns="http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2"`
	doc := etree.NewDocument()
	doc.SetRoot(templateDoc)
	xmlStr, err := doc.WriteToString()
	if err != nil {
		return fmt.Errorf("failed to serialize XML: %v", err)
	}
	xmlStr = strings.Replace(xmlStr, "<PartitionTemplate>", "<"+templateNamespace+">", 1)

	if verbose {
		hmcLogger.Printf("Template XML:\n%s", xmlStr)
	}

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate", hmcIP)
	req, err := http.NewRequest("PUT", url, strings.NewReader(xmlStr))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.templates+xml;type=PartitionTemplate")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Copy response body:\n%s", string(body))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	doc = etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return fmt.Errorf("failed to parse response XML: %v", err)
	}
	root := doc.Root()
	if root == nil {
		return fmt.Errorf("no root element in response XML")
	}

	errorMsgs := root.FindElements("//Message")
	if len(errorMsgs) > 0 {
		return fmt.Errorf("error in response: %s", errorMsgs[0].Text())
	}

	return nil
}

// DeletePartitionTemplate deletes a partition template by name
func DeletePartitionTemplate(client *http.Client, hmcIP, session, templateName string, verbose bool) error {
	if verbose {
		hmcLogger.Printf("Deleting partition template: %s", templateName)
	}

	templateDoc, err := GetPartitionTemplate(client, hmcIP, session, "", templateName, verbose)
	if err != nil || templateDoc == nil {
		return fmt.Errorf("failed to fetch partition template %s: %v", templateName, err)
	}

	atomIDs := templateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 {
		return fmt.Errorf("AtomID not found in template")
	}
	templateUUID := atomIDs[0].Text()

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", hmcIP, templateUUID)
	if verbose {
		hmcLogger.Printf("Delete URL: %s", url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status: %s", resp.Status)
	}

	return nil
}

// CheckPartitionTemplate checks a partition template against a CEC UUID
func CheckPartitionTemplate(client *http.Client, hmcIP, session, templateName, cecUUID string, verbose bool) (string, error) {
	templateDoc, err := GetPartitionTemplate(client, hmcIP, session, "", templateName, verbose)
	if err != nil || templateDoc == nil {
		return "", fmt.Errorf("failed to fetch partition template %s: %v", templateName, err)
	}

	atomIDs := templateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 {
		return "", fmt.Errorf("AtomID not found in template")
	}
	templateUUID := atomIDs[0].Text()

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/check", hmcIP, templateUUID)
	if verbose {
		hmcLogger.Printf("Checking template %s with CEC UUID %s, URL: %s", templateName, cecUUID, url)
	}

	type Operation struct {
		XMLName       xml.Name `xml:"Operation"`
		OperationName string   `xml:"OperationName"`
		GroupName     string   `xml:"GroupName"`
		ProgressType  string   `xml:"ProgressType"`
	}

	type JobParameter struct {
		XMLName xml.Name `xml:"JobParameter"`
		Name    string   `xml:"name"`
		Value   string   `xml:"value"`
	}

	type JobRequest struct {
		XMLName       xml.Name       `xml:"JobRequest"`
		SchemaVersion string         `xml:"schemaVersion,attr"`
		Operation     Operation      `xml:"RequestedOperation>Operation"`
		Parameters    []JobParameter `xml:"JobParameters>JobParameter"`
	}

	payload := JobRequest{
		SchemaVersion: "V1_0",
		Operation: Operation{
			OperationName: "Check",
			GroupName:     "PartitionTemplate",
			ProgressType:  "DISCRETE",
		},
		Parameters: []JobParameter{
			{Name: "K_X_API_SESSION_MEMENTO", Value: session},
			{Name: "TargetUuid", Value: cecUUID},
		},
	}

	body, err := xml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job request payload: %v", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Check response body:\n%s", string(respBody))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(respBody); err != nil {
		return "", fmt.Errorf("failed to parse response XML: %v", err)
	}
	root := doc.Root()
	if root == nil {
		return "", fmt.Errorf("no root element in response XML")
	}

	jobIDs := root.FindElements("//JobID")
	if len(jobIDs) == 0 {
		return "", fmt.Errorf("JobID not found in response")
	}

	return jobIDs[0].Text(), nil
}

// FetchJobStatus retrieves the status of a job by its ID (Placeholder)
func FetchJobStatus(client *http.Client, hmcIP, session, jobID string, template bool, verbose bool) (string, error) {
	return "", fmt.Errorf("FetchJobStatus not implemented")
}

// GetMaximumPartitions retrieves the MaximumPartitions for a system by UUID
func GetMaximumPartitions(client *http.Client, hmcIP, session, systemUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Requesting system details for UUID: %s, URL: %s", systemUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")
	req.Header.Set("X-API-Session", session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Response status for system details: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Raw response body for system details:\n%s", string(body))
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

	if verbose {
		hmcLogger.Printf("MaximumPartitions: %s", system.MaxPartitions)
	}

	return system.MaxPartitions, nil
}
