package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// / ConfigDevice submits a job request to configure a device on a Virtual I/O Server.
// If devName is empty, it attempts to configure all devices.
// It waits for the job to complete and checks for success.
func (c *HmcRestClient) ConfigDevice(viosID string, devName string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/ConfigDevice", c.hmcIP, viosID)
	if verbose {
		hmcLogger.Printf("Submitting ConfigDevice job for VIOS ID %s, URL: %s", viosID, url)
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
	payload, err := createJobRequestPayload(operation, params, schemaVersion, verbose, includeJobParamSchema)
	if err != nil {
		return fmt.Errorf("failed to create JobRequest payload: %v", err)
	}

	if verbose {
		hmcLogger.Printf("JobRequest XML:\n%s", payload)
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
		hmcLogger.Printf("ConfigDevice response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("ConfigDevice response body:\n%s", string(body))
	}

	// Check for non-success status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("ConfigDevice job submission failed with status %s: %s", resp.Status, string(body))
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
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Fetch the job response
	jobDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	var jobDocStr string
	if verbose {
		jobDocStr, _ = jobDoc.WriteToString()
		hmcLogger.Printf("ConfigDevice job response:\n%s", jobDocStr)
	}

	// Check job status
	statusElem := jobDoc.FindElement("//Status")
	if statusElem == nil || statusElem.Text() != "COMPLETED_OK" {
		messageElem := jobDoc.FindElement("//Message")
		msg := ""
		if messageElem != nil {
			msg = messageElem.Text()
		}
		return fmt.Errorf("job failed: status %s, message: %s", statusElem.Text(), msg)
	}

	// Optionally check StdError
	stdErrorElem := jobDoc.FindElement("//StdError")
	if stdErrorElem != nil && stdErrorElem.Text() != "" {
		return fmt.Errorf("config device error: %s", stdErrorElem.Text())
	}

	return nil
}

// GetVirtualIOServersQuick retrieves the list of Virtual I/O Servers for a given managed system UUID
func (c *HmcRestClient) GetVirtualIOServersQuick(systemUUID string, verbose bool) ([]VIOS, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer/quick/All", c.hmcIP, systemUUID)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("GetVirtualIOServersQuick failed with status: %s", resp.Status)
		}
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetVirtualIOServersQuick response body:\n%s", string(body))
	}

	var viosList []VIOS
	if err := json.Unmarshal(body, &viosList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %v", err)
	}

	return viosList, nil
}

// GetFreePhyVolume retrieves free physical volumes for a given VIOS UUID
func (c *HmcRestClient) GetFreePhyVolume(viosUUID string, verbose bool) ([]PhysicalVolume, error) {
	if verbose {
		hmcLogger.Printf("VIOS UUID: %s", viosUUID)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_3_0", verbose, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("Job request payload:\n%s", payload)
	}
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/GetFreePhysicalVolumes", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Requesting free physical volumes for VIOS UUID %s, URL: %s", viosUUID, url)
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
	if verbose {
		hmcLogger.Printf("Request headers: %+v", req.Header)
	}

	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log the response status and body
	if verbose {
		hmcLogger.Printf("GetFreePhyVolume response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	if verbose {
		hmcLogger.Printf("GetFreePhyVolume response body:\n%s", string(body))
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
			return nil, fmt.Errorf("HMC error: %s, status: %s, body: %s", errorMsgs[0].Text(), resp.Status, string(body))
		}
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
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Fetch the job response
	pvDoc, err := c.FetchJobStatus(jobID, false, 10, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job response: %v", err)
	}

	// Log the job response
	var pvDocStr string
	if verbose {
		pvDocStr, _ = pvDoc.WriteToString()
		hmcLogger.Printf("Free Physical Volume job response:\n%s", pvDocStr)
	}
	// Extract the result XML from the job response
	resultElem := pvDoc.FindElement("//Results/JobParameter/ParameterValue")
	if verbose {
		if resultElem != nil {
			hmcLogger.Printf("resultElem content: %s", resultElem.Text())
		} else {
			hmcLogger.Printf("resultElem is nil: no ParameterValue found for ParameterName 'result'")
		}
	}
	if resultElem == nil {
		return nil, fmt.Errorf("result not found in job response: %s", pvDocStr)
	}
	pvXML := resultElem.Text()

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
		if verbose {
			hmcLogger.Printf("No free physical volumes found for VIOS UUID %s", viosUUID)
		}
		// Return an empty list instead of an error, as no volumes is a valid case
		return listPv, nil
	}
	if verbose {
		hmcLogger.Printf("Found %d free physical volumes for VIOS UUID %s", len(listPv), viosUUID)
		for i, pv := range listPv {
			hmcLogger.Printf("Physical Volume %d: Name=%s, VolumeUniqueID=%s, UniqueDeviceID=%s, StorageLabel=%s, Capacity=%d, LocationCode=%s, State=%s", i+1, pv.VolumeName, pv.VolumeUniqueID, pv.UniqueDeviceID, pv.StorageLabel, pv.VolumeCapacity, pv.LocationCode, pv.VolumeState)
		}
	}
	return listPv, nil
}

