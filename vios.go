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
	
	if verbose {
		hmcLogger.Printf("Fetching VSCSI mappings for VIOS UUID %s, URL: %s", viosUUID, mappingsURL)
	}
	
	mappingsDoc, err := c.fetchAndParseHMCXML(mappingsURL, verbose)
	if err != nil {
		return err
	}

	// Print the entire XML document if verbose is enabled
	if verbose && mappingsDoc != nil {
		docStr, _ := mappingsDoc.WriteToString()
		hmcLogger.Printf("ViosSCSIMapping XML response body:\n%s", docStr)
	}

	var clientSlotNum, serverSlotNum string
	targetLparLower := strings.ToLower(lparUUID)

	for _, mapping := range mappingsDoc.FindElements(".//*[local-name()='VirtualSCSIMapping']") {
		assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
		if assocLpar == nil { continue }

		href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
		
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

		serverSlotElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='VirtualSlotNumber']")
		sSlot := ""
		if serverSlotElem != nil { sSlot = serverSlotElem.Text() }

		// Verbose logging of ALL parsed XML values for every mapping evaluated
		if verbose {
			hmcLogger.Printf("Evaluating Mapping -> LPAR href: %s | VolumeName: %s | ClientSlot: %s | ServerSlot: %s", href, vName, cSlot, sSlot)
		}

		if strings.HasSuffix(href, targetLparLower) {
			// Match either the real volume name OR our special empty slot tag
			isMatch := false
			if vName != "" && vName == volumeName {
				isMatch = true
			} else if vName == "" && volumeName == ("EMPTY_VSCSI_SLOT_" + cSlot) {
				isMatch = true
			}

			if isMatch {
				clientSlotNum = cSlot
				serverSlotNum = sSlot
				if verbose {
					hmcLogger.Printf("--> MATCH FOUND! Will delete Client Slot: %s and Server Slot: %s", clientSlotNum, serverSlotNum)
				}
				break
			}
		}
	}

	if clientSlotNum == "" {
		return fmt.Errorf("could not find mapping for %s on VIOS %s", volumeName, viosUUID)
	}

	// =====================================================================
	// STEP 2: Resolve the DELETE URLs for both Adapters
	// =====================================================================
	var clientAdapterDeleteURL, serverAdapterDeleteURL string

	// Get Client Adapter URL
	clientAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/VirtualSCSIClientAdapter", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching VirtualSCSIClientAdapters for LPAR UUID %s, URL: %s", lparUUID, clientAdaptersURL)
	}
	
	clientAdaptersDoc, _ := c.fetchAndParseHMCXML(clientAdaptersURL, verbose)
	if clientAdaptersDoc != nil {
		if verbose {
			docStr, _ := clientAdaptersDoc.WriteToString()
			hmcLogger.Printf("VirtualSCSIClientAdapter XML response body:\n%s", docStr)
		}

		for _, entry := range clientAdaptersDoc.FindElements("//entry") {
			if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil {
				if verbose {
					hmcLogger.Printf("Found Client Adapter in XML with VirtualSlotNumber: %s", slot.Text())
				}
				if slot.Text() == clientSlotNum {
					for _, link := range entry.FindElements("./link") {
						if link.SelectAttrValue("rel", "") == "SELF" {
							clientAdapterDeleteURL = link.SelectAttrValue("href", "")
							if verbose {
								hmcLogger.Printf("--> Resolved Client Adapter DELETE URL: %s", clientAdapterDeleteURL)
							}
							break
						}
					}
					break
				}
			}
		}
	}

	// Get Server Adapter URL
	if serverSlotNum != "" {
		serverAdaptersURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/VirtualSCSIServerAdapter", c.hmcIP, viosUUID)
		if verbose {
			hmcLogger.Printf("Fetching VirtualSCSIServerAdapters for VIOS UUID %s, URL: %s", viosUUID, serverAdaptersURL)
		}
		
		serverAdaptersDoc, _ := c.fetchAndParseHMCXML(serverAdaptersURL, verbose)
		if serverAdaptersDoc != nil {
			if verbose {
				docStr, _ := serverAdaptersDoc.WriteToString()
				hmcLogger.Printf("VirtualSCSIServerAdapter XML response body:\n%s", docStr)
			}

			for _, entry := range serverAdaptersDoc.FindElements("//entry") {
				if slot := entry.FindElement(".//VirtualSlotNumber"); slot != nil {
					if verbose {
						hmcLogger.Printf("Found Server Adapter in XML with VirtualSlotNumber: %s", slot.Text())
					}
					if slot.Text() == serverSlotNum {
						for _, link := range entry.FindElements("./link") {
							if link.SelectAttrValue("rel", "") == "SELF" {
								serverAdapterDeleteURL = link.SelectAttrValue("href", "")
								if verbose {
									hmcLogger.Printf("--> Resolved Server Adapter DELETE URL: %s", serverAdapterDeleteURL)
								}
								break
							}
						}
						break
					}
				}
			}
		}
	}

	// =====================================================================
	// STEP 3: EXECUTE STAGE 1 - Delete Client Adapter (Unmaps Disk)
	// =====================================================================
	if clientAdapterDeleteURL != "" {
		if verbose { hmcLogger.Printf("Executing DELETE on LPAR Client Adapter (Slot %s)...", clientSlotNum) }
		reqDelClient, _ := http.NewRequest("DELETE", clientAdapterDeleteURL, nil)
		reqDelClient.Header.Set("X-API-Session", c.session)
		
		respDelClient, err := c.client.Do(reqDelClient)
		if err != nil {
			return fmt.Errorf("HTTP request failed while deleting client adapter: %v", err)
		}
		
		clientBody, _ := io.ReadAll(respDelClient.Body)
		respDelClient.Body.Close()
		
		if verbose {
			hmcLogger.Printf("Client Adapter DELETE Status: %s", respDelClient.Status)
			if len(clientBody) > 0 {
				hmcLogger.Printf("Client Adapter DELETE Response Body:\n%s", string(clientBody))
			}
		}

		if respDelClient.StatusCode >= 400 {
			return fmt.Errorf("failed to delete client adapter. Status: %s, Response: %s", respDelClient.Status, string(clientBody))
		}

		// CRITICAL: We must wait for the VIOS to finish destroying the vtscsi device
		if verbose { hmcLogger.Printf("Waiting 10 seconds for VIOS to release device locks...") }
		time.Sleep(10 * time.Second)
	}

	// =====================================================================
	// STEP 4: EXECUTE STAGE 2 - Delete Server Adapter (Cleans VIOS vhost)
	// =====================================================================
	if serverAdapterDeleteURL != "" {
		if verbose { hmcLogger.Printf("Executing DELETE on VIOS Server Adapter (Slot %s)...", serverSlotNum) }
		reqDelServer, _ := http.NewRequest("DELETE", serverAdapterDeleteURL, nil)
		reqDelServer.Header.Set("X-API-Session", c.session)
		
		respDelServer, err := c.client.Do(reqDelServer)
		if err != nil {
			return fmt.Errorf("HTTP request failed while deleting server adapter: %v", err)
		}
		
		serverBody, _ := io.ReadAll(respDelServer.Body)
		respDelServer.Body.Close()

		if verbose {
			hmcLogger.Printf("Server Adapter DELETE Status: %s", respDelServer.Status)
			if len(serverBody) > 0 {
				hmcLogger.Printf("Server Adapter DELETE Response Body:\n%s", string(serverBody))
			}
		}

		if respDelServer.StatusCode >= 400 {
			return fmt.Errorf("failed to delete server adapter. Status: %s, Response: %s", respDelServer.Status, string(serverBody))
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
// RemoveVIOSDevice executes the RemoveDevice job on the VIOS to delete a physical volume (e.g., hdisk3)
func (c *HmcRestClient) RemoveVIOSDevice(viosUUID, deviceName string, verbose bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/RemoveDevice", c.hmcIP, viosUUID)
	
	operation := map[string]string{
		"OperationName": "RemoveDevice",
		"GroupName":     "VirtualIOServer",
		"ProgressType":  "DISCRETE",
	}

	params := map[string]string{
		"devName": deviceName, 
	}

	payload, err := createJobRequestPayload(operation, params, "V1_1_0", verbose, true)
	if err != nil { return err }

	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil { return err }
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("RemoveDevice failed with status %s: %s", resp.Status, string(body))
	}

	doc, _ := xmlStripNamespace(body)
	if jobIDElem := doc.FindElement("//JobID"); jobIDElem != nil {
		if verbose { hmcLogger.Printf("Removing device %s from VIOS (Job ID: %s)...", deviceName, jobIDElem.Text()) }
		// Wait for the VIOS to finish deleting the disk
		_, err = c.FetchJobStatus(jobIDElem.Text(), false, 5, verbose)
		return err
	}
	
	return fmt.Errorf("JobID not found in RemoveDevice response")
}

// RunVIOSCommand executes an OS-level command by tunneling it through the HMC CLIRunner job via viosvrcmd.
func (c *HmcRestClient) RunVIOSCommand(cmdString string, verbose bool) error {
	// 1. Fetch the Management Console UUID
	mcURL := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole", c.hmcIP)
	
	req, err := http.NewRequest("GET", mcURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")
	req.Header.Set("Accept", "*/*")
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to fetch Management Console: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get Management Console (HTTP %d): %s", resp.StatusCode, string(body))
	}
	
	mcDoc, err := xmlStripNamespace(body)
	if err != nil {
		return fmt.Errorf("failed to parse Management Console XML: %v", err)
	}
	
	// Extract the Management Console UUID from the entry element
	// The feed has its own <id>, but we need the <entry><id> which is the actual MC UUID
	var mcUUID string
	if entryElem := mcDoc.FindElement("//entry/id"); entryElem != nil {
		mcUUID = entryElem.Text()
	} else if uuidElem := mcDoc.FindElement("//ManagementConsole/Metadata/Atom/AtomID"); uuidElem != nil {
		mcUUID = uuidElem.Text()
	} else if uuidElem := mcDoc.FindElement("//UUID"); uuidElem != nil {
		mcUUID = uuidElem.Text()
	}

	if mcUUID == "" {
		if verbose {
			hmcLogger.Printf("Management Console response:\n%s", string(body))
		}
		return fmt.Errorf("could not resolve Management Console UUID from response")
	}
	
	if verbose {
		hmcLogger.Printf("Resolved Management Console UUID: %s", mcUUID)
	}

	// 2. Target the Management Console's CLIRunner Job endpoint
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole/%s/do/CLIRunner", c.hmcIP, mcUUID)

	// 3. Construct the exact viosvrcmd string with proper quoting
	//cmdString := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "rmdev -dev %s "`, sysName, viosName, diskName)

	if verbose {
		hmcLogger.Printf("Executing HMC CLI Command: %s", cmdString)
	}

	// 4. Create the CLIRunner Job Payload with required acknowledgeThisAPIMayGoAwayInTheFuture parameter
	// Using schemaVersion="V1_0" as confirmed working in Postman
	payload := fmt.Sprintf(`<JobRequest:JobRequest xmlns:JobRequest="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
	   <Metadata>
	       <Atom/>
	   </Metadata>
	   <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
	       <Metadata>
	           <Atom/>
	       </Metadata>
	       <OperationName kxe="false" kb="ROR">CLIRunner</OperationName>
	       <GroupName kxe="false" kb="ROR">ManagementConsole</GroupName>
	   </RequestedOperation>
	   <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
	       <Metadata>
	           <Atom/>
	       </Metadata>
	       <JobParameter schemaVersion="V1_0">
	           <Metadata>
	               <Atom/>
	           </Metadata>
	           <ParameterName kxe="false" kb="ROR">cmd</ParameterName>
	           <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
	       </JobParameter>
	       <JobParameter schemaVersion="V1_0">
	           <Metadata>
	               <Atom/>
	           </Metadata>
	           <ParameterName kxe="false" kb="ROR">acknowledgeThisAPIMayGoAwayInTheFuture</ParameterName>
	           <ParameterValue kxe="false" kb="CUR">true</ParameterValue>
	       </JobParameter>
	   </JobParameters>
</JobRequest:JobRequest>`, cmdString)

	if verbose {
		hmcLogger.Printf("CLIRunner payload:\n%s", payload)
		hmcLogger.Printf("CLIRunner URL: %s", url)
	}

	// 5. Submit the JobRequest via PUT (Jobs in HMC REST API use PUT)
	req2, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create CLIRunner request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req2.Header.Set("X-API-Session", c.session)
	req2.Header.Set("Accept", "application/atom+xml")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()

	resp2, err := c.client.Do(req2.WithContext(ctx2))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return fmt.Errorf("failed to read CLIRunner response: %v", err)
	}

	// Look for success status codes
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusAccepted && resp2.StatusCode != http.StatusCreated {
		return fmt.Errorf("CLIRunner failed with status %s: %s", resp2.Status, string(body2))
	}

	if verbose {
		hmcLogger.Printf("CLIRunner response (HTTP %d):\n%s", resp2.StatusCode, string(body2))
	}

	// Wait for the job to complete
	doc2, err := xmlStripNamespace(body2)
	if err != nil {
		return fmt.Errorf("failed to parse CLIRunner response XML: %v", err)
	}

	jobIDElem := doc2.FindElement("//JobID")
	if jobIDElem == nil {
		// Try alternative path
		jobIDElem = doc2.FindElement("//JobResponse/JobID")
	}
	
	if jobIDElem != nil {
		jobID := jobIDElem.Text()
		if verbose {
			hmcLogger.Printf("CLIRunner Job submitted (Job ID: %s), waiting for completion...", jobID)
		}
		_, err = c.FetchJobStatus(jobID, false, 10, verbose)
		if err != nil {
			return fmt.Errorf("CLIRunner job failed: %v", err)
		}
		if verbose {
			hmcLogger.Printf("✅ CLIRunner job completed successfully")
		}
	} else {
		return fmt.Errorf("JobID not found in CLIRunner response: %s", string(body2))
	}

	return nil
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

/// GetVirtualIOServers retrieves detailed, comprehensive information for all Virtual I/O Servers of a managed system.
func (c *HmcRestClient) GetVirtualIOServers(systemUUID string, verbose bool) ([]VirtualIOServerDetails, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching comprehensive VIOS details for system UUID %s, URL: %s", systemUUID, url)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetVirtualIOServers HTTP response status: %s", resp.Status)
	}
	if verbose {
		hmcLogger.Printf("GetVirtualIOServers HTTP response status: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Strip XML namespaces to make path searching easier with etree
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var viosList []VirtualIOServerDetails

	// Find all VirtualIOServer elements in the stripped XML
	entries := doc.FindElements("//VirtualIOServer")
	if verbose {
		hmcLogger.Printf("Found %d potential VirtualIOServer XML elements", len(entries))
	}

	for _, entry := range entries {
		// Ensure it is a primary VIOS element by checking for PartitionID
		partitionID := getElementText(entry, "PartitionID")
		if partitionID == "" {
			continue
		}

		partitionName := getElementText(entry, "PartitionName")
		if verbose {
			hmcLogger.Printf("\n--- Processing VIOS: %s (Partition ID: %s) ---", partitionName, partitionID)
		}

		vios := VirtualIOServerDetails{
			UUID:                        getElementText(entry, "PartitionUUID"),
			PartitionID:                 partitionID,
			PartitionName:               partitionName,
			PartitionState:              getElementText(entry, "PartitionState"),
			PartitionType:               getElementText(entry, "PartitionType"),
			SystemName:                  getElementText(entry, "SystemName"),
			OperatingSystemVersion:      getElementText(entry, "OperatingSystemVersion"),
			ResourceMonitoringIPAddress: getElementText(entry, "ResourceMonitoringIPAddress"),
			LogicalSerialNumber:         getElementText(entry, "LogicalSerialNumber"),
			IsBootable:                  getElementText(entry, "IsBootable"),
			Uptime:                      getElementText(entry, "Uptime"),
		}

		// 1. Parse Memory Configuration
		memConfig := entry.FindElement("PartitionMemoryConfiguration")
		if memConfig != nil {
			if verbose {
				hmcLogger.Printf("  [%s] Parsing Memory Configuration", partitionName)
			}
			vios.Memory = VIOSMemoryConfig{
				DesiredMemory: getElementText(memConfig, "DesiredMemory"),
				MaximumMemory: getElementText(memConfig, "MaximumMemory"),
				MinimumMemory: getElementText(memConfig, "MinimumMemory"),
			}
		}

		// 2. Parse Processor Configuration
		procConfig := entry.FindElement("PartitionProcessorConfiguration")
		if procConfig != nil {
			if verbose {
				hmcLogger.Printf("  [%s] Parsing Processor Configuration", partitionName)
			}
			vios.Processor = VIOSProcessorConfig{
				HasDedicatedProcessors: getElementText(procConfig, "HasDedicatedProcessors"),
				SharingMode:            getElementText(procConfig, "SharingMode"),
			}

			if dedProc := procConfig.FindElement("DedicatedProcessorConfiguration"); dedProc != nil {
				vios.Processor.DesiredProcessors = getElementText(dedProc, "DesiredProcessors")
				vios.Processor.MaximumProcessors = getElementText(dedProc, "MaximumProcessors")
				vios.Processor.MinimumProcessors = getElementText(dedProc, "MinimumProcessors")
			}
		}

		// 3. Parse Storage Configuration (PVs, VFC Mappings, and Physical FC Ports)
		pvs := entry.FindElements("PhysicalVolumes/PhysicalVolume")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Physical Volumes", partitionName, len(pvs))
		}
		for _, pv := range pvs {
			vios.Storage.PhysicalVolumes = append(vios.Storage.PhysicalVolumes, VIOSPhysicalVolume{
				VolumeName:     getElementText(pv, "VolumeName"),
				VolumeCapacity: getElementText(pv, "VolumeCapacity"),
				VolumeState:    getElementText(pv, "VolumeState"),
				UniqueDeviceID: getElementText(pv, "UniqueDeviceID"),
				LocationCode:   getElementText(pv, "LocationCode"),
			})
		}
		
		vfcs := entry.FindElements("VirtualFibreChannelMappings/VirtualFibreChannelMapping")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Virtual Fibre Channel Mappings", partitionName, len(vfcs))
		}
		for _, vfc := range vfcs {
			vios.Storage.VFCMappings = append(vios.Storage.VFCMappings, VIOSVFCMapping{
				ServerAdapterSlot: getElementText(vfc, "ServerAdapter/VirtualSlotNumber"),
				ClientPartitionID: getElementText(vfc, "ServerAdapter/ConnectingPartitionID"),
				ClientAdapterSlot: getElementText(vfc, "ServerAdapter/ConnectingVirtualSlotNumber"),
				MapPort:           getElementText(vfc, "ServerAdapter/MapPort"),
				PortWWPN:          getElementText(vfc, "Port/WWPN"),
				PortWWNN:          getElementText(vfc, "Port/WWNN"),
			})
		}

		// Use a deep search to find all PhysicalFibreChannelPort nodes regardless of exact PCI slot nesting
		fcPorts := entry.FindElements(".//PhysicalFibreChannelPort")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Physical Fibre Channel Ports", partitionName, len(fcPorts))
		}
		for _, fcPort := range fcPorts {
			portName := getElementText(fcPort, "PortName")
			wwpn := getElementText(fcPort, "WWPN")
			
			if verbose {
				hmcLogger.Printf("    -> Extracted Port: %-5s | WWPN: %s", portName, wwpn)
			}
			
			vios.Storage.FibreChannelPorts = append(vios.Storage.FibreChannelPorts, VIOSFibreChannelPort{
				PortName:     portName,
				LocationCode: getElementText(fcPort, "LocationCode"),
				WWPN:         wwpn,
				WWNN:         getElementText(fcPort, "WWNN"),
			})
		}

		// 4. Parse Network Configuration (Shared Ethernet Adapters and Trunks)
		seas := entry.FindElements("SharedEthernetAdapters/SharedEthernetAdapter")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Shared Ethernet Adapters", partitionName, len(seas))
		}
		for _, sea := range seas {
			vios.Network.SharedEthernetAdapters = append(vios.Network.SharedEthernetAdapters, VIOSSharedEthernetAdapter{
				DeviceName:         getElementText(sea, "DeviceName"),
				HighAvailability:   getElementText(sea, "HighAvailabilityMode"),
				PortVLANID:         getElementText(sea, "PortVLANID"),
				BackingDevice:      getElementText(sea, "BackingDeviceChoice/EthernetBackingDevice/DeviceName"),
				ConfigurationState: getElementText(sea, "ConfigurationState"),
			})
		}
		
		trunks := entry.FindElements("TrunkAdapters/TrunkAdapter")
		if verbose {
			hmcLogger.Printf("  [%s] Found %d Trunk Adapters", partitionName, len(trunks))
		}
		for _, trunk := range trunks {
			vios.Network.TrunkAdapters = append(vios.Network.TrunkAdapters, VIOSTrunkAdapter{
				DeviceName:        getElementText(trunk, "DeviceName"),
				MACAddress:        getElementText(trunk, "MACAddress"),
				PortVLANID:        getElementText(trunk, "PortVLANID"),
				VirtualSlotNumber: getElementText(trunk, "VirtualSlotNumber"),
			})
		}

		viosList = append(viosList, vios)
	}

	if verbose {
		hmcLogger.Printf("Successfully parsed %d valid Virtual I/O Servers with complete details", len(viosList))
	}

	return viosList, nil
}
// GetVirtualIOServer retrieves detailed information for a specific Virtual I/O Server using its UUID.
func (c *HmcRestClient) GetVirtualIOServer(viosUUID string, verbose bool) (*VirtualIOServerDetails, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s", c.hmcIP, viosUUID)
	if verbose {
		hmcLogger.Printf("Fetching VIOS details for UUID %s, URL: %s", viosUUID, url)
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

	// Strip XML namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// When querying a single item, the root or an entry contains the VirtualIOServer node
	entry := doc.FindElement("//VirtualIOServer")
	if entry == nil {
		return nil, fmt.Errorf("could not find VirtualIOServer in response")
	}

	vios := &VirtualIOServerDetails{
		UUID:                        getElementText(entry, "PartitionUUID"),
		PartitionID:                 getElementText(entry, "PartitionID"),
		PartitionName:               getElementText(entry, "PartitionName"),
		PartitionState:              getElementText(entry, "PartitionState"),
		PartitionType:               getElementText(entry, "PartitionType"),
		SystemName:                  getElementText(entry, "SystemName"),
		OperatingSystemVersion:      getElementText(entry, "OperatingSystemVersion"),
		ResourceMonitoringIPAddress: getElementText(entry, "ResourceMonitoringIPAddress"),
		LogicalSerialNumber:         getElementText(entry, "LogicalSerialNumber"),
		IsBootable:                  getElementText(entry, "IsBootable"),
		Uptime:                      getElementText(entry, "Uptime"),
	}

	// 1. Parse Memory Configuration
	if memConfig := entry.FindElement("PartitionMemoryConfiguration"); memConfig != nil {
		vios.Memory = VIOSMemoryConfig{
			DesiredMemory: getElementText(memConfig, "DesiredMemory"),
			MaximumMemory: getElementText(memConfig, "MaximumMemory"),
			MinimumMemory: getElementText(memConfig, "MinimumMemory"),
		}
	}

	// 2. Parse Processor Configuration
	if procConfig := entry.FindElement("PartitionProcessorConfiguration"); procConfig != nil {
		vios.Processor = VIOSProcessorConfig{
			HasDedicatedProcessors: getElementText(procConfig, "HasDedicatedProcessors"),
			SharingMode:            getElementText(procConfig, "SharingMode"),
		}
		if dedProc := procConfig.FindElement("DedicatedProcessorConfiguration"); dedProc != nil {
			vios.Processor.DesiredProcessors = getElementText(dedProc, "DesiredProcessors")
			vios.Processor.MaximumProcessors = getElementText(dedProc, "MaximumProcessors")
			vios.Processor.MinimumProcessors = getElementText(dedProc, "MinimumProcessors")
		}
	}

	// 3. Parse Storage Configuration
	for _, pv := range entry.FindElements("PhysicalVolumes/PhysicalVolume") {
		vios.Storage.PhysicalVolumes = append(vios.Storage.PhysicalVolumes, VIOSPhysicalVolume{
			VolumeName:     getElementText(pv, "VolumeName"),
			VolumeCapacity: getElementText(pv, "VolumeCapacity"),
			VolumeState:    getElementText(pv, "VolumeState"),
			UniqueDeviceID: getElementText(pv, "UniqueDeviceID"),
			LocationCode:   getElementText(pv, "LocationCode"),
		})
	}
	for _, vfc := range entry.FindElements("VirtualFibreChannelMappings/VirtualFibreChannelMapping") {
		vios.Storage.VFCMappings = append(vios.Storage.VFCMappings, VIOSVFCMapping{
			ServerAdapterSlot: getElementText(vfc, "ServerAdapter/VirtualSlotNumber"),
			ClientPartitionID: getElementText(vfc, "ServerAdapter/ConnectingPartitionID"),
			ClientAdapterSlot: getElementText(vfc, "ServerAdapter/ConnectingVirtualSlotNumber"),
			MapPort:           getElementText(vfc, "ServerAdapter/MapPort"),
			PortWWPN:          getElementText(vfc, "Port/WWPN"),
			PortWWNN:          getElementText(vfc, "Port/WWNN"),
		})
	}
	for _, fcPort := range entry.FindElements(".//PhysicalFibreChannelPort") {
		vios.Storage.FibreChannelPorts = append(vios.Storage.FibreChannelPorts, VIOSFibreChannelPort{
			PortName:     getElementText(fcPort, "PortName"),
			LocationCode: getElementText(fcPort, "LocationCode"),
			WWPN:         getElementText(fcPort, "WWPN"),
			WWNN:         getElementText(fcPort, "WWNN"),
		})
	}

	// 4. Parse Network Configuration
	for _, sea := range entry.FindElements("SharedEthernetAdapters/SharedEthernetAdapter") {
		vios.Network.SharedEthernetAdapters = append(vios.Network.SharedEthernetAdapters, VIOSSharedEthernetAdapter{
			DeviceName:         getElementText(sea, "DeviceName"),
			HighAvailability:   getElementText(sea, "HighAvailabilityMode"),
			PortVLANID:         getElementText(sea, "PortVLANID"),
			BackingDevice:      getElementText(sea, "BackingDeviceChoice/EthernetBackingDevice/DeviceName"),
			ConfigurationState: getElementText(sea, "ConfigurationState"),
		})
	}
	for _, trunk := range entry.FindElements("TrunkAdapters/TrunkAdapter") {
		vios.Network.TrunkAdapters = append(vios.Network.TrunkAdapters, VIOSTrunkAdapter{
			DeviceName:        getElementText(trunk, "DeviceName"),
			MACAddress:        getElementText(trunk, "MACAddress"),
			PortVLANID:        getElementText(trunk, "PortVLANID"),
			VirtualSlotNumber: getElementText(trunk, "VirtualSlotNumber"),
		})
	}

	return vios, nil
}