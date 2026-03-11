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

// DeployPartitionTemplate deploys a partition template to a managed system
func (c *HmcRestClient) DeployPartitionTemplate(draftUUID, cecUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/deploy", c.hmcIP, draftUUID)
	if verbose {
		hmcLogger.Printf("Deploying partition template with UUID %s to system UUID %s, URL: %s", draftUUID, cecUUID, url)
	}

	// Operation details for the job request
	reqdOperation := map[string]string{
		"OperationName": "Deploy",
		"GroupName":     "PartitionTemplate",
		"ProgressType":  "DISCRETE",
	}

	// Job parameters
	jobParams := map[string]string{
		"K_X_API_SESSION_MEMENTO": c.session,
		"TargetUuid":              cecUUID,
	}

	// Create the XML payload for the job request
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return "", fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("Deploy job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")
	// Enable basic auth to match Python's force_basic_auth=True
	req.SetBasicAuth("", "") // Credentials handled by session token
	if verbose {
		hmcLogger.Printf("Deploy request headers: %+v", req.Header)
	}
	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log the response status and body
	if verbose {
		hmcLogger.Printf("DeployPartitionTemplate response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	if verbose {
		hmcLogger.Printf("DeployPartitionTemplate response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Parse the response to check for specific error messages
		doc, err := xmlStripNamespace(body)
		if err != nil {
			return "", fmt.Errorf("failed to parse error response: %v, status: %s, body: %s", err, resp.Status, string(body))
		}
		errorMsgs := doc.FindElements("//Message")
		if len(errorMsgs) > 0 {
			return "", fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(body))
		}
		return "", fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Strip namespaces from the response XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// Check for error messages in the response
	errorMsgs := doc.FindElements("//Message")
	if len(errorMsgs) > 0 {
		return "", fmt.Errorf("error in response: %s", errorMsgs[0].Text())
	}

	// Extract the JobID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return "", fmt.Errorf("JobID not found in response: %s", string(body))
	}
	jobID := jobIDElem.Text()
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Fetch and return the job status
	depDoc, err := c.FetchJobStatus(jobID, true, 10, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	statusElem := depDoc.FindElement("//Status")
	statusE := statusElem.Text()
	if verbose {
		hmcLogger.Printf("Deploy job status: %s", statusE)
	}
	if verbose {
		log.Printf("Job status: %s", statusE)
	}
	//var partUUID string
	if statusE == "COMPLETED_OK" {
		stripNamespace(depDoc.Root())
		jobParams := depDoc.FindElements("//JobParameter")
		for _, param := range jobParams {
			// Look for ParameterName = PartitionUuid
			nameElem := param.FindElement("ParameterName")
			if nameElem != nil && strings.TrimSpace(nameElem.Text()) == "PartitionUuid" {
				partUUID := param.FindElement("ParameterValue")
				if partUUID != nil {
					fmt.Println("PartitionUuid:", partUUID.Text())
					fmt.Printf("Partition creation completed successfully UUID %s\n", partUUID.Text())
					return partUUID.Text(), nil
				}
			}
		}

	}
	if statusE == "FAILED" || statusE == "COMPLETED_WITH_ERROR" {
		log.Fatalf("Partition creation failed with status: %s", statusE)
	}

	return statusE, err
}

// GetPartitionTemplateID retrieves the AtomID for a partition template by name
func (c *HmcRestClient) GetPartitionTemplateID(name string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate?draft=false&detail=table", c.hmcIP)
	if verbose {
		hmcLogger.Printf("Requesting template ID for name: %s, URL: %s", name, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
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

	for _, entry := range feed.Entries {
		if entry.PartitionTemplate.Name == name {
			if entry.PartitionTemplate.AtomID == "" {
				return "", fmt.Errorf("no AtomID found for template name: %s", name)
			}
			return entry.PartitionTemplate.AtomID, nil
		}
	}

	return "", fmt.Errorf("template with name %s not found", name)
}

// ListPartitionTemplateIDs retrieves all PartitionTemplate AtomIDs
func (c *HmcRestClient) ListPartitionTemplateIDs(verbose bool) ([]string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate?draft=false&detail=table", c.hmcIP)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", c.session)

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
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("XML unmarshal failed: %v", err)
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

	return ids, nil
}

// GetPartitionTemplate retrieves the full PartitionTemplate XML by UUID or name
func (c *HmcRestClient) GetPartitionTemplate(uuid, name string, verbose bool) (*etree.Element, error) {
	if uuid == "" && name != "" {
		var err error
		uuid, err = c.GetPartitionTemplateID(name, verbose)
		if err != nil {
			return nil, fmt.Errorf("failed to get template UUID for name %s: %v", name, err)
		}
	}

	if uuid == "" {
		return nil, fmt.Errorf("no template found for name %s", name)
	}

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", c.hmcIP, uuid)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", c.session)

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
		return nil, fmt.Errorf("reading response failed: %v", err)
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
func (c *HmcRestClient) CopyPartitionTemplate(fromName, toName string, verbose bool) error {
	if verbose {
		hmcLogger.Printf("Copying template from %s to %s", fromName, toName)
	}

	templateDoc, err := c.GetPartitionTemplate("", fromName, verbose)
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

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate", c.hmcIP)
	req, err := http.NewRequest("PUT", url, strings.NewReader(xmlStr))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.templates+xml;type=PartitionTemplate")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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

// UpdatePartitionTemplate updates an existing partition template with the provided XML
func (c *HmcRestClient) UpdatePartitionTemplate(uuid string, templateXML *etree.Element, verbose bool) error {
	if uuid == "" {
		return fmt.Errorf("UUID cannot be empty")
	}

	// Convert the etree.Element to a string
	doc := etree.NewDocument()
	doc.SetRoot(templateXML)
	xmlStr, err := doc.WriteToString()
	if err != nil {
		return fmt.Errorf("failed to serialize XML: %v", err)
	}

	// Replace the PartitionTemplate tag with the namespace, mimicking the Python behavior
	xmlStr = strings.Replace(xmlStr, "<PartitionTemplate>", "<"+LPAR_TEMPLATE_NS+">", 1)

	if verbose {
		hmcLogger.Printf("Updating partition template XML for UUID %s:\n%s", uuid, xmlStr)
	}

	// Construct the URL
	templateURL := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", c.hmcIP, uuid)

	// Prepare the HTTP request
	req, err := http.NewRequest("POST", templateURL, strings.NewReader(xmlStr))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.templates+xml;type=PartitionTemplate")

	// Set context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Update partition template response status: %s", resp.Status)
		hmcLogger.Printf("Response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreatePartition creates a partition using a template UUID
func (c *HmcRestClient) CreatePartition(systemUUID, templateUUID, osType string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/do/CreatePartitionFromTemplate", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Creating partition for system %s with template UUID %s and osType %s", systemUUID, templateUUID, osType)
	}

	payload := JobRequest{
		SchemaVersion: "V1_0",
		Operation: Operation{
			OperationName: "CreatePartitionFromTemplate",
			GroupName:     "ManagedSystem",
			ProgressType:  "DISCRETE",
		},
		Parameters: []JobParameter{
			{Name: "K_X_API_SESSION_MEMENTO", Value: c.session},
			{Name: "TemplateUuid", Value: templateUUID},
			{Name: "OsType", Value: osType},
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

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Create partition response body:\n%s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("request failed with status: %d", resp.StatusCode)
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

// DeletePartitionTemplate deletes a partition template by name
func (c *HmcRestClient) DeletePartitionTemplate(templateName string, verbose bool) error {
	if verbose {
		hmcLogger.Printf("Deleting partition template: %s", templateName)
	}

	// Fetch the partition template to get the UUID
	templateDoc, err := c.GetPartitionTemplate("", templateName, verbose)
	if err != nil || templateDoc == nil {
		return fmt.Errorf("failed to fetch partition template %s: %v", templateName, err)
	}

	atomIDs := templateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 {
		return fmt.Errorf("AtomID not found for partition template %s", templateName)
	}
	templateUUID := atomIDs[0].Text()
	if verbose {
		hmcLogger.Printf("Found template UUID %s for template %s", templateUUID, templateName)
	}

	// Construct the DELETE URL
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", c.hmcIP, templateUUID)
	if verbose {
		hmcLogger.Printf("DELETE request URL: %s", url)
	}

	// Create and configure the DELETE request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("DeletePartitionTemplate response status: %s", resp.Status)
	}

	// Read the response body for error details
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	if verbose {
		hmcLogger.Printf("Successfully deleted partition template %s", templateName)
	}

	return nil
}

// TransformPartitionTemplate transforms a draft partition template for a managed system
func (c *HmcRestClient) TransformPartitionTemplate(draftUUID, cecUUID string, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/transform", c.hmcIP, draftUUID)
	if verbose {
		hmcLogger.Printf("Transforming partition template UUID %s for system UUID %s, URL: %s", draftUUID, cecUUID, url)
	}

	// Define operation details
	reqdOperation := map[string]string{
		"OperationName": "Transform",
		"GroupName":     "PartitionTemplate",
		"ProgressType":  "DISCRETE",
	}

	// Build job parameters
	jobParams := map[string]string{
		"K_X_API_SESSION_MEMENTO": c.session,
		"TargetUuid":              cecUUID,
	}

	// Create XML payload using createJobRequestPayload
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("Transform job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")

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
		hmcLogger.Printf("TransformPartitionTemplate response status: %s", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("TransformPartitionTemplate response body:\n%s", string(respBody))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Parse the response to check for specific error messages
		doc, err := xmlStripNamespace(respBody)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %v, status: %s, body: %s", err, resp.Status, string(respBody))
		}
		errorMsgs := doc.FindElements("//Message")
		if len(errorMsgs) > 0 {
			return nil, fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(respBody))
		}
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
	jobDoc, err := c.FetchJobStatus(jobID, true, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}

// CheckPartitionTemplate checks a partition template for a managed system
func (c *HmcRestClient) CheckPartitionTemplate(templateName, cecUUID string, verbose bool) (*etree.Document, error) {
	if verbose {
		hmcLogger.Printf("Checking partition template %s for system UUID %s", templateName, cecUUID)
	}

	// Fetch the partition template to get the UUID
	templateDoc, err := c.GetPartitionTemplate("", templateName, verbose)
	if err != nil || templateDoc == nil {
		return nil, fmt.Errorf("failed to fetch partition template %s: %v", templateName, err)
	}

	atomIDs := templateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 {
		return nil, fmt.Errorf("AtomID not found for partition template %s", templateName)
	}
	templateUUID := atomIDs[0].Text()
	if verbose {
		hmcLogger.Printf("Found template UUID %s for template %s", templateUUID, templateName)
	}

	// Construct the check URL
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/check", c.hmcIP, templateUUID)
	if verbose {
		hmcLogger.Printf("Check request URL: %s", url)
	}

	// Define operation details
	reqdOperation := map[string]string{
		"OperationName": "Check",
		"GroupName":     "PartitionTemplate",
		"ProgressType":  "DISCRETE",
	}

	// Build job parameters
	jobParams := map[string]string{
		"K_X_API_SESSION_MEMENTO": c.session,
		"TargetUuid":              cecUUID,
	}

	// Create XML payload using createJobRequestPayload
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("Check job request payload:\n%s", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")

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
		hmcLogger.Printf("CheckPartitionTemplate response status: %s", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("CheckPartitionTemplate response body:\n%s", string(respBody))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Parse the response to check for specific error messages
		doc, err := xmlStripNamespace(respBody)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %v, status: %s, body: %s", err, resp.Status, string(respBody))
		}
		errorMsgs := doc.FindElements("//Message")
		if len(errorMsgs) > 0 {
			return nil, fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(respBody))
		}
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
	jobDoc, err := c.FetchJobStatus(jobID, true, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	return jobDoc, nil
}
