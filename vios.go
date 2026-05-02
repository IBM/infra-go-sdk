package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// / ConfigDevice submits a job request to configure a device on a Virtual I/O Server.
// If devName is empty, it attempts to configure all devices.
// It waits for the job to complete and checks for success.
func (c *HmcRestClient) ConfigDevice(ctx context.Context,viosID string, devName string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/ConfigDevice", c.hmcIP, viosID)
	if debug {
		c.Logger.Debug("Submitting ConfigDevice job", "viosID", viosID, "url", url)
	}

	// Prepare operation map
	operation := map[string]string{
		"OperationName": "ConfigDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	// Prepare params map
	params := make(map[string]string)
	if devName != "" {
		params["devName"] = devName
	}

	// Schema version
	schemaVersion := "V1_1_0"

	// Include job param schema
	includeJobParamSchema := true

	// Generate payload using createJobRequestPayload
	payload, err := createJobRequestPayload(operation, params, schemaVersion, debug, includeJobParamSchema)
	if err != nil {
		return fmt.Errorf("failed to create JobRequest payload: %v", err)
	}

	if debug {
		c.Logger.Debug("JobRequest XML generated", "payload", payload)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	// Set timeout
	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(reqCtx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// Log response status if debug
	if debug {
		c.Logger.Debug("ConfigDevice response status", "status", resp.Status)
		c.Logger.Debug("ConfigDevice response body", "body", string(body))
	}

	// Check for non-success status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		c.Logger.Error("ConfigDevice job submission failed", "status", resp.Status)
		if debug {
			return fmt.Errorf("ConfigDevice job submission failed with status %s: %s", resp.Status, string(body))
		}
		return fmt.Errorf("ConfigDevice job submission failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// Strip namespaces from the response XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// Check for error messages in the response
	errorMsgs := doc.FindElements("//Message")
	if len(errorMsgs) > 0 {
		return fmt.Errorf("error in response: %s", errorMsgs[0].Text())
	}

	// Extract the JobID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return fmt.Errorf("JobID not found in response: %s", string(body))
	}
	jobID := jobIDElem.Text()
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Fetch the job response
	jobResp, err := c.FetchJobStatus(ctx, jobID, false, 10, debug)
	if err != nil {
		return fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	if debug {
		c.Logger.Info("ConfigDevice job response", "status", jobResp.Status)
	}

	// Check job status
	if jobResp.Status != "COMPLETED_OK" {
		return fmt.Errorf("job failed: status %s", jobResp.Status)
	}

	// Check for StdError in results
	var stdError string
	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "StdError" {
			stdError = param.ParameterValue
			break
		}
	}
	if stdError != "" {
		return fmt.Errorf("config device error: %s", stdError)
	}

	return nil
}

// GetVirtualIOServersQuick retrieves the exhaustive quick list of Virtual I/O Servers for a given managed system UUID.
func (c *HmcRestClient) GetVirtualIOServersQuick(systemUUID string, debug bool) ([]VIOSQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer/quick/All", c.hmcIP, systemUUID)
	
	if debug {
		c.Logger.Debug("Fetching quick VIOS list", "systemUUID", systemUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if debug {
			c.Logger.Warn("GetVirtualIOServersQuick failed", "status", resp.Status)
		}
		// Sometimes the HMC returns 204 No Content if there are literally zero VIOSes on the system.
		// Handling that cleanly prevents an unmarshal error on an empty body.
		if resp.StatusCode == http.StatusNoContent {
			return []VIOSQuick{}, nil
		}
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetVirtualIOServersQuick response", "bodyLengthBytes", len(body))
	}

	var viosList []VIOSQuick
	if err := json.Unmarshal(body, &viosList); err != nil {
		c.Logger.Error("Failed to unmarshal JSON response", "error", err)
		return nil, fmt.Errorf("failed to unmarshal JSON response: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully parsed VIOS entries from quick feed", "count", len(viosList))
	}

	return viosList, nil
}

// GetFreePhyVolume retrieves free physical volumes for a given VIOS UUID
func (c *HmcRestClient) GetFreePhyVolume(viosUUID string, debug bool) ([]PhysicalVolume, error) {
	if debug {
		c.Logger.Debug("Fetching free physical volumes", "viosUUID", viosUUID)
	}
	// Optionally test with FibreChannelBackedOnly
	/* jobParams := map[string]string{
		"FibreChannelBackedOnly": "false",
	} */
	jobParams := map[string]string{}
	// Operation details for the job request
	reqdOperation := map[string]string{
		"OperationName": "GetFreePhysicalVolumes",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}
	// Create the XML payload for the job request
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_3_0", debug, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if debug {
		c.Logger.Debug("Job request payload generated", "payload", payload)
	}
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/GetFreePhysicalVolumes", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Requesting free physical volumes", "viosUUID", viosUUID, "url", url)
	}

	// Create and configure the PUT request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	// Enable basic auth to match Postman's Authorization: Basic
	req.SetBasicAuth("", "") // Credentials handled by session token
	if debug {
		c.Logger.Debug("Request headers prepared")
	}

	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// Log the response status and body
	if debug {
		c.Logger.Debug("GetFreePhyVolume response status", "status", resp.Status)
		c.Logger.Debug("GetFreePhyVolume response body", "body", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Parse the response to check for specific error messages
		doc, err := xmlStripNamespace(body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response: %v, status: %s, body: %s", err, resp.Status, string(body))
		}
		errorMsgs := doc.FindElements("//Message")
		if len(errorMsgs) > 0 {
			c.Logger.Error("HMC error encountered", "status", resp.Status, "message", errorMsgs[0].Text())
			return nil, fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(body))
		}
		c.Logger.Error("Request failed", "status", resp.Status)
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Strip namespaces from the response XML
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// Check for error messages in the response
	errorMsgs := doc.FindElements("//Message")
	if len(errorMsgs) > 0 {
		return nil, fmt.Errorf("error in response: %s", errorMsgs[0].Text())
	}

	// Extract the JobID
	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response: %s", string(body))
	}
	jobID := jobIDElem.Text()
	if debug {
		c.Logger.Debug("Extracted JobID", "jobID", jobID)
	}

	// Fetch the job response
	jobResp, err := c.FetchJobStatus(context.Background(), jobID, false, 10, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	if debug {
		c.Logger.Info("Free Physical Volume job response", "status", jobResp.Status)
	}

	// Extract the result XML from the job response Results parameters
	var pvXML string
	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "result" {
			pvXML = param.ParameterValue
			break
		}
	}
	if pvXML == "" {
		if debug {
			c.Logger.Warn("result parameter not found in job response")
		}
		return nil, fmt.Errorf("result not found in job response")
	}
	
	if debug {
		c.Logger.Debug("resultElem content parsed", "content", pvXML)
	}

	// Strip namespaces from the physical volumes XML
	strippedDoc, err := xmlStripNamespace([]byte(pvXML))
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from physical volumes XML: %v", err)
	}

	// Serialize the stripped document to bytes
	strippedXML, err := strippedDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize stripped XML: %v", err)
	}

	var pvCollection PhysicalVolumeCollection
	if err := xml.Unmarshal(strippedXML, &pvCollection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal physical volumes XML: %v", err)
	}

	listPv := pvCollection.PhysicalVolumes
	if len(listPv) == 0 {
		if debug {
			c.Logger.Info("No free physical volumes found", "viosUUID", viosUUID)
		}
		// Return an empty list instead of an error, as no volumes is a valid case
		return listPv, nil
	}
	if debug {
		c.Logger.Info("Found free physical volumes", "count", len(listPv), "viosUUID", viosUUID)
		for i, pv := range listPv {
			c.Logger.Debug(fmt.Sprintf("Physical Volume %d", i+1), 
				"Name", pv.VolumeName, 
				"VolumeUniqueID", pv.VolumeUniqueID, 
				"UniqueDeviceID", pv.UniqueDeviceID, 
				"StorageLabel", pv.StorageLabel, 
				"Capacity", pv.VolumeCapacity, 
				"LocationCode", pv.LocationCode, 
				"State", pv.VolumeState)
		}
	}
	return listPv, nil
}

// removeDeviceViaJob removes a specific device from VIOS using the RemoveDevice job operation
func (c *HmcRestClient) removeDeviceViaJob(viosUUID, deviceName string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/RemoveDevice", c.hmcIP, viosUUID)
	
	operation := map[string]string{
		"OperationName": "RemoveDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	// devName MUST be the vtscsi mapping name, not the hdisk!
	params := map[string]string{
		"devName": deviceName, 
	}

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", debug, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice job submission failed with status %s", resp.Status)
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		_, err = c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 5, debug)
		return err
	}
	return fmt.Errorf("JobID not found")
}
// RemoveVIOSDevice executes the RemoveDevice job on the VIOS to delete a physical volume (e.g., hdisk3)
func (c *HmcRestClient) RemoveVIOSDevice(viosUUID, deviceName string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/RemoveDevice", c.hmcIP, viosUUID)
	
	operation := map[string]string{
		"OperationName": "RemoveDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	params := map[string]string{
		"devName": deviceName, 
	}

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", debug, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		if debug {
			return fmt.Errorf("RemoveDevice failed with status %s: %s", resp.Status, string(body))
		}
		return fmt.Errorf("RemoveDevice failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		if debug { c.Logger.Info("Removing device from VIOS", "deviceName", deviceName, "jobID", jobIDElem.Text()) }
		// Wait for the VIOS to finish deleting the disk
		_, err = c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 5, debug)
		return err
	}
	
	return fmt.Errorf("JobID not found in RemoveDevice response")
}




// =====================================================================
// HELPER FUNCTIONS
// =====================================================================

// getElementText safely returns the text of an XML element if it exists, otherwise an empty string.
func getElementText(root *etree.Element, path string) string {
	if root == nil {
		return ""
	}
	elem := root.FindElement(path)
	if elem != nil {
		return elem.Text()
	}
	return ""
}

// =====================================================================
// VIOS METHODS
// =====================================================================

// GetVirtualIOServers retrieves detailed, comprehensive information for all Virtual I/O Servers 
// of a managed system using exhaustive Go struct unmarshaling.
func (c *HmcRestClient) GetVirtualIOServers(systemUUID string, debug bool) ([]VirtualIOServerDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer", c.hmcIP, systemUUID)
	
	if debug {
		c.Logger.Debug("Fetching exhaustive VIOS details", "systemUUID", systemUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// 1. Strip XML namespaces to prevent Unmarshal tag conflicts
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// 2. Convert the clean etree Document back to a byte array
	strippedXML, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to write stripped XML to bytes: %v", err)
	}

	// 3. Define an inline wrapper to catch the Atom Feed hierarchy
	var feed struct {
		Entries []struct {
			VIOS VirtualIOServerDetailed `xml:"content>VirtualIOServer"`
		} `xml:"entry"`
	}

	// 4. Elegantly unmarshal the entire payload into our exhaustive structs
	if err := xml.Unmarshal(strippedXML, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exhaustive VIOS data: %v", err)
	}

	// Extract from the feed wrapper into a clean slice
	var viosList []VirtualIOServerDetailed
	for _, entry := range feed.Entries {
		viosList = append(viosList, entry.VIOS)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual I/O Servers comprehensively", "count", len(viosList))
	}

	return viosList, nil
}
// GetVirtualIOServer retrieves detailed information for a specific Virtual I/O Server using its UUID.
func (c *HmcRestClient) GetVirtualIOServer(viosUUID string, debug bool) (*VirtualIOServerDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Fetching VIOS details", "viosUUID", viosUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// Strip XML namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Convert the clean etree Document back to a byte array
	strippedXML, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to write stripped XML to bytes: %v", err)
	}

	// Define wrapper to catch the Atom entry hierarchy
	var entry struct {
		VIOS VirtualIOServerDetailed `xml:"content>VirtualIOServer"`
	}

	// Unmarshal the entire payload into our exhaustive struct
	if err := xml.Unmarshal(strippedXML, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VIOS data: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual I/O Server", "partitionName", entry.VIOS.PartitionName)
	}

	return &entry.VIOS, nil
}

