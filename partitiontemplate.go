package hmc

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// DeployPartitionTemplate deploys a partition template to a managed system
func (c *HmcRestClient) DeployPartitionTemplate(draftUUID, cecUUID string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/deploy", c.hmcIP, draftUUID)
	if debug {
		c.Logger.Debug("Deploying partition template", "draftUUID", draftUUID, "cecUUID", cecUUID, "url", url)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", debug, true)
	if err != nil {
		return "", fmt.Errorf("failed to create job request payload: %v", err)
	}
	if debug {
		c.Logger.Debug("Deploy job request payload", "payload", payload)
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
	
	if debug {
		c.Logger.Debug("Deploy request headers", "headers", req.Header)
	}
	
	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log the response status and body
	if debug {
		c.Logger.Debug("DeployPartitionTemplate response status", "status", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	
	c.logRawTraffic("RESPONSE", url, string(body))
	
	if debug {
		c.Logger.Debug("DeployPartitionTemplate response body", "body", string(body))
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
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Fetch and return the job status
	jobResp, err := c.FetchJobStatus(context.Background(), jobID, true, 10, debug)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	if debug {
		c.Logger.Info("Deploy job status", "status", jobResp.Status)
	}

	if jobResp.Status == "COMPLETED_OK" {
		// Look for PartitionUuid in the Results parameters
		var partUUID string
		for _, param := range jobResp.Results.Parameters {
			if param.ParameterName == "PartitionUuid" {
				partUUID = param.ParameterValue
				break
			}
		}
		if partUUID != "" {
			if debug {
				c.Logger.Info("Partition creation completed successfully", "partUUID", partUUID)
			}
			return partUUID, nil
		}
	}
	
	if jobResp.Status == "FAILED" || jobResp.Status == "COMPLETED_WITH_ERROR" {
		c.Logger.Fatal("Partition creation failed", "status", jobResp.Status)
	}

	return jobResp.Status, err
}

// GetPartitionTemplateID retrieves the AtomID for a partition template by name
func (c *HmcRestClient) GetPartitionTemplateID(name string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate?draft=false&detail=table", c.hmcIP)
	if debug {
		c.Logger.Debug("Requesting template ID for name", "name", name, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if debug {
		c.Logger.Debug("Response status", "status", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("Raw response body", "body", string(body))
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
func (c *HmcRestClient) ListPartitionTemplateIDs(debug bool) ([]string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate?draft=false&detail=table", c.hmcIP)
	if debug {
		c.Logger.Debug("Listing Partition Template IDs", "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

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

	if debug {
		c.Logger.Info("Successfully retrieved partition template IDs", "count", len(ids))
	}

	return ids, nil
}

// GetPartitionTemplate retrieves the full PartitionTemplate XML by UUID or name
func (c *HmcRestClient) GetPartitionTemplate(uuid, name string, debug bool) (*etree.Element, error) {
	if uuid == "" && name != "" {
		var err error
		uuid, err = c.GetPartitionTemplateID(name, debug)
		if err != nil {
			return nil, fmt.Errorf("failed to get template UUID for name %s: %v", name, err)
		}
	}

	if uuid == "" {
		return nil, fmt.Errorf("no template found for name %s", name)
	}

	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", c.hmcIP, uuid)
	if debug {
		c.Logger.Debug("Fetching Partition Template", "uuid", uuid, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=PartitionTemplate")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

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
func (c *HmcRestClient) CopyPartitionTemplate(fromName, toName string, debug bool) error {
	if debug {
		c.Logger.Debug("Copying template", "fromName", fromName, "toName", toName)
	}

	templateDoc, err := c.GetPartitionTemplate("", fromName, debug)
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

	c.logRawTraffic("REQUEST (PUT)", url, xmlStr)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

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

	if debug {
		c.Logger.Info("Successfully copied partition template", "fromName", fromName, "toName", toName)
	}

	return nil
}

// UpdatePartitionTemplate updates an existing partition template with the provided XML
func (c *HmcRestClient) UpdatePartitionTemplate(uuid string, templateXML *etree.Element, debug bool) error {
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

	if debug {
		c.Logger.Debug("Updating partition template XML", "uuid", uuid, "xmlStr", xmlStr)
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

	c.logRawTraffic("REQUEST (POST)", templateURL, xmlStr)

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

	c.logRawTraffic("RESPONSE", templateURL, string(body))

	if debug {
		c.Logger.Debug("Update partition template response", "status", resp.Status, "body", string(body))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if debug {
			return fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("request failed with status: %d. Enable debug mode to see full response", resp.StatusCode)
	}

	return nil
}

// CreatePartition creates a partition using a template UUID
func (c *HmcRestClient) CreatePartition(systemUUID, templateUUID, osType string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/do/CreatePartitionFromTemplate", c.hmcIP, systemUUID)
	if debug {
		c.Logger.Debug("Creating partition from template", "systemUUID", systemUUID, "templateUUID", templateUUID, "osType", osType)
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

	c.logRawTraffic("REQUEST (PUT)", url, string(body))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if debug {
		c.Logger.Debug("Create partition response", "body", string(respBody))
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
func (c *HmcRestClient) DeletePartitionTemplate(templateName string, debug bool) error {
	if debug {
		c.Logger.Debug("Deleting partition template", "templateName", templateName)
	}

	// Fetch the partition template to get the UUID
	templateDoc, err := c.GetPartitionTemplate("", templateName, debug)
	if err != nil || templateDoc == nil {
		return fmt.Errorf("failed to fetch partition template %s: %v", templateName, err)
	}

	atomIDs := templateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 {
		return fmt.Errorf("AtomID not found for partition template %s", templateName)
	}
	templateUUID := atomIDs[0].Text()
	if debug {
		c.Logger.Debug("Found template UUID", "templateUUID", templateUUID, "templateName", templateName)
	}

	// Construct the DELETE URL
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s", c.hmcIP, templateUUID)
	if debug {
		c.Logger.Debug("DELETE request URL", "url", url)
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

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if debug {
		c.Logger.Debug("DeletePartitionTemplate response status", "status", resp.Status)
	}

	// Read the response body for error details
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	if debug {
		c.Logger.Info("Successfully deleted partition template", "templateName", templateName)
	}

	return nil
}

// TransformPartitionTemplate transforms a draft partition template for a managed system
// Returns a TransformResult struct with the transformation details
func (c *HmcRestClient) TransformPartitionTemplate(draftUUID, cecUUID string, debug bool) (*TransformResult, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/transform", c.hmcIP, draftUUID)
	if debug {
		c.Logger.Debug("Transforming partition template", "draftUUID", draftUUID, "cecUUID", cecUUID, "url", url)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", debug, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if debug {
		c.Logger.Debug("Transform job request payload", "payload", payload)
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

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if debug {
		c.Logger.Debug("TransformPartitionTemplate response status", "status", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if debug {
		c.Logger.Debug("TransformPartitionTemplate response body", "body", string(respBody))
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
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Monitor job status
	jobResp, err := c.FetchJobStatus(context.Background(), jobID, true, 10, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	// Build TransformResult from job response
	result := &TransformResult{
		JobID:   jobID,
		Status:  jobResp.Status,
		Success: jobResp.Status == "COMPLETED_OK",
	}

	// Extract transformed UUID from results if available
	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "TransformedUuid" {
			result.TransformedUUID = param.ParameterValue
			break
		}
	}

	if debug {
		c.Logger.Info("Transform result", "success", result.Success, "status", result.Status)
	}

	return result, nil
}

// CheckPartitionTemplate checks a partition template for a managed system
// Returns a TemplateValidationResult struct with validation details
func (c *HmcRestClient) CheckPartitionTemplate(templateName, cecUUID string, debug bool) (*TemplateValidationResult, error) {
	if debug {
		c.Logger.Debug("Checking partition template", "templateName", templateName, "cecUUID", cecUUID)
	}

	// Fetch the partition template to get the UUID
	templateDoc, err := c.GetPartitionTemplate("", templateName, debug)
	if err != nil || templateDoc == nil {
		return nil, fmt.Errorf("failed to fetch partition template %s: %v", templateName, err)
	}

	atomIDs := templateDoc.FindElements("//AtomID")
	if len(atomIDs) == 0 {
		return nil, fmt.Errorf("AtomID not found for partition template %s", templateName)
	}
	templateUUID := atomIDs[0].Text()
	if debug {
		c.Logger.Debug("Found template UUID", "templateUUID", templateUUID, "templateName", templateName)
	}

	// Construct the check URL
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/check", c.hmcIP, templateUUID)
	if debug {
		c.Logger.Debug("Check request URL", "url", url)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", debug, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if debug {
		c.Logger.Debug("Check job request payload", "payload", payload)
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

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if debug {
		c.Logger.Debug("CheckPartitionTemplate response status", "status", resp.Status)
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if debug {
		c.Logger.Debug("CheckPartitionTemplate response body", "body", string(respBody))
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
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Monitor job status
	jobResp, err := c.FetchJobStatus(context.Background(), jobID, true, 10, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %v", err)
	}

	// Build TemplateValidationResult from job response
	result := &TemplateValidationResult{
		JobID:        jobID,
		Status:       jobResp.Status,
		IsValid:      jobResp.Status == "COMPLETED_OK",
		Errors:       []string{},
		Warnings:     []string{},
	}

	// Parse validation results from job response
	if jobResp.Status == "COMPLETED_WITH_ERROR" || jobResp.Status == "FAILED" {
		result.IsValid = false
		// Add any error messages from Results parameters
		for _, param := range jobResp.Results.Parameters {
			if strings.Contains(strings.ToLower(param.ParameterName), "error") {
				errorMsg := fmt.Sprintf("%s: %s", param.ParameterName, param.ParameterValue)
				result.ErrorMessage = errorMsg
				result.Errors = append(result.Errors, errorMsg)
			}
		}
	}

	// Look for warnings in results
	for _, param := range jobResp.Results.Parameters {
		if strings.Contains(strings.ToLower(param.ParameterName), "warning") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", param.ParameterName, param.ParameterValue))
		}
	}

	if debug {
		c.Logger.Info("Template validation result", "isValid", result.IsValid, "status", result.Status, "errorCount", len(result.Errors), "warningCount", len(result.Warnings))
	}

	return result, nil
}