// GetViosSCSIMappings retrieves the VSCSI mappings for a VIOS using the extended group.
func (c *HmcRestClient) GetViosSCSIMappings(viosUUID string, verbose bool) ([]*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Fetching VSCSI mappings for VIOS UUID %s, URL: %s", viosUUID, url)
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
		hmcLogger.Printf("GetViosSCSIMappings response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetViosSCSIMappings response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	mappings := doc.FindElements("//VirtualSCSIMapping")
	if verbose {
		hmcLogger.Printf("Found %d VSCSI mappings for VIOS %s", len(mappings), viosUUID)
	}

	return mappings, nil
}
// RemoveVolumeLPARMapping completely removes the storage mapping by deleting the Client Adapter (unmaps disk), then deleting the Server Adapter (cleans VIOS).
func (c *HmcRestClient) RemoveVolumeLPARMapping(viosUUID, lparUUID, volumeName string, verbose bool) error {
	// =====================================================================
	// STEP 1: Find the Client and Server Slot Numbers from the Mapping
	// =====================================================================
	mappingsURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
	mappingsDoc, err := c.fetchAndParseHMCXML(mappingsURL, verbose)
	if err != nil {
		return err
	}

	var clientSlotNum, serverSlotNum string
	targetLparLower := strings.ToLower(lparUUID)

	for _, mapping := range mappingsDoc.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if assocLpar == nil { continue }

		href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
		if strings.HasSuffix(href, targetLparLower) {
			
			backingDevElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='BackingDeviceName']")
			storageVolElem := mapping.FindElement(".//*[local-name()='Storage']/*[local-name()='PhysicalVolume']/*[local-name()='VolumeName']")

			vName := ""
			if backingDevElem != nil && backingDevElem.Text() != "" {
				vName = backingDevElem.Text()
			} else if storageVolElem != nil && storageVolElem.Text() != "" {
				vName = storageVolElem.Text()
			}

			clientSlotElem := mapping.FindElement(".//*[local-name()='ClientAdapter']/*[local-name()='VirtualSlotNumber']")
			cSlot := ""
			if clientSlotElem != nil { cSlot = clientSlotElem.Text() }

			// Match either the real volume name OR our special empty slot tag
			isMatch := false
			if vName != "" && vName == volumeName {
				isMatch = true
			} else if vName == "" && volumeName == ("EMPTY_VSCSI_SLOT_" + cSlot) {
				isMatch = true
			}

			if isMatch {
				clientSlotNum = cSlot
				serverSlotElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='VirtualSlotNumber']")
				if serverSlotElem != nil { serverSlotNum = serverSlotElem.Text() }
				break
			}
		}
	}

	if clientSlotNum == "" {
		return fmt.Errorf("could not find mapping for %s on VIOS %s", volumeName, viosUUID)
	}

	if verbose {
		hmcLogger.Printf("Target %s is mapped via Client Slot %s <-> Server Slot %s", volumeName, clientSlotNum, serverSlotNum)
	}

	// =====================================================================
	// STEP 2: Resolve the DELETE URLs for both Adapters
	// =====================================================================
	var clientAdapterDeleteURL, serverAdapterDeleteURL string

	// Get Client Adapter URL
	clientAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)
	clientAdaptersDoc, _ := c.fetchAndParseHMCXML(clientAdaptersURL, verbose)
	if clientAdaptersDoc != nil {
		for _, entry := range clientAdaptersDoc.FindElements("//entry") {
			if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil && slot.Text() == clientSlotNum {
				for _, link := range entry.FindElements("./link") {
					if link.SelectAttrValue("rel", "") == "SELF" {
						clientAdapterDeleteURL = link.SelectAttrValue("href", "")
						break
					}
				}
				break
			}
		}
	}

	// Get Server Adapter URL
	if serverSlotNum != "" {
		serverAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
		serverAdaptersDoc, _ := c.fetchAndParseHMCXML(serverAdaptersURL, verbose)
		if serverAdaptersDoc != nil {
			for _, entry := range serverAdaptersDoc.FindElements("//entry") {
				if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil && slot.Text() == serverSlotNum {
					for _, link := range entry.FindElements("./link") {
						if link.SelectAttrValue("rel", "") == "SELF" {
							serverAdapterDeleteURL = link.SelectAttrValue("href", "")
							break
						}
					}
					break
				}
			}
		}
	}

	// =====================================================================
	// STEP 3: EXECUTE STAGE 1 - Delete Client Adapter (Unmaps Disk)
	// =====================================================================
	if clientAdapterDeleteURL != "" {
		if verbose { hmcLogger.Printf("Deleting LPAR Client Adapter (Slot %s)...", clientSlotNum) }
		reqDelClient, _ := http.NewRequest("DELETE", clientAdapterDeleteURL, nil)
		reqDelClient.Header.Set("X-API-Session", c.session)
		respDelClient, err := c.client.Do(reqDelClient)
		if err == nil {
			respDelClient.Body.Close()
		}

		// CRITICAL: We must wait for the VIOS to finish destroying the vtscsi device
		// before we attempt to delete the vhost. 
		if verbose { hmcLogger.Printf("Waiting 10 seconds for VIOS to release device locks...") }
		time.Sleep(10 * time.Second)
	}

	// =====================================================================
	// STEP 4: EXECUTE STAGE 2 - Delete Server Adapter (Cleans VIOS vhost)
	// =====================================================================
	if serverAdapterDeleteURL != "" {
		if verbose { hmcLogger.Printf("Deleting VIOS Server Adapter (Slot %s)...", serverSlotNum) }
		reqDelServer, _ := http.NewRequest("DELETE", serverAdapterDeleteURL, nil)
		reqDelServer.Header.Set("X-API-Session", c.session)
		respDelServer, err := c.client.Do(reqDelServer)
		if err == nil {
			respDelServer.Body.Close()
		}
	}

	if verbose {
		hmcLogger.Printf("✅ Successfully completely removed mapping architecture for %s", volumeName)
	}

	return nil
}

// removeDeviceViaJob removes a specific device from VIOS using the RemoveDevice job operation
func (c *HmcRestClient) removeDeviceViaJob(viosUUID, deviceName string, verbose bool) error {
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

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", verbose, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice job submission failed with status %s", resp.Status)
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		_, err = c.FetchJobStatus(jobIDElem.Text(), false, 5, verbose)
		return err
	}
	return fmt.Errorf("JobID not found")
}