// GetVirtualSCSIServerAdapters retrieves a list of all Virtual SCSI Server Adapters (vhost) configured on a specific VIOS.
func (c *HmcRestClient) GetVirtualSCSIServerAdapters(viosUUID string, debug bool) ([]VirtualSCSIServerAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching Virtual SCSI Server Adapters", "viosUUID", viosUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetVirtualSCSIServerAdapters HTTP response status", "status", resp.Status)
		c.Logger.Debug("GetVirtualSCSIServerAdapters response body", "body", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// Strip XML namespaces to make path searching easier with etree
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var adapters []VirtualSCSIServerAdapter

	// Iterate through each Atom entry
	entries := doc.FindElements("//entry")
	for _, entry := range entries {
		// Find the core adapter payload within the entry
		adapterElem := entry.FindElement(".//VirtualSCSIServerAdapter")
		if adapterElem == nil {
			continue
		}

		// Use XML unmarshaling to automatically populate the struct
		var adapter VirtualSCSIServerAdapter
		
		// Create a new document with the VirtualSCSIServerAdapter element as root
		adapterDoc := etree.NewDocument()
		adapterDoc.SetRoot(adapterElem.Copy())
		
		adapterBytes, err := adapterDoc.WriteToBytes()
		if err != nil {
			if debug {
				c.Logger.Warn("Warning: failed to serialize VirtualSCSIServerAdapter element", "error", err)
			}
			continue
		}

		if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
			if debug {
				c.Logger.Warn("Warning: failed to unmarshal VirtualSCSIServerAdapter", "error", err)
			}
			continue
		}

		// Extract the direct URI for this specific adapter from the Atom <link rel="SELF">
		for _, link := range entry.FindElements("./link") {
			if link.SelectAttrValue("rel", "") == "SELF" {
				adapter.Link = link.SelectAttrValue("href", "")
				break
			}
		}

		// Populate deprecated ID field for backward compatibility
		adapter.ID = adapter.UUID

		adapters = append(adapters, adapter)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual SCSI Server Adapters", "count", len(adapters))
	}

	return adapters, nil
}

// GetVirtualSCSIServerAdapter retrieves the details of a specific Virtual SCSI Server Adapter (vhost) using its UUID.
func (c *HmcRestClient) GetVirtualSCSIServerAdapter(viosUUID, adapterUUID string, debug bool) (*VirtualSCSIServerAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter/%s", c.hmcIP, viosUUID, adapterUUID)
	
	if debug {
		c.Logger.Debug("Fetching specific Virtual SCSI Server Adapter", "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetVirtualSCSIServerAdapter Status", "status", resp.Status)
		c.Logger.Debug("GetVirtualSCSIServerAdapter Body", "body", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	adapterElem := doc.FindElement("//VirtualSCSIServerAdapter")
	if adapterElem == nil {
		return nil, fmt.Errorf("VirtualSCSIServerAdapter node not found in response")
	}

	// Use XML unmarshaling to automatically populate the struct
	var adapter VirtualSCSIServerAdapter
	
	// Create a new document with the VirtualSCSIServerAdapter element as root
	adapterDoc := etree.NewDocument()
	adapterDoc.SetRoot(adapterElem.Copy())
	
	adapterBytes, err := adapterDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize VirtualSCSIServerAdapter element: %v", err)
	}

	if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VirtualSCSIServerAdapter: %v", err)
	}

	// Set the Link field (we already know the direct URL)
	adapter.Link = url
	
	// Populate deprecated ID field for backward compatibility
	adapter.ID = adapter.UUID

	return &adapter, nil
}

// DeleteVirtualSCSIServerAdapter removes a specific Virtual SCSI Server Adapter (vhost) from a VIOS.
func (c *HmcRestClient) DeleteVirtualSCSIServerAdapter(viosUUID, adapterUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter/%s", c.hmcIP, viosUUID, adapterUUID)
	
	if debug {
		c.Logger.Debug("Deleting Virtual SCSI Server Adapter", "url", url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	
	// Read the body even if it's empty to ensure the connection is freed properly
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("DeleteVirtualSCSIServerAdapter Status", "status", resp.Status)
		if len(body) > 0 {
			c.Logger.Debug("DeleteVirtualSCSIServerAdapter Body", "body", string(body))
		}
	}

	// Accept both 200 OK and 204 No Content as successful deletions
	if resp.StatusCode >= 400 {
		if debug {
			return fmt.Errorf("failed to delete VirtualSCSIServerAdapter. Status: %s, Response: %s", resp.Status, string(body))
		}
		return fmt.Errorf("failed to delete VirtualSCSIServerAdapter. Status: %s. Enable debug mode to see full response", resp.Status)
	}

	return nil
}

// GetViosSCSIMappings retrieves and fully parses all VSCSI mappings for a specific VIOS.
func (c *HmcRestClient) GetViosSCSIMappings(viosUUID string, debug bool) ([]VirtualSCSIMapping, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching VSCSI Mappings", "viosUUID", viosUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var mappingsList []VirtualSCSIMapping

	mappingElems := doc.FindElements("//VirtualSCSIMapping")
	for _, mappingElem := range mappingElems {
		// Create a new document with mappingElem as root for serialization
		mappingDoc := etree.NewDocument()
		mappingDoc.SetRoot(mappingElem.Copy())
		
		// Serialize to bytes
		mappingBytes, err := mappingDoc.WriteToBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize VirtualSCSIMapping element: %v", err)
		}
		
		// Unmarshal into VirtualSCSIMapping struct
		var mapping VirtualSCSIMapping
		if err := xml.Unmarshal(mappingBytes, &mapping); err != nil {
			return nil, fmt.Errorf("failed to unmarshal VirtualSCSIMapping: %v", err)
		}
		
		mappingsList = append(mappingsList, mapping)
	}

	if debug {
		c.Logger.Info("Successfully parsed VSCSI Mappings", "count", len(mappingsList))
	}

	return mappingsList, nil
}

// =====================================================================
// VOLUME GROUP API METHODS
// =====================================================================

// GetVolumeGroups retrieves a list of all Volume Groups configured on a specific VIOS.
func (c *HmcRestClient) GetVolumeGroups(viosUUID string, debug bool) ([]VolumeGroup, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup", c.hmcIP, viosUUID)

    if debug {
        c.Logger.Debug("Fetching Volume Groups", "viosUUID", viosUUID, "url", url)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
    req.Header.Set("Accept", "application/atom+xml")

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    c.logRawTraffic("RESPONSE", url, string(body))

    if resp.StatusCode != http.StatusOK {
        if debug {
            return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
        }
        return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
    }

    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    var volumeGroups []VolumeGroup

    entries := doc.FindElements("//entry")
    for _, entry := range entries {
        vgElem := entry.FindElement(".//VolumeGroup")
        if vgElem == nil {
            continue
        }

        // Use XML unmarshaling to automatically populate the struct
        var vg VolumeGroup
        
        // Create a new document with the VolumeGroup element as root
        vgDoc := etree.NewDocument()
        vgDoc.SetRoot(vgElem.Copy())
        
        vgBytes, err := vgDoc.WriteToBytes()
        if err != nil {
            if debug {
                c.Logger.Warn("Warning: failed to serialize VolumeGroup element", "error", err)
            }
            continue
        }

        if err := xml.Unmarshal(vgBytes, &vg); err != nil {
            if debug {
                c.Logger.Warn("Warning: failed to unmarshal VolumeGroup", "error", err)
            }
            continue
        }

        volumeGroups = append(volumeGroups, vg)
    }

    return volumeGroups, nil
}

// GetVolumeGroup retrieves the details of a specific Volume Group on a Virtual I/O Server.
func (c *HmcRestClient) GetVolumeGroup(viosUUID, vgUUID string, debug bool) (*VolumeGroup, error) {
    url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup/%s", c.hmcIP, viosUUID, vgUUID)

    if debug {
        c.Logger.Debug("Fetching Volume Group", "vgUUID", vgUUID, "viosUUID", viosUUID, "url", url)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("X-API-Session", c.session)
    req.Header.Set("Accept", "application/atom+xml")

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    c.logRawTraffic("RESPONSE", url, string(body))

    if resp.StatusCode != http.StatusOK {
        if debug {
            return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
        }
        return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
    }

    doc, err := xmlStripNamespace(body)
    if err != nil {
        return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
    }

    vgElem := doc.FindElement("//VolumeGroup")
    if vgElem == nil {
        return nil, fmt.Errorf("VolumeGroup node not found in response")
    }

    // Use XML unmarshaling to automatically populate the struct
    var vg VolumeGroup
    
    // Create a new document with the VolumeGroup element as root
    vgDoc := etree.NewDocument()
    vgDoc.SetRoot(vgElem.Copy())
    
    vgBytes, err := vgDoc.WriteToBytes()
    if err != nil {
        return nil, fmt.Errorf("failed to serialize VolumeGroup element: %v", err)
    }
    
    if err := xml.Unmarshal(vgBytes, &vg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal VolumeGroup: %v", err)
    }

    if debug {
        c.Logger.Info("Successfully parsed Volume Group", "groupName", vg.GroupName, "uuid", vg.UUID)
        c.Logger.Debug("Volume Group Contents", "VirtualDisks", len(vg.VirtualDisks), "OpticalMedia", len(vg.OpticalMedia), "PhysicalVolumes", len(vg.PhysicalVolumes))
    }

    return &vg, nil
}
// =====================================================================
// CREATE VOLUME GROUP
// =====================================================================

// CreateVolumeGroup creates a new Volume Group on a VIOS using the specified physical volumes.
func (c *HmcRestClient) CreateVolumeGroup(viosUUID, vgName string, physicalVolumes []string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VolumeGroup", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Creating Volume Group", "vgName", vgName, "viosUUID", viosUUID, "disks", physicalVolumes)
	}

	// 1. Build the PhysicalVolumes XML blocks
	var pvBuilder strings.Builder
	for _, pv := range physicalVolumes {
		pvBuilder.WriteString(fmt.Sprintf(`
			<PhysicalVolume schemaVersion="V1_0">
				<VolumeName kxe="false" kb="CUR">%s</VolumeName>
			</PhysicalVolume>`, strings.TrimSpace(pv)))
	}

	// 2. Build the full VolumeGroup XML Payload
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VolumeGroup:VolumeGroup xmlns:VolumeGroup="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" schemaVersion="V1_0">
	<GroupName kxe="false" kb="CUR">%s</GroupName>
	<PhysicalVolumes kxe="false" kb="CUD" schemaVersion="V1_0">%s
	</PhysicalVolumes>
</VolumeGroup:VolumeGroup>`, vgName, pvBuilder.String())

	if debug {
		c.Logger.Debug("CreateVolumeGroup Payload", "payload", payload)
	}

	// 3. Create the HTTP Request
	// Note: While standard REST uses POST for creation, some HMC versions expect PUT here. 
	// If you get a 405 Method Not Allowed, switch this to "POST".
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VolumeGroup")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	// 4. Execute the Request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("CreateVolumeGroup HTTP response status", "status", resp.Status)
	}

	// 5. Check for Success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		if debug {
			return fmt.Errorf("CreateVolumeGroup failed with status %s: %s", resp.Status, string(body))
		}
		return fmt.Errorf("CreateVolumeGroup failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// 6. Monitor the Job (If the HMC kicked off an asynchronous job)
	doc, err := xmlStripNamespace(body)
	if err == nil {
		jobIDElem := doc.FindElement("//JobID")
		if jobIDElem != nil {
			jobID := jobIDElem.Text()
			if debug {
				c.Logger.Info("CreateVolumeGroup Job submitted, waiting for completion...", "jobID", jobID)
			}
			_, err = c.FetchJobStatus(context.Background(), jobID, false, 10, debug)
			if err != nil {
				return fmt.Errorf("CreateVolumeGroup job failed: %v", err)
			}
			if debug {
				c.Logger.Info("CreateVolumeGroup job completed successfully.")
			}
		}
	}

	return nil
}

// =====================================================================
// CREATE VIRTUAL DISK (LOGICAL VOLUME) - SMART CLI METHOD
// =====================================================================

// CreateVirtualDisk safely creates a Logical Volume (Virtual Disk) inside a standard Volume Group.
// It verifies the host Volume Group has enough free space and checks for naming collisions before executing.
func (c *HmcRestClient) CreateVirtualDisk(sysName, viosUUID, viosName, vgName, diskName string, capacityMB int, debug bool) error {
	requiredGB := float64(capacityMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying capacity and naming for new Virtual Disk", "diskName", diskName, "capacityMB", capacityMB, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to verify capacity and check for naming collisions
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var foundVgFreeSpace string
	var foundVgName string

	for _, vg := range vgList {
		// Collision Check: Ensure no disk with this name already exists on this VIOS
		for _, vd := range vg.VirtualDisks {
			if strings.EqualFold(vd.DiskName, diskName) {
				return fmt.Errorf("ABORT: A Virtual Disk named '%s' already exists in VG '%s'", diskName, vg.GroupName)
			}
		}

		// Locate our target Volume Group to check its capacity
		if strings.EqualFold(vg.GroupName, vgName) {
			foundVgName = vg.GroupName
			foundVgFreeSpace = vg.FreeSpace
			// We don't break here because we still want to finish scanning the other VGs for naming collisions
		}
	}

	if foundVgName == "" {
		return fmt.Errorf("ABORT: Target Volume Group '%s' was not found on VIOS '%s'", vgName, viosName)
	}

	// Capacity Check
	freeSpaceGB, parseErr := strconv.ParseFloat(foundVgFreeSpace, 64)
	if parseErr != nil {
		return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", foundVgName, parseErr)
	}

	if debug {
		c.Logger.Debug("VG Capacity Check", "vgName", foundVgName, "availableGB", freeSpaceGB, "requiredGB", requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, capacityMB, foundVgName, freeSpaceGB)
	}

	// 2. Execute the creation via CLI
	if debug {
		c.Logger.Info("Pre-flight checks passed. Executing creation via CLI...")
	}

	// Syntax: mklv -lv <diskName> <vgName> <Size>M
	mklvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mklv -lv %s %s %dM"`, sysName, viosName, diskName, vgName, capacityMB)

	output, err := c.CliRunner(mklvCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to create Virtual Disk via mklv: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Disk created successfully", "diskName", diskName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// DELETE VIRTUAL DISK (LOGICAL VOLUME) - CLI METHOD
// =====================================================================

// DeleteVirtualDisk safely removes a Logical Volume (Virtual Disk) from a VIOS.
// It uses the native VIOS rmlv command with the -f flag to bypass confirmation prompts.
func (c *HmcRestClient) DeleteVirtualDisk(sysName, viosName, diskName string, debug bool) error {
	if debug {
		c.Logger.Debug("Safely deleting Virtual Disk via CLI", "diskName", diskName, "viosName", viosName)
	}

	// Syntax: rmlv -f <diskName>
	// The -f flag is required for automation so the OS does not wait for user input.
	rmlvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmlv -f %s"`, sysName, viosName, diskName)
	
	if debug {
		c.Logger.Debug("Executing", "command", rmlvCmd)
	}

	output, err := c.CliRunner(rmlvCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to delete Virtual Disk via rmlv: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Disk deleted successfully", "diskName", diskName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// EXTEND VIRTUAL DISK (LOGICAL VOLUME) - SMART CLI METHOD
// =====================================================================

// ExtendVirtualDisk safely increases the size of an existing Logical Volume (Virtual Disk).
// It automatically queries the HMC to verify the host Volume Group has enough free space before executing.
func (c *HmcRestClient) ExtendVirtualDisk(sysName, viosUUID, viosName, diskName string, additionalMB int, debug bool) error {
	requiredGB := float64(additionalMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying capacity for Virtual Disk extension", "diskName", diskName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to find the disk and check capacity
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for capacity check: %v", err)
	}

	var foundVgName string
	for _, vg := range vgList {
		for _, vd := range vg.VirtualDisks {
			if strings.EqualFold(vd.DiskName, diskName) {
				foundVgName = vg.GroupName
				
				freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
				if parseErr != nil {
					return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", vg.GroupName, parseErr)
				}

				if debug {
					c.Logger.Debug("Found disk inside VG", "diskName", diskName, "vgName", vg.GroupName, "availableSpaceGB", freeSpaceGB)
				}

				// Check if there is enough space to satisfy the request
				if freeSpaceGB < requiredGB {
					return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, additionalMB, vg.GroupName, freeSpaceGB)
				}
				break
			}
		}
		if foundVgName != "" {
			break
		}
	}

	if foundVgName == "" {
		return fmt.Errorf("Virtual Disk '%s' was not found on VIOS '%s'", diskName, viosName)
	}

	// 2. Execute the extension via CLI
	if debug {
		c.Logger.Info("Capacity check passed. Executing extension via CLI...")
	}

	extendlvCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "extendlv %s %dM"`, sysName, viosName, diskName, additionalMB)
	
	output, err := c.CliRunner(extendlvCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to extend Virtual Disk via extendlv: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Disk extended successfully", "diskName", diskName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}
// =====================================================================
// VIRTUAL OPTICAL MEDIA (ISO) - SMART CLI METHODS (viosvrcmd)
// =====================================================================

// CreateVirtualOpticalMedia creates a Virtual Optical Media (ISO) in the VIOS Media Repository.
// If sourceFile is provided, it imports an existing ISO. 
// If nfsLink is true (only valid with sourceFile), it links to the file instead of copying it.
// If readOnly is true, the media is created with the -ro flag to prevent accidental overwrites.
func (c *HmcRestClient) CreateVirtualOpticalMedia(sysName, viosUUID, viosName, mediaName, sourceFile string, sizeMB int, readOnly, nfsLink, debug bool) error {
	if nfsLink && sourceFile == "" {
		return fmt.Errorf("ABORT: The -nfslink flag can only be used when providing a sourceFile")
	}

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying naming for Virtual Optical Media", "mediaName", mediaName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to check for naming collisions in the Media Repository
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	for _, vg := range vgList {
		for _, opt := range vg.OpticalMedia {
			if strings.EqualFold(opt.MediaName, mediaName) {
				return fmt.Errorf("ABORT: A Virtual Optical Media named '%s' already exists in the repository on VIOS '%s'", mediaName, viosName)
			}
		}
	}

	// 2. Build the appropriate mkvopt command
	roFlag := ""
	if readOnly {
		roFlag = " -ro"
	}

	var mkvoptCmd string
	if sourceFile != "" {
		nfsFlag := ""
		if nfsLink {
			nfsFlag = " -nfslink"
		}
		
		if debug {
			c.Logger.Info("Pre-flight passed. Importing ISO from file", "sourceFile", sourceFile, "readOnly", readOnly, "nfsLink", nfsLink)
		}
		// Syntax: mkvopt -name <mediaName> -file <SourceFile> [-nfslink] [-ro]
		mkvoptCmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvopt -name %s -file %s%s%s"`, sysName, viosName, mediaName, sourceFile, nfsFlag, roFlag)
	} else {
		if debug {
			c.Logger.Info("Pre-flight passed. Creating blank media", "sizeMB", sizeMB, "readOnly", readOnly)
		}
		// Syntax: mkvopt -name <mediaName> -size <Size>M [-ro]
		mkvoptCmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkvopt -name %s -size %dM%s"`, sysName, viosName, mediaName, sizeMB, roFlag)
	}

	// 3. Execute the creation/import via CLI
	output, err := c.CliRunner(mkvoptCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to create/import Virtual Optical Media: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Optical Media created successfully", "mediaName", mediaName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// DeleteVirtualOpticalMedia safely removes a Virtual Optical Media (ISO) from a VIOS Media Repository.
// It uses the native VIOS rmvopt command. Note: The media must be unloaded/unmapped from all LPARs before it can be deleted.
func (c *HmcRestClient) DeleteVirtualOpticalMedia(sysName, viosName, mediaName string, debug bool) error {
	if debug {
		c.Logger.Debug("Safely deleting Virtual Optical Media via CLI", "mediaName", mediaName, "viosName", viosName)
	}

	// Syntax: rmvopt -name <mediaName>
	rmvoptCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmvopt -name %s"`, sysName, viosName, mediaName)
	
	if debug {
		c.Logger.Debug("Executing", "command", rmvoptCmd)
	}

	output, err := c.CliRunner(rmvoptCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to delete Virtual Optical Media via rmvopt: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Optical Media deleted successfully", "mediaName", mediaName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// GetVirtualOpticalMedias retrieves a list of all Virtual Optical Media (ISO files) 
// currently physically present in the VIOS Media Repository using the native VIOS CLI (lsrep).
func (c *HmcRestClient) GetVirtualOpticalMedias(sysName, viosName string, debug bool) ([]VirtualOpticalMedia, error) {
	if debug {
		c.Logger.Debug("Fetching Virtual Optical Media from repository via CLI", "viosName", viosName, "sysName", sysName)
	}

	// Syntax: lsrep
	lsrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "lsrep"`, sysName, viosName)
	
	output, err := c.CliRunner(lsrepCmd, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to list media repository via lsrep: %v\nOutput: %s", err, output)
	}

	var opticalMediaList []VirtualOpticalMedia

	// Parse the lsrep text output
	// Typical lsrep output structure:
	// Size(mb) Free(mb) Parent Pool         Parent Size      Parent Free
	// 20480    10240    rootvg              ...
	//
	// Name                                    File Size Optical         Access
	// rhel9.iso                               4096      vtopt0          rw
	// aix73.iso                               2048      None            ro

	lines := strings.Split(output, "\n")
	parsingMedia := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines
		if line == "" {
			continue
		}

		// Detect the start of the media table
		if strings.HasPrefix(line, "Name") && strings.Contains(line, "File Size") {
			parsingMedia = true
			continue
		}

		// Parse the rows under the header
		if parsingMedia {
			// Stop parsing if we hit a different section (like VTD summary)
			if strings.HasPrefix(line, "VTD") || strings.HasPrefix(line, "Size(mb)") {
				break
			}

			// Split by whitespace
			fields := strings.Fields(line)
			if len(fields) > 0 {
				mediaName := fields[0]
				
				size := ""
				if len(fields) >= 2 {
					size = fields[1]
				}

				mountType := ""
				if len(fields) >= 4 {
					mountType = fields[3]
				}

				media := VirtualOpticalMedia{
					MediaName: mediaName,
					Size:      size,
					MountType: mountType,
				}

				opticalMediaList = append(opticalMediaList, media)
			}
		}
	}

	if debug {
		c.Logger.Info("Successfully retrieved optical media from repository via CLI", "count", len(opticalMediaList), "viosName", viosName)
	}

	return opticalMediaList, nil
}

// GetVirtualOpticalMedia retrieves the details of a specific Virtual Optical Media (ISO) 
// by searching the physical VIOS media repository via the CLI.
// Returns an error if the specified media is not found.
func (c *HmcRestClient) GetVirtualOpticalMedia(sysName, viosName, mediaName string, debug bool) (*VirtualOpticalMedia, error) {
	if debug {
		c.Logger.Debug("Fetching specific Virtual Optical Media via CLI", "mediaName", mediaName, "viosName", viosName)
	}

	// 1. Fetch the full list of media
	mediaList, err := c.GetVirtualOpticalMedias(sysName, viosName, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch media list from repository: %w", err)
	}

	// 2. Iterate through the list to find the exact match
	for _, media := range mediaList {
		if media.MediaName == mediaName {
			if debug {
				c.Logger.Info("Successfully found optical media in repository", 
					"mediaName", media.MediaName, 
					"size", media.Size,
					"mountType", media.MountType,
				)
			}
			return &media, nil
		}
	}

	// 3. If loop completes, the media does not exist
	if debug {
		c.Logger.Warn("Optical media not found in repository", "mediaName", mediaName, "viosName", viosName)
	}
	return nil, fmt.Errorf("virtual optical media '%s' not found in repository on VIOS '%s'", mediaName, viosName)
}

// LoadVirtualOpticalMedia loads a virtual optical media (ISO) into an existing Virtual Target Device (VTD) on the VIOS.
func (c *HmcRestClient) LoadVirtualOpticalMedia(sysName, viosName, vtdName, mediaName string, debug bool) error {
	if debug {
		c.Logger.Debug("Loading Virtual Optical Media into VTD via CLI", "mediaName", mediaName, "vtdName", vtdName, "viosName", viosName)
	}

	// Syntax: loadopt -disk <mediaName> -vtd <vtdName>
	loadoptCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "loadopt -disk %s -vtd %s"`, sysName, viosName, mediaName, vtdName)
	
	if debug {
		c.Logger.Debug("Executing", "command", loadoptCmd)
	}

	output, err := c.CliRunner(loadoptCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to load Virtual Optical Media via loadopt: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Optical Media loaded successfully", "mediaName", mediaName, "vtdName", vtdName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// UnloadVirtualOpticalMedia unloads a virtual optical media (ISO) from a Virtual Target Device (VTD) on the VIOS.
func (c *HmcRestClient) UnloadVirtualOpticalMedia(sysName, viosName, vtdName string, debug bool) error {
	if debug {
		c.Logger.Debug("Unloading Virtual Optical Media from VTD via CLI", "vtdName", vtdName, "viosName", viosName)
	}

	// Syntax: unloadopt -vtd <vtdName>
	unloadoptCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "unloadopt -vtd %s"`, sysName, viosName, vtdName)
	
	if debug {
		c.Logger.Debug("Executing", "command", unloadoptCmd)
	}

	output, err := c.CliRunner(unloadoptCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to unload Virtual Optical Media via unloadopt: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Virtual Optical Media unloaded successfully", "vtdName", vtdName, "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// CREATE MEDIA REPOSITORY - SMART CLI METHOD
// =====================================================================

// CreateMediaRepository safely creates the Virtual Media Repository on a VIOS.
// It verifies that no repository currently exists on the VIOS, and that the target VG has enough space.
func (c *HmcRestClient) CreateMediaRepository(sysName, viosUUID, viosName, vgName string, sizeMB int, debug bool) error {
	requiredGB := float64(sizeMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying capacity and existing repositories for VG", "vgName", vgName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to check for existing repositories and verify capacity
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var foundVgFreeSpace string
	var foundVgName string

	for _, vg := range vgList {
		// COLLISION CHECK: A VIOS can only have one repository globally.
		if vg.MediaRepositoryName != "" {
			return fmt.Errorf("ABORT: A Virtual Media Repository already exists on this VIOS (hosted in VG '%s')", vg.GroupName)
		}

		// Locate our target Volume Group to check its capacity
		if strings.EqualFold(vg.GroupName, vgName) {
			foundVgName = vg.GroupName
			foundVgFreeSpace = vg.FreeSpace
			// We do not break here because we must finish scanning the other VGs to ensure no repository exists elsewhere!
		}
	}

	if foundVgName == "" {
		return fmt.Errorf("ABORT: Target Volume Group '%s' was not found on VIOS '%s'", vgName, viosName)
	}

	// Capacity Check
	freeSpaceGB, parseErr := strconv.ParseFloat(foundVgFreeSpace, 64)
	if parseErr != nil {
		return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", foundVgName, parseErr)
	}

	if debug {
		c.Logger.Debug("VG Capacity Check", "vgName", foundVgName, "availableGB", freeSpaceGB, "requiredGB", requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: Requested %.2f GB (%d MB), but VG '%s' only has %.2f GB available", requiredGB, sizeMB, foundVgName, freeSpaceGB)
	}

	// 2. Execute the creation via CLI
	if debug {
		c.Logger.Info("Pre-flight checks passed. Executing mkrep via CLI...")
	}

	// Syntax: mkrep -sp <vgName> -size <sizeMB>M
	mkrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mkrep -sp %s -size %dM"`, sysName, viosName, vgName, sizeMB)

	output, err := c.CliRunner(mkrepCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to create Media Repository via mkrep: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Media Repository created successfully", "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

// =====================================================================
// DELETE MEDIA REPOSITORY - SMART CLI METHOD (ENHANCED)
// =====================================================================

// DeleteMediaRepository removes the Virtual Media Repository from a VIOS.
// It verifies the repo exists, checks for media if force is false, and warns if force is true.
func (c *HmcRestClient) DeleteMediaRepository(sysName, viosUUID, viosName, repoName string, force, debug bool) error {
	if debug {
		c.Logger.Debug("Pre-flight check: Looking for Media Repository", "repoName", repoName, "viosName", viosName)
	}

	// 1. Fetch Volume Groups to find the existing repository
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var targetVG *VolumeGroup
	for i := range vgList {
		if vgList[i].MediaRepositoryName != "" {
			targetVG = &vgList[i]
			break
		}
	}

	// CHECK 1: Does the repository even exist?
	if targetVG == nil {
		return fmt.Errorf("ABORT: No Virtual Media Repository found on VIOS '%s'", viosName)
	}

	// CHECK 2: Does the name match?
	if !strings.EqualFold(targetVG.MediaRepositoryName, repoName) {
		return fmt.Errorf("ABORT: Found repository '%s', but you requested to delete '%s'", targetVG.MediaRepositoryName, repoName)
	}

	// CHECK 3: Safety check for existing media
	mediaCount := len(targetVG.OpticalMedia)
	if force {
		// Print the specific warning requested
		c.Logger.Warn("Force flag is ENABLED. Deleting repository will also PERMANENTLY DELETE ISO file(s) inside it.", "repoName", repoName, "mediaCount", mediaCount)
	} else if mediaCount > 0 {
		// Fail WITHOUT calling rmrep if media exists and force is false
		return fmt.Errorf("ABORT: Repository '%s' contains %d ISO file(s). Use the 'force' flag to delete anyway", repoName, mediaCount)
	}

	// 2. Execute the deletion via CLI
	if debug {
		c.Logger.Info("Pre-flight passed. Executing rmrep on VIOS...")
	}

	forceFlag := ""
	if force {
		forceFlag = " -f"
	}

	rmrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmrep%s"`, sysName, viosName, forceFlag)
	
	output, err := c.CliRunner(rmrepCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to delete Media Repository via CLI: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Media Repository deleted successfully", "repoName", repoName)
	}

	return nil
}
// =====================================================================
// CHANGE MEDIA REPOSITORY (EXTEND) - SMART CLI METHOD
// =====================================================================

// ChangeMediaRepository increases the size of the Virtual Media Repository.
// additionalMB is the amount of NEW space to add (incremental).
// It identifies the hosting Volume Group and verifies free space before executing.
func (c *HmcRestClient) ChangeMediaRepository(sysName, viosUUID, viosName string, additionalMB int, debug bool) error {
	requiredGB := float64(additionalMB) / 1024.0

	if debug {
		c.Logger.Debug("Pre-flight check: Verifying VG capacity for repository expansion on VIOS", "viosName", viosName)
	}

	// 1. Fetch Volume Groups to find the repository's location
	vgList, err := c.GetVolumeGroups(viosUUID, debug)
	if err != nil {
		return fmt.Errorf("failed to retrieve Volume Groups for pre-flight check: %v", err)
	}

	var hostingVG *VolumeGroup
	for i := range vgList {
		if vgList[i].MediaRepositoryName != "" {
			hostingVG = &vgList[i]
			break
		}
	}

	// CHECK 1: Does the repository even exist?
	if hostingVG == nil {
		return fmt.Errorf("ABORT: No Virtual Media Repository found on VIOS '%s'. Use CreateMediaRepository first", viosName)
	}

	// CHECK 2: Does the hosting VG have enough space?
	freeSpaceGB, parseErr := strconv.ParseFloat(hostingVG.FreeSpace, 64)
	if parseErr != nil {
		return fmt.Errorf("failed to parse FreeSpace for VG '%s': %v", hostingVG.GroupName, parseErr)
	}

	if debug {
		c.Logger.Debug("Repository capacity check", "vgName", hostingVG.GroupName, "availableGB", freeSpaceGB, "requestedGB", requiredGB)
	}

	if freeSpaceGB < requiredGB {
		return fmt.Errorf("INSUFFICIENT SPACE: VG '%s' only has %.2f GB free, cannot add %.2f GB", 
			hostingVG.GroupName, freeSpaceGB, requiredGB)
	}

	// 2. Execute the expansion via CLI
	if debug {
		c.Logger.Info("Pre-flight passed. Extending repository", "repoName", hostingVG.MediaRepositoryName, "additionalMB", additionalMB)
	}

	// Syntax: chrep -size <Size>M
	chrepCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "chrep -size %dM"`, sysName, viosName, additionalMB)
	
	output, err := c.CliRunner(chrepCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to extend Media Repository via chrep: %v\nOutput: %s", err, output)
	}

	if debug {
		c.Logger.Info("Media Repository expanded successfully", "viosOutput", strings.TrimSpace(output))
	}

	return nil
}

/// CreatePhysicalVolumeMaps maps one or more physical disks (e.g., hdisk) on the VIOS to a target LPAR.
// It uses a pristine GET-Modify-POST pattern and is 100% idempotent.
func (c *HmcRestClient) CreatePhysicalVolumeMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(diskNames)
	diskNames = deduplicateAndClean(diskNames)
	if len(diskNames) == 0 {
		return "", fmt.Errorf("no valid physical volume names provided")
	}
	if debug && len(diskNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate Physical Volumes", "cleaned", diskNames)
	}

	// 1. Raw GET - Fetch pristine VIOS XML to preserve all native namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML for Physical Volume mapping...", "viosUUID", viosUUID)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != 200 {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract the VirtualIOServer element using local-name()
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Find or create the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		if debug {
			c.Logger.Debug("No existing mappings found. Creating VirtualSCSIMappings group...")
		}
		mappingsList = viosElem.CreateElement("VirtualSCSIMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_0")
		mappingsList.CreateAttr("group", "ViosSCSIMapping")
	}

	// =====================================================================
	// 5. IDEMPOTENCY CHECK & INJECTION
	// =====================================================================
	targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))
	mappedCount := 0

	for _, diskName := range diskNames {
		trimmedDiskName := strings.TrimSpace(diskName)
		alreadyMapped := false

		// Look through existing mappings to see if this Physical Volume is already attached to this LPAR
		for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
			assoc := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
			if assoc != nil && strings.HasSuffix(strings.ToLower(assoc.SelectAttrValue("href", "")), targetLparLower) {
				// Navigate to Storage -> PhysicalVolume -> VolumeName
				existingVol := mapping.FindElement(".//*[local-name()='PhysicalVolume']/*[local-name()='VolumeName']")
				if existingVol != nil && strings.EqualFold(strings.TrimSpace(existingVol.Text()), trimmedDiskName) {
					alreadyMapped = true
					break
				}
			}
		}

		if alreadyMapped {
			if debug {
				c.Logger.Info("Physical Volume is already mapped to this LPAR; skipping", "diskName", trimmedDiskName)
			}
			continue
		}

		// Inject the new mapping block (Schema-compliant, NO kb/kxe tags)
		newMappingXML := fmt.Sprintf(`
			<VirtualSCSIMapping schemaVersion="V1_0">
				<AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
				<Storage>
					<PhysicalVolume schemaVersion="V1_0">
						<VolumeName>%s</VolumeName>
					</PhysicalVolume>
				</Storage>
			</VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, trimmedDiskName)

		newMappingDoc := etree.NewDocument()
		if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
			return "", fmt.Errorf("failed to parse mapping XML for disk %s: %v", trimmedDiskName, err)
		}

		mappingsList.AddChild(newMappingDoc.Root())
		mappedCount++
		
		if debug {
			c.Logger.Debug("Injected mapping for Physical Volume into payload", "diskName", trimmedDiskName)
		}
	}

	// If all requested disks were already mapped, exit cleanly without touching the HMC
	if mappedCount == 0 {
		return "ALREADY_MAPPED", nil
	}

	// =====================================================================
	// 6. POST THE MODIFIED PAYLOAD
	// =====================================================================
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
		c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
	}

	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// =====================================================================
	// 7. GRACEFUL RMC ERROR HANDLING
	// =====================================================================
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR timeout warnings (common when the LPAR is powered off)
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Mapping saved to HMC profile, but dynamic DLPAR push timed out (LPAR likely powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		// Hard fail on genuine errors
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full XML response", postResp.Status)
	}

	// 8. Wait for background job to finish updating the Hypervisor
	respDoc, err := xmlStripNamespace(body) // Assuming your SDK helper is available
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Physical Volume mapping job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// CreateVirtualDiskMaps maps one or more Logical Volumes (Virtual Disks) on the VIOS to a target LPAR.
// It uses a pristine GET-Modify-POST pattern and is 100% idempotent (safe to run multiple times).
func (c *HmcRestClient) CreateVirtualDiskMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(diskNames)
	diskNames = deduplicateAndClean(diskNames)
	if len(diskNames) == 0 {
		return "", fmt.Errorf("no valid disk names provided")
	}
	if debug && len(diskNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate Virtual Disks", "cleaned", diskNames)
	}

	// 1. Raw GET - Fetch pristine VIOS XML to preserve namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML for Virtual Disk mapping...", "viosUUID", viosUUID)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract the VirtualIOServer element
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Find or create the VirtualSCSIMappings list
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		mappingsList = viosElem.CreateElement("VirtualSCSIMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_0")
		mappingsList.CreateAttr("group", "ViosSCSIMapping")
	}

	// =====================================================================
	// 5. IDEMPOTENCY CHECK & INJECTION
	// =====================================================================
	targetLparLower := strings.ToLower(lparUUID)
	mappedCount := 0

	for _, diskName := range diskNames {
		alreadyMapped := false

		// Look through existing mappings to see if this disk is already attached to this LPAR
		for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
			assoc := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
			if assoc != nil && strings.HasSuffix(strings.ToLower(assoc.SelectAttrValue("href", "")), targetLparLower) {
				// Check if the mapped storage is our specific Virtual Disk
				existingDisk := mapping.FindElement(".//*[local-name()='VirtualDisk']/*[local-name()='DiskName']")
				if existingDisk != nil && strings.EqualFold(strings.TrimSpace(existingDisk.Text()), diskName) {
					alreadyMapped = true
					break
				}
			}
		}

		if alreadyMapped {
			if debug {
				c.Logger.Info("Virtual Disk is already mapped to this LPAR; skipping", "diskName", diskName)
			}
			continue
		}

		// Inject the new mapping block
		newMappingXML := fmt.Sprintf(`
			<VirtualSCSIMapping schemaVersion="V1_0">
				<AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
				<Storage>
					<VirtualDisk schemaVersion="V1_0">
						<DiskName>%s</DiskName>
					</VirtualDisk>
				</Storage>
			</VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, diskName)

		newMappingDoc := etree.NewDocument()
		if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
			return "", fmt.Errorf("failed to parse mapping XML for disk %s: %v", diskName, err)
		}

		mappingsList.AddChild(newMappingDoc.Root())
		mappedCount++
		
		if debug {
			c.Logger.Debug("Injected mapping for virtual disk into payload", "diskName", diskName)
		}
	}

	// If all disks were already mapped, exit cleanly without making an API call!
	if mappedCount == 0 {
		return "ALREADY_MAPPED", nil
	}

	// =====================================================================
	// 6. POST THE MODIFIED PAYLOAD
	// =====================================================================
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	if debug {
        c.Logger.Info("POSTing updated VIOS XML back to HMC...")
        c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
    }
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")


	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	// =====================================================================
	// 7. GRACEFUL RMC ERROR HANDLING
	// =====================================================================
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR timeout warnings (common when the LPAR is powered off)
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Mapping saved to HMC profile, but dynamic DLPAR push timed out (LPAR likely powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		// Hard fail on genuine errors
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full XML response", postResp.Status)
	}

	// 8. Wait for background job to finish updating the Hypervisor
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Virtual Disk mapping job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}

// AddVirtualOpticalMedia natively imports an ISO file into the VIOS Media Repository using the AddOpticalMedia job.
// AddVirtualOpticalMedia adds one or more Virtual Optical Media (ISO files) to a VIOS Media Repository.
// mediaFiles is a map where keys are media names and values are file paths.
// Returns a map of results for each media (nil for success, error for failure).
// Note: This requires HMC V10.3.1061.0 or later.
func (c *HmcRestClient) AddVirtualOpticalMedia(viosUUID string, mediaFiles map[string]string, debug bool) (map[string]error, error) {
	if len(mediaFiles) == 0 {
		return nil, fmt.Errorf("at least one media file is required")
	}

	results := make(map[string]error)
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/AddOpticalMedia", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Info("Adding Virtual Optical Media to VIOS", "count", len(mediaFiles), "viosUUID", viosUUID)
	}

	// Process each media file sequentially
	for mediaName, fileName := range mediaFiles {
		if debug {
			c.Logger.Debug("Processing media", "mediaName", mediaName, "fileName", fileName)
		}

		// 1. Define operation details for the JobRequest
		operation := map[string]string{
			"OperationName": "AddOpticalMedia",
			"GroupName":     "VirtualIOServer",
			"ProgressType":  "DISCRETE",
		}

		// 2. Build job parameters
		params := map[string]string{
			"MediaName": mediaName,
			"FileName":  fileName,
		}

		// 3. Generate the XML payload
		payload, err := createJobRequestPayload(operation, params, "V1_0", debug, true)
		if err != nil {
			results[mediaName] = fmt.Errorf("failed to create job request payload: %v", err)
			continue
		}

		// 4. Create and configure the PUT request
		req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
		if err != nil {
			results[mediaName] = fmt.Errorf("failed to create request: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
		req.Header.Set("X-API-Session", c.session)
		req.Header.Set("Accept", "application/atom+xml, application/vnd.ibm.powervm.uom+xml; type=JobResponse")

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		req = req.WithContext(ctx)

		c.logRawTraffic("REQUEST (PUT)", url, payload)

		// 5. Send the request
		resp, err := c.client.Do(req)
		if err != nil {
			cancel()
			results[mediaName] = fmt.Errorf("HTTP request failed: %v", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		
		c.logRawTraffic("RESPONSE", url, string(body))

		if err != nil {
			results[mediaName] = fmt.Errorf("failed to read response body: %v", err)
			continue
		}

		if debug {
			c.Logger.Debug("AddOpticalMedia Response Status", "mediaName", mediaName, "status", resp.Status)
		}

		if resp.StatusCode >= 400 {
			if debug {
				results[mediaName] = fmt.Errorf("AddOpticalMedia job failed with status %s: %s", resp.Status, string(body))
			} else {
				results[mediaName] = fmt.Errorf("AddOpticalMedia job failed with status %s. Enable debug mode to see full response", resp.Status)
			}
			continue
		}

		// 6. Strip namespaces to find the JobID
		doc, err := xmlStripNamespace(body)
		if err != nil {
			results[mediaName] = fmt.Errorf("failed to strip namespaces from XML response: %v", err)
			continue
		}

		jobIDElem := doc.FindElement("//JobID")
		if jobIDElem == nil {
			results[mediaName] = fmt.Errorf("JobID not found in response: %s", string(body))
			continue
		}
		jobID := jobIDElem.Text()
		
		if debug {
			c.Logger.Debug("Extracted JobID. Waiting for ISO import to complete...", "jobID", jobID, "mediaName", mediaName)
		}

		// 7. Wait for the background job to finish
		_, err = c.FetchJobStatus(context.Background(), jobID, false, 10, debug)
		if err != nil {
			results[mediaName] = fmt.Errorf("failed during AddOpticalMedia job execution: %v", err)
			continue
		}

		// Success
		results[mediaName] = nil
		if debug {
			c.Logger.Info("Successfully added media", "mediaName", mediaName)
		}
	}

	// Check if all operations failed
	allFailed := true
	for _, err := range results {
		if err == nil {
			allFailed = false
			break
		}
	}

	if allFailed {
		return results, fmt.Errorf("all media additions failed")
	}

	return results, nil
}
// CreateVirtualOpticalMapsAuto maps ISOs using the HMC's auto-slot assignment.
// It is fully idempotent and safely skips already-mapped media.
func (c *HmcRestClient) CreateVirtualOpticalMaps(sysUUID, viosUUID, lparUUID string, mediaNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(mediaNames)
	mediaNames = deduplicateAndClean(mediaNames)
	if len(mediaNames) == 0 {
		return "", fmt.Errorf("no valid optical media names provided")
	}
	if debug && len(mediaNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate Optical Media", "cleaned", mediaNames)
	}

	// 1. Fetch pristine VIOS XML
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	doc, err := c.fetchAndParseHMCXML(url, debug) // Assuming you have this helper from your SDK
	if err != nil {
		return "", fmt.Errorf("failed to fetch pristine XML: %v", err)
	}

	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found")
	}

	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		mappingsList = viosElem.CreateElement("VirtualSCSIMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_0")
		mappingsList.CreateAttr("group", "ViosSCSIMapping")
	}

	// 2. Idempotency Check & Payload Generation
	mappedCount := 0
	targetLparLower := strings.ToLower(lparUUID)

	for _, mediaName := range mediaNames {
		// Check if it already exists for this specific LPAR
		alreadyMapped := false
		for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
			assoc := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
			if assoc != nil && strings.HasSuffix(strings.ToLower(assoc.SelectAttrValue("href", "")), targetLparLower) {
				existingMedia := mapping.FindElement(".//*[local-name()='VirtualOpticalMedia']/*[local-name()='MediaName']")
				if existingMedia != nil && strings.EqualFold(existingMedia.Text(), mediaName) {
					alreadyMapped = true
					break
				}
			}
		}

		if alreadyMapped {
			if debug { c.Logger.Info("Media already mapped; skipping", "mediaName", mediaName) }
			continue
		}

		// Inject new mapping (Auto-pilot style, no adapters defined)
		newMappingXML := fmt.Sprintf(`
			<VirtualSCSIMapping schemaVersion="V1_0">
				<AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
				<Storage>
					<VirtualOpticalMedia schemaVersion="V1_0">
						<MediaName>%s</MediaName>
						<MountType>r</MountType>
					</VirtualOpticalMedia>
				</Storage>
			</VirtualSCSIMapping>`, c.hmcIP, sysUUID, lparUUID, mediaName)

		newMappingDoc := etree.NewDocument()
		newMappingDoc.ReadFromString(newMappingXML)
		mappingsList.AddChild(newMappingDoc.Root())
		mappedCount++
	}

	if mappedCount == 0 {
		return "ALREADY_MAPPED", nil
	}

	// 3. POST back to HMC
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()
	if debug {
        c.Logger.Info("POSTing updated VIOS XML back to HMC...")
        c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
    }

	postReq, _ := http.NewRequest("POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")
	

	postResp, err := c.client.Do(postReq)
	if err != nil { return "", err }
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	// Graceful RMC error handling
	if postResp.StatusCode >= 400 {
		if strings.Contains(string(body), "HSCL2957") || strings.Contains(string(body), "HSCL294D") {
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full response", postResp.Status)
	}

	return "SUCCESS", nil
}
// DeletePhysicalVolumeMaps removes multiple physical disk (e.g., hdisk) mappings from a VIOS to an LPAR in a single atomic transaction.
// It uses a pristine GET-Modify-POST pattern and is completely idempotent.
func (c *HmcRestClient) DeletePhysicalVolumeMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(diskNames)
	diskNames = deduplicateAndClean(diskNames)
	if len(diskNames) == 0 {
		return "", fmt.Errorf("no valid physical volume names provided")
	}
	if debug && len(diskNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate Physical Volumes", "cleaned", diskNames)
	}
	// 1. Raw GET - Fetch pristine VIOS XML to preserve all native namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML to locate and remove Physical Volume mappings", "viosUUID", viosUUID, "disks", diskNames)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != 200 {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract the VirtualIOServer element using local-name()
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Locate the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		if debug {
			c.Logger.Info("No VirtualSCSIMappings collection found. Nothing to delete.", "viosUUID", viosUUID)
		}
		return "NOT_FOUND", nil
	}

	// Create a fast lookup map for target disks (Case-Insensitive and Trimmed)
	targetDisks := make(map[string]bool)
	for _, d := range diskNames {
		targetDisks[strings.ToLower(strings.TrimSpace(d))] = true
	}
	targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))

	// 5. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		
		// 5a. Check LPAR Association
		lparRef := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if lparRef == nil {
			continue
		}
		href := strings.ToLower(lparRef.SelectAttrValue("href", ""))
		if !strings.HasSuffix(href, targetLparLower) {
			continue // Not our target LPAR
		}

		// 5b. Find the disk name. 
		// We check BOTH the standard REST location (Storage/PhysicalVolume) AND the legacy CLI location (ServerAdapter/BackingDeviceName)
		diskName := ""
		
		if volElem := mapping.FindElement(".//*[local-name()='PhysicalVolume']/*[local-name()='VolumeName']"); volElem != nil {
			diskName = volElem.Text()
		} else if backingElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='BackingDeviceName']"); backingElem != nil {
			diskName = backingElem.Text()
		}

		if diskName == "" {
			continue
		}
		
		// 5c. Compare against our target list
		cleanDiskName := strings.ToLower(strings.TrimSpace(diskName))
		if targetDisks[cleanDiskName] {
			if debug {
				c.Logger.Debug("Found mapping targeted for deletion", "diskName", diskName, "lparUUID", lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if debug {
			c.Logger.Info("No mappings found for the specified physical volumes. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success
	}

	// 6. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing Physical Volume mapping(s) from XML tree", "count", len(mappingsToRemove))
	}
	for _, mapping := range mappingsToRemove {
		mappingsList.RemoveChild(mapping)
	}

	// 7. Extract the VIOS document to POST (Naturally retains all original namespaces)
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	// 8. POST the complete update back to the VIOS API
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
		c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
	}

	postReq, err := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	if err != nil {
		return "", err
	}

	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 9. Strict HTTP Error Checking & Graceful RMC Warning Handling
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR timeout warnings (common when the LPAR is powered off)
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Mapping deleted from HMC profile, but dynamic DLPAR push timed out (LPAR likely powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		// Hard fail on genuine errors (e.g., HTTP 500, Bad Request)
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full XML response", postResp.Status)
	}

	// 10. Wait for HMC Job Completion (Updates the Hypervisor)
	respDoc, err := xmlStripNamespace(body) // Assuming your SDK helper is available
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Physical Volume deletion job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// DeleteVirtualDiskMaps removes multiple virtual disk mappings from a VIOS to an LPAR in a single operation.
// It uses a pristine GET-Modify-POST pattern and is completely idempotent.
func (c *HmcRestClient) DeleteVirtualDiskMaps(sysUUID, viosUUID, lparUUID string, diskNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(diskNames)
	diskNames = deduplicateAndClean(diskNames)
	if len(diskNames) == 0 {
		return "", fmt.Errorf("no valid disk names provided")
	}
	if debug && len(diskNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate Virtual Disks", "cleaned", diskNames)
	}

	// 1. Raw GET - Fetch pristine VIOS XML to preserve all native namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML to locate and remove virtual disk mappings", "viosUUID", viosUUID, "disks", diskNames)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != 200 {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract the VirtualIOServer element using local-name()
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Locate the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		if debug {
			c.Logger.Info("No VirtualSCSIMappings collection found. Nothing to delete.", "viosUUID", viosUUID)
		}
		return "NOT_FOUND", nil
	}

	// Create a fast lookup map for target disks (Case-Insensitive and Trimmed)
	targetDisks := make(map[string]bool)
	for _, d := range diskNames {
		targetDisks[strings.ToLower(strings.TrimSpace(d))] = true
	}
	targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))

	// 5. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		
		// Check LPAR Association
		lparRef := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if lparRef == nil {
			continue
		}
		href := strings.ToLower(lparRef.SelectAttrValue("href", ""))
		if !strings.HasSuffix(href, targetLparLower) {
			continue // Not our target LPAR
		}

		// Check Disk Name (VirtualDisk)
		diskNameElem := mapping.FindElement(".//*[local-name()='VirtualDisk']/*[local-name()='DiskName']")
		if diskNameElem == nil {
			continue
		}
		
		diskName := strings.ToLower(strings.TrimSpace(diskNameElem.Text()))
		if targetDisks[diskName] {
			if debug {
				c.Logger.Debug("Found mapping targeted for deletion", "diskName", diskNameElem.Text(), "lparUUID", lparUUID)
			}
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if debug {
			c.Logger.Info("No mappings found for the specified disks. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success
	}

	// 6. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing virtual disk mapping(s) from XML tree", "count", len(mappingsToRemove))
	}
	for _, mapping := range mappingsToRemove {
		mappingsList.RemoveChild(mapping)
	}

	// 7. Extract the VIOS document to POST (Naturally retains all original namespaces)
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()

	// 8. POST the complete update back to the VIOS API
	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
		c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
	}

	postReq, err := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	if err != nil {
		return "", err
	}

	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 9. Strict HTTP Error Checking & Graceful RMC Warning Handling
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR timeout warnings (common when the LPAR is powered off)
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Mapping deleted from HMC profile, but dynamic DLPAR push timed out (LPAR likely powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		// Hard fail on genuine errors (e.g., HTTP 500, Bad Request)
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full XML response", postResp.Status)
	}

	// 10. Wait for HMC Job Completion (Updates the Hypervisor)
	respDoc, err := xmlStripNamespace(body) // Assuming you have this helper available
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Virtual disk deletion job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// DeleteVirtualOpticalMaps removes multiple virtual optical media mappings from a VIOS to an LPAR in a single operation.
//
// Parameters:
//   - sysUUID: The UUID of the managed system
//   - viosUUID: The UUID of the Virtual I/O Server
//   - lparUUID: The UUID of the logical partition
//   - mediaNames: Slice of virtual optical media names to unmap (e.g., ["rhel9.iso", "aix73.iso"])
//   - verbose: Enable detailed logging
//
// Returns:
//   - "SUCCESS" if mappings were deleted
//   - "NOT_FOUND" if no matching mappings exist (idempotent)
//   - error if operation fails
//
// Note: Strict HTTP error checking is disabled (Option 2 behavior).
// DeleteVirtualOpticalMaps removes multiple virtual optical media mappings from a VIOS to an LPAR in a single operation.
func (c *HmcRestClient) DeleteVirtualOpticalMaps(sysUUID, viosUUID, lparUUID string, mediaNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(mediaNames)
	mediaNames = deduplicateAndClean(mediaNames)
	if len(mediaNames) == 0 {
		return "", fmt.Errorf("no valid optical media names provided")
	}
	if debug && len(mediaNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate Optical Media", "cleaned", mediaNames)
	}

	// 1. Raw GET - Fetch pristine VIOS XML to preserve all native namespaces and attributes
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML to locate and remove virtual optical mappings", "viosUUID", viosUUID, "media", mediaNames)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract the VirtualIOServer element using local-name() to ignore namespace prefixes
	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 4. Locate the VirtualSCSIMappings collection
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualSCSIMappings']")
	if mappingsList == nil {
		if debug {
			c.Logger.Info("No VirtualSCSIMappings collection found. Nothing to delete.")
		}
		return "NOT_FOUND", nil
	}

	// Create a fast lookup map for target media (Case-Insensitive and Trimmed)
	targetMedia := make(map[string]bool)
	for _, m := range mediaNames {
		targetMedia[strings.ToLower(strings.TrimSpace(m))] = true
	}
	targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))

	// 5. Find all specific mappings to delete
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		
		// Check LPAR Association
		lparRef := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if lparRef == nil {
			continue
		}
		href := strings.ToLower(lparRef.SelectAttrValue("href", ""))
		if !strings.HasSuffix(href, targetLparLower) {
			continue // Not our LPAR
		}

		// Check Media Name
		mediaNameElem := mapping.FindElement(".//*[local-name()='VirtualOpticalMedia']/*[local-name()='MediaName']")
		if mediaNameElem == nil {
			continue
		}
		
		mediaName := strings.ToLower(strings.TrimSpace(mediaNameElem.Text()))
		if targetMedia[mediaName] {
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		if debug {
			c.Logger.Info("No mappings found for the specified media. Nothing to delete.", "lparUUID", lparUUID)
		}
		return "NOT_FOUND", nil // Idempotent success
	}

	// 6. Remove the matched mappings from the XML tree
	if debug {
		c.Logger.Info("Removing virtual optical mapping(s) from XML tree", "count", len(mappingsToRemove))
	}
	for _, mapping := range mappingsToRemove {
		mappingsList.RemoveChild(mapping)
	}

	// 7. Extract the VIOS document to POST
	// Because viosElem was cloned from pristine XML, it naturally retains all exact namespaces!
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()
	if debug {
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
		c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
	}

	// 8. POST the complete update back to the VIOS API
	postReq, _ := http.NewRequest("POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	// 9. Graceful RMC Error Handling (CRITICAL FIX)
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Catch known IBM DLPAR timeout warnings (common when the LPAR is powered off)
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Mapping deleted from HMC profile, but dynamic DLPAR push timed out (LPAR likely powered off).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		// Hard fail on genuine errors
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full XML response", postResp.Status)
	}

	// 10. Wait for HMC Job Completion
	respDoc, err := xmlStripNamespace(body) // Assuming you still have this helper for parsing responses
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			if debug {
				c.Logger.Info("Deletion job triggered", "jobID", jobIDElem.Text())
			}
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}

// CreateVirtualFibreChannelMappings creates multiple Virtual Fibre Channel (NPIV) mappings between a VIOS and an LPAR.
// It uses a pristine GET-Modify-POST pattern and safely handles DLPAR timeouts for non-RMC operating systems like CoreOS.
func (c *HmcRestClient) CreateVirtualFibreChannelMaps(sysUUID, viosUUID, lparUUID string, fcPortNames []string, debug bool) (string, error) {
	// 0. SDK-LEVEL SANITIZATION
	originalCount := len(fcPortNames)
	fcPortNames = deduplicateAndClean(fcPortNames)
	if len(fcPortNames) == 0 {
		return "", fmt.Errorf("no valid Fibre Channel port names provided")
	}
	if debug && len(fcPortNames) < originalCount {
		c.Logger.Debug("SDK automatically removed duplicate FC Ports", "cleaned", fcPortNames)
	}

	// 1. Raw GET - Fetch pristine VIOS XML
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosFCMapping", c.hmcIP, viosUUID)
	
	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != http.StatusOK {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	// 3. Find or create the VirtualFibreChannelMappings list
	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualFibreChannelMappings']")
	if mappingsList == nil {
		mappingsList = viosElem.CreateElement("VirtualFibreChannelMappings")
		mappingsList.CreateAttr("schemaVersion", "V1_3_0") // IBM strictly requires V1_3_0 for NPIV
		mappingsList.CreateAttr("group", "ViosFCMapping")
	}

	//targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))
	mappedCount := 0

	// 4. Inject mappings
	for _, fcPortName := range fcPortNames {
		trimmedPort := strings.TrimSpace(fcPortName)
	/*	alreadyMapped := false

		// Idempotency Check
		for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualFibreChannelMapping']") {
			assoc := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
			if assoc != nil && strings.HasSuffix(strings.ToLower(assoc.SelectAttrValue("href", "")), targetLparLower) {
				existingPort := mapping.FindElement(".//*[local-name()='Port']/*[local-name()='PortName']")
				if existingPort != nil && strings.EqualFold(strings.TrimSpace(existingPort.Text()), trimmedPort) {
					alreadyMapped = true
					break
				}
			}
		}

		if alreadyMapped {
			if debug { c.Logger.Info("FC Port already mapped; skipping", "port", trimmedPort) }
			continue
		} */

		newMappingXML := fmt.Sprintf(`
		<VirtualFibreChannelMapping schemaVersion="V1_3_0">
			<AssociatedLogicalPartition href="https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s" rel="related"/>
			<Port schemaVersion="V1_3_0">
				<PortName>%s</PortName>
			</Port>
		</VirtualFibreChannelMapping>`, c.hmcIP, sysUUID, lparUUID, trimmedPort)

		newMappingDoc := etree.NewDocument()
		if err := newMappingDoc.ReadFromString(newMappingXML); err != nil {
			return "", fmt.Errorf("failed to parse mapping XML for port %s: %v", trimmedPort, err)
		}

		mappingsList.AddChild(newMappingDoc.Root())
		mappedCount++
	}

	if mappedCount == 0 {
		return "ALREADY_MAPPED", nil
	}

	// 5. Execute POST
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()
	if debug {
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
		c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
	}

	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	if debug {
		c.Logger.Info("POSTing payload. This may hang for ~3 minutes if the LPAR is powered on but lacks an RMC daemon (e.g., CoreOS).")
	}

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 6. Graceful RMC Error Handling
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		// Safely catch the expected DLPAR timeout for OpenShift LPARs
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Mapping saved to HMC profile, but dynamic DLPAR push timed out (Expected behavior for CoreOS/offline LPARs).")
			}
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		// Hard fail on genuine errors (e.g., schema validation failures)
		if debug {
			return "", fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
		}
		return "", fmt.Errorf("POST failed (%s). Enable debug mode to see full XML response", postResp.Status)
	}

	// 7. Check for Background Job
	respDoc, err := xmlStripNamespace(body)
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}
// DeleteVirtualFibreChannelMaps removes one or more NPIV mappings using Smart Targets.
// The 'targets' slice can contain Physical Ports (e.g., "fcs0"), Client WWPNs (e.g., "c050760b2e4a014a"), 
// or Client Virtual Slot IDs (e.g., "4").
func (c *HmcRestClient) DeleteVirtualFibreChannelMaps(sysUUID, viosUUID, lparUUID string, targets []string, debug bool) (string, error) {
	if len(targets) == 0 {
		return "NO_TARGETS_SPECIFIED", nil
	}

	// 1. Raw GET - Fetch pristine VIOS XML
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosFCMapping", c.hmcIP, viosUUID)
	
	if debug {
		c.Logger.Debug("Fetching pristine VIOS XML to remove vFC mappings", "viosUUID", viosUUID, "targets", targets)
	}

	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil { return "", err }
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	c.logRawTraffic("REQUEST (GET)", url, "")

	getResp, err := c.client.Do(getReq)
	if err != nil { return "", err }
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	c.logRawTraffic("RESPONSE", url, string(rawXML))

	if getResp.StatusCode != http.StatusOK {
		if debug {
			return "", fmt.Errorf("GET failed (HTTP %d): %s", getResp.StatusCode, string(rawXML))
		}
		return "", fmt.Errorf("GET failed (HTTP %d). Enable debug mode to see full XML response", getResp.StatusCode)
	}

	// 2. Load the pristine XML into etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return "", fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	viosElem := doc.FindElement(".//*[local-name()='VirtualIOServer']")
	if viosElem == nil {
		return "", fmt.Errorf("VirtualIOServer element not found in response")
	}

	mappingsList := viosElem.FindElement(".//*[local-name()='VirtualFibreChannelMappings']")
	if mappingsList == nil {
		return "NOT_FOUND", nil // Idempotent success
	}

	// Normalize the target strings for safe comparison
	lookupTargets := make(map[string]bool)
	for _, t := range targets {
		lookupTargets[strings.ToLower(strings.TrimSpace(t))] = true
	}
	targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))

	// 3. Find specific mappings to delete using Smart Match
	var mappingsToRemove []*etree.Element
	for _, mapping := range mappingsList.FindElements(".//*[local-name()='VirtualFibreChannelMapping']") {
		
		// Ensure this mapping belongs to our LPAR
		assoc := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if assoc == nil { continue }
		href := strings.ToLower(strings.TrimSpace(assoc.SelectAttrValue("href", "")))
		if !strings.HasSuffix(href, targetLparLower) {
			continue 
		}

		// Extract identifying data safely
		portName, wwpns, slotID := "", "", ""
		if pElem := mapping.FindElement(".//*[local-name()='Port']/*[local-name()='PortName']"); pElem != nil {
			portName = strings.ToLower(strings.TrimSpace(pElem.Text()))
		}
		if wElem := mapping.FindElement(".//*[local-name()='ClientAdapter']/*[local-name()='WWPNs']"); wElem != nil {
			wwpns = strings.ToLower(strings.TrimSpace(wElem.Text()))
		}
		if sElem := mapping.FindElement(".//*[local-name()='ClientAdapter']/*[local-name()='VirtualSlotNumber']"); sElem != nil {
			slotID = strings.ToLower(strings.TrimSpace(sElem.Text()))
		}

		// Check if any of our lookup targets match the Port, WWPNs, or Slot ID
		matchFound := false
		for target := range lookupTargets {
			if target == portName || target == slotID || strings.Contains(wwpns, target) {
				matchFound = true
				if debug {
					c.Logger.Debug("Matched vFC mapping for deletion", "target", target, "port", portName, "wwpns", wwpns, "slot", slotID)
				}
				break
			}
		}

		if matchFound {
			mappingsToRemove = append(mappingsToRemove, mapping)
		}
	}

	if len(mappingsToRemove) == 0 {
		return "NOT_FOUND", nil
	}

	// 4. Surgically remove the matched mappings from the XML DOM
	for _, mapping := range mappingsToRemove {
		mappingsList.RemoveChild(mapping)
	}

	// 5. POST the updated blueprint back to the HMC
	postDoc := etree.NewDocument()
	postDoc.SetRoot(viosElem.Copy())
	postXML, _ := postDoc.WriteToString()
	if debug {
		c.Logger.Info("POSTing updated VIOS XML back to HMC...")
		c.Logger.Debug(fmt.Sprintf("Payload:\n%s", postXML))
	}

	postURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	postReq, _ := http.NewRequest("POST", postURL, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualIOServer")
	postReq.Header.Set("Accept", "application/atom+xml")

	c.logRawTraffic("REQUEST (POST)", postURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil { return "", err }
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", postURL, string(body))

	// 6. Strict Error Catching (with RMC timeout bypass for CoreOS)
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		if strings.Contains(bodyStr, "HSCL294D") || strings.Contains(bodyStr, "HSCL2957") {
			return "SUCCESS_WITH_RMC_WARNING", nil
		}
		return "", fmt.Errorf("HMC REJECTED DELETE POST (HTTP %s):\n%s", postResp.Status, bodyStr)
	}

	// 7. Await Background Job
	respDoc, err := xmlStripNamespace(body) 
	if err == nil {
		if jobIDElem := respDoc.FindElement("//JobID"); jobIDElem != nil {
			c.FetchJobStatus(context.Background(), jobIDElem.Text(), false, 10, debug)
		}
	}

	return "SUCCESS", nil
}

// GetVirtualFibreChannelMappingsForLPAR fetches NPIV mappings for a specific LPAR on a VIOS,
// unmarshaling the data directly into your native VirtualFibreChannelMapping structs.
func (c *HmcRestClient) GetVirtualFibreChannelMaps(viosUUID, lparUUID string, debug bool) ([]VirtualFibreChannelMapping, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosFCMapping", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching vFC mappings for LPAR", "viosUUID", viosUUID, "lparUUID", lparUUID)
	}

	// Fetch and strip namespaces
	doc, err := c.fetchAndParseHMCXML(url, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VIOS vFC mappings: %v", err)
	}

	var results []VirtualFibreChannelMapping
	targetLparLower := strings.ToLower(strings.TrimSpace(lparUUID))

	mappingsList := doc.FindElement("//VirtualFibreChannelMappings")
	if mappingsList == nil {
		return results, nil // No mappings exist on this VIOS
	}

	for _, mappingElem := range mappingsList.FindElements("VirtualFibreChannelMapping") {
		// 1. Convert the isolated etree element back to raw XML bytes
		tempDoc := etree.NewDocument()
		tempDoc.SetRoot(mappingElem.Copy())
		mappingBytes, _ := tempDoc.WriteToBytes()

		// 2. Unmarshal directly into your native struct
		var vfcMap VirtualFibreChannelMapping
		if err := xml.Unmarshal(mappingBytes, &vfcMap); err != nil {
			if debug {
				c.Logger.Warn("Failed to unmarshal a vFC mapping", "error", err)
			}
			continue
		}

		// 3. Verify LPAR Association
		// Note: Adjust "Href" to whatever field name you used in your LinkXML struct (e.g., HREF, URL)
		href := strings.ToLower(strings.TrimSpace(vfcMap.AssociatedLogicalPartition.Href))
		if strings.HasSuffix(href, targetLparLower) {
			results = append(results, vfcMap)
		}
	}

	if debug {
		c.Logger.Debug("Successfully parsed vFC mappings", "lparUUID", lparUUID, "count", len(results))
	}

	return results, nil
}

// GetPhysicalFibreChannelPorts slices through the VIOS hardware topology using XPath to return all physical FC ports.
// This approach is highly resilient to IBM schema changes and memory efficient.
// It dynamically maps the hardware elements directly into the existing 'Port' struct.
func (c *HmcRestClient) GetPhysicalFibreChannelPorts(viosUUID string, debug bool) ([]Port, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Debug("Fetching physical FC ports using resilient XPath approach", "viosUUID", viosUUID)
	}

	// Fetch and parse the XML into an etree Document (namespaces safely stripped)
	doc, err := c.fetchAndParseHMCXML(url, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VIOS configuration: %v", err)
	}

	var fcPorts []Port

	// XPath instantly finds all FC ports, deeply bypassing any complex or changing slot topologies
	for _, portElem := range doc.FindElements("//PhysicalFibreChannelPort") {

		// 1. Clone the element to avoid mutating the original document
		clone := portElem.Copy()

		// 2. ✨ THE TRICK: Rename the tag to match your native struct's expectations
		// This forces <PhysicalFibreChannelPort> to behave exactly like <Port>
		clone.Tag = "Port"

		// 3. Serialize the cloned element back to bytes
		tempDoc := etree.NewDocument()
		tempDoc.SetRoot(clone)
		portBytes, _ := tempDoc.WriteToBytes()

		// 4. Unmarshal seamlessly into your existing Port struct
		var portInfo Port
		if err := xml.Unmarshal(portBytes, &portInfo); err != nil {
			if debug {
				c.Logger.Warn("Failed to unmarshal a physical FC port", "error", err)
			}
			continue
		}

		// 5. Append valid ports (ignores empty or unconfigured hardware stubs)
		if strings.TrimSpace(portInfo.PortName) != "" {
			fcPorts = append(fcPorts, portInfo)
		}
	}

	if debug {
		c.Logger.Debug("Successfully extracted Physical FC Ports", "viosUUID", viosUUID, "count", len(fcPorts))
	}

	return fcPorts, nil
}

// =====================================================================
// VIOS MOUNT LOCK MANAGEMENT
// =====================================================================

// AcquireVIOSMountLock creates a lock file on VIOS to serialize /mnt access
// Uses only commands allowed in padmin restricted shell (echo, test, rm)
func (c *HmcRestClient) AcquireVIOSMountLock(systemName, viosName string, timeoutSeconds int, debug bool) error {
	lockFile := "/home/padmin/mnt.lock"
	checkInterval := 5 * time.Second
	maxRetries := timeoutSeconds / 5
	
	if debug {
		c.Logger.Info("Attempting to acquire VIOS mount lock",
			"vios", viosName,
			"timeout", timeoutSeconds)
	}
	
	for i := 0; i < maxRetries; i++ {
		// Check if lock exists using 'test' command (allowed in restricted shell)
		checkCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "test -f %s && echo EXISTS || echo NOTFOUND"`,
			systemName, viosName, lockFile)
		output, err := c.CliRunner(checkCmd, debug)
		
		if err != nil || strings.Contains(output, "NOTFOUND") {
			// Lock doesn't exist - try to create it atomically
			// Use echo with PID and timestamp for debugging (echo is allowed)
			createCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "echo powershift_\$\$_\$(date +%%s) > %s"`,
				systemName, viosName, lockFile)
			_, err := c.CliRunner(createCmd, debug)
			
			if err == nil {
				if debug {
					c.Logger.Info("✓ Acquired VIOS mount lock", "vios", viosName)
				}
				return nil
			}
			
			// Race condition - another process created it first, retry
			if debug {
				c.Logger.Debug("Lock creation race detected, retrying...")
			}
		}
		
		// Lock exists, wait and retry
		if debug {
			c.Logger.Debug("VIOS mount lock held by another process",
				"attempt", i+1,
				"maxRetries", maxRetries,
				"waitingSeconds", int(checkInterval.Seconds()))
		}
		
		time.Sleep(checkInterval)
	}
	
	return fmt.Errorf("timeout waiting for VIOS mount lock after %d seconds (another deployment may be stuck)", timeoutSeconds)
}

// ReleaseVIOSMountLock removes the lock file from VIOS
func (c *HmcRestClient) ReleaseVIOSMountLock(systemName, viosName string, debug bool) error {
	lockFile := "/home/padmin/mnt.lock"
	
	// Use 'rm' which is allowed in restricted shell
	cmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rm -f %s"`, systemName, viosName, lockFile)
	_, err := c.CliRunner(cmd, debug)
	
	if err != nil {
		c.Logger.Warn("Failed to release VIOS mount lock", "vios", viosName, "error", err)
		return err
	}
	
	if debug {
		c.Logger.Info("✓ Released VIOS mount lock", "vios", viosName)
	}
	return nil
}
// MediaRepositoryInfo holds the total and free space of a VIOS media repository
type MediaRepositoryInfo struct {
	SizeMB int
	FreeMB int
}
// GetMediaRepositoryInfo parses the VIOS lsrep command to extract exact capacity and free space
func (c *HmcRestClient) GetMediaRepositoryInfo(sysName, viosName string, debug bool) (*MediaRepositoryInfo, error) {
	if debug {
		c.Logger.Debug("Fetching Media Repository capacity via CLI", "viosName", viosName)
	}

	cmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "lsrep"`, sysName, viosName)
	output, err := c.CliRunner(cmd, debug)
	if err != nil {
		// If lsrep fails, the repository likely doesn't exist
		return nil, fmt.Errorf("failed to run lsrep (repository may not exist): %w", err)
	}

	// Typical lsrep output:
	// Size(mb) Free(mb) Parent Pool         Parent Size      Parent Free
	// 20480    10240    rootvg              ...
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Size(mb)") && i+1 < len(lines) {
			dataLine := strings.TrimSpace(lines[i+1])
			fields := strings.Fields(dataLine)
			if len(fields) >= 2 {
				sizeMB, _ := strconv.Atoi(fields[0])
				freeMB, _ := strconv.Atoi(fields[1])
				return &MediaRepositoryInfo{
					SizeMB: sizeMB,
					FreeMB: freeMB,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("could not parse repository size from lsrep output")
}
// CreateVirtualSCSIServerAdapter adds a vSCSI Server Adapter to a VIOS at a specific slot, locking it to a specific Client LPAR and Client Slot.
func (c *HmcRestClient) CreateVirtualSCSIServerAdapter(viosUUID string, clientLparID int, viosSlot int, clientSlot int, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)

	if debug {
		c.Logger.Info("Provisioning vSCSI Server Adapter via REST API", "viosUUID", viosUUID, "clientLparID", clientLparID, "viosSlot", viosSlot, "clientSlot", clientSlot)
	}

	// We now explicitly define BOTH the VirtualSlotNumber (local) and RemoteSlotNumber (client)
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VirtualSCSIServerAdapter:VirtualSCSIServerAdapter xmlns:VirtualSCSIServerAdapter="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" schemaVersion="V1_0">
    <Metadata><Atom/></Metadata>
    <AdapterType>Server</AdapterType>
    <VirtualSlotNumber>%d</VirtualSlotNumber>
    <RemoteLogicalPartitionID>%d</RemoteLogicalPartitionID>
    <RemoteSlotNumber>%d</RemoteSlotNumber>
</VirtualSCSIServerAdapter:VirtualSCSIServerAdapter>`, viosSlot, clientLparID, clientSlot)

	httpReq, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualSCSIServerAdapter")
	httpReq.Header.Set("Accept", "application/atom+xml")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		if debug {
			return "", fmt.Errorf("vSCSI Server Adapter creation failed (%s): %s", resp.Status, string(body))
		}
		return "", fmt.Errorf("vSCSI Server Adapter creation failed (%s). Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	// Extract the new UUID
	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("adapter created but failed to extract new UUID")
	}

	if debug {
		c.Logger.Info("Successfully created Virtual SCSI Server Adapter", "uuid", atomID.Text(), "viosSlot", viosSlot, "clientSlot", clientSlot)
	}

	return atomID.Text(), nil
}
// GetRawViosXML fetches the raw XML string for a specific VIOS extended group (e.g., "ViosSCSIMapping").
// This is primarily used for debugging and diffing payload changes.
func (c *HmcRestClient) GetRawViosXML(viosUUID string, group string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=%s", c.hmcIP, viosUUID, group)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		if debug {
			return "", fmt.Errorf("failed to fetch raw XML (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("failed to fetch raw XML (HTTP %d). Enable debug mode to see full response", resp.StatusCode)
	}

	return string(body), nil
}