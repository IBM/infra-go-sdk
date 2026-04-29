package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// GetVirtualSwitchQuickAll retrieves a JSON array of all Virtual Switches on a Managed System.
func (c *HmcRestClient) GetVirtualSwitchQuickAll(sysUUID string, debug bool) ([]VirtualSwitchQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/quick/All", c.hmcIP, sysUUID)

	if debug {
		c.Logger.Debug("Fetching all Virtual Switches (Quick)", "systemUUID", sysUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Failed to fetch Virtual Switches Quick/All", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("failed to fetch Virtual Switches Quick/All: %s - %s", resp.Status, string(body))
	}

	var switches []VirtualSwitchQuick
	if err := json.Unmarshal(body, &switches); err != nil {
		c.Logger.Error("Failed to parse JSON response", "error", err)
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual Switches Quick/All", "count", len(switches))
	}

	return switches, nil
}

// GetVirtualSwitchQuick retrieves JSON details for a specific Virtual Switch.
func (c *HmcRestClient) GetVirtualSwitchQuick(sysUUID, switchUUID string, debug bool) (*VirtualSwitchQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s/quick", c.hmcIP, sysUUID, switchUUID)

	if debug {
		c.Logger.Debug("Fetching Virtual Switch (Quick)", "switchUUID", switchUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Failed to fetch Virtual Switch Quick", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("failed to fetch Virtual Switch Quick: %s - %s", resp.Status, string(body))
	}

	var vSwitch VirtualSwitchQuick
	if err := json.Unmarshal(body, &vSwitch); err != nil {
		c.Logger.Error("Failed to parse JSON response", "error", err)
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// The singular /quick endpoint omits the UUID, so we inject it manually
	vSwitch.UUID = switchUUID

	if debug {
		c.Logger.Info("Successfully retrieved Virtual Switch Quick", "switchName", vSwitch.SwitchName)
	}

	return &vSwitch, nil
}

// GetVirtualSwitches retrieves the comprehensive XML feed of Virtual Switches on a Managed System.
func (c *HmcRestClient) GetVirtualSwitches(sysUUID string, debug bool) ([]VirtualSwitch, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch", c.hmcIP, sysUUID)

	if debug {
		c.Logger.Debug("Fetching Virtual Switches (XML)", "systemUUID", sysUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Failed to fetch Virtual Switches XML", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("failed to fetch Virtual Switches XML: %s - %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	var switches []VirtualSwitch

	// Loop through every <entry> in the Atom feed
	for _, entry := range doc.FindElements("//entry") {
		vSwitch := VirtualSwitch{}

		// Extract Core properties
		if atomID := entry.FindElement(".//AtomID"); atomID != nil {
			vSwitch.UUID = atomID.Text()
		}
		if switchID := entry.FindElement(".//SwitchID"); switchID != nil {
			vSwitch.SwitchID = switchID.Text()
		}
		if switchName := entry.FindElement(".//SwitchName"); switchName != nil {
			vSwitch.SwitchName = switchName.Text()
		}
		if switchMode := entry.FindElement(".//SwitchMode"); switchMode != nil {
			vSwitch.SwitchMode = switchMode.Text()
		}

		// Extract VirtualNetwork Links
		for _, link := range entry.FindElements(".//VirtualNetworks/link") {
			if href := link.SelectAttrValue("href", ""); href != "" {
				vSwitch.VirtualNetworks = append(vSwitch.VirtualNetworks, href)
			}
		}

		switches = append(switches, vSwitch)
	}

	if debug {
		c.Logger.Info("Successfully parsed Virtual Switches XML", "count", len(switches))
	}

	return switches, nil
}

// GetClientNetworkAdapters retrieves all ClientNetworkAdapter details for a partition as parsed Go structs.
func (c *HmcRestClient) GetClientNetworkAdapters(systemUUID, lparUUID string, debug bool) ([]ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, systemUUID, lparUUID)
	
	if debug {
		c.Logger.Debug("Fetching ClientNetworkAdapters", "systemUUID", systemUUID, "lparUUID", lparUUID, "url", url)
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
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	// ✨ THE FIX: Handle the 204 No Content status cleanly
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			if debug {
				c.Logger.Debug("No Client Network Adapters found on LPAR (204 No Content)")
			}
			return []ClientNetworkAdapter{}, nil
		}
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
	}

	// Parse XML response and strip namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var adapters []ClientNetworkAdapter

	// Extract the core elements and natively unmarshal them into structs
	adapterElements := doc.FindElements("//ClientNetworkAdapter")
	
	for _, elem := range adapterElements {
		adapterDoc := etree.NewDocument()
		adapterDoc.SetRoot(elem.Copy())
		adapterBytes, err := adapterDoc.WriteToBytes()
		if err != nil {
			c.Logger.Warn("Failed to serialize adapter element", "error", err)
			return nil, fmt.Errorf("failed to serialize adapter element: %v", err)
		}

		var adapter ClientNetworkAdapter
		if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
			c.Logger.Warn("Failed to unmarshal adapter XML", "error", err)
			return nil, fmt.Errorf("failed to unmarshal adapter XML: %v", err)
		}
		
		adapters = append(adapters, adapter)
	}

	if debug {
		c.Logger.Info("Successfully parsed ClientNetworkAdapter(s)", "count", len(adapters))
	}

	return adapters, nil
}
// GetNetworkBootDevices retrieves network boot devices from an LPAR's profile using the HMC REST API job.
//
// WARNING: This operation will power off the LPAR if it is currently running.
// The HMC GetNetworkBootDevices job requires the LPAR to be in a powered-off state
// to retrieve accurate network boot device information from the profile.
//
// Parameters:
//   - lparUUID: UUID of the logical partition
//   - profileUUID: UUID of the partition profile to query
//   - debug: Enable detailed logging
//
// Returns:
//   - []NetworkBootDevice: List of network boot devices configured in the profile
//   - error: Error if the operation fails
//
// Reference: HMC REST API - GetNetworkBootDevices job operation
func (c *HmcRestClient) GetNetworkBootDevices(lparUUID, profileUUID string, debug bool) ([]NetworkBootDevice, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/GetNetworkBootDevices", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Getting network boot devices", "lparUUID", lparUUID, "profileUUID", profileUUID)
	}

	// Define operation details for the JobRequest
	reqdOperation := map[string]string{
		"OperationName": "GetNetworkBootDevices",
		"GroupName":     "LogicalPartition",
		"ProgressType":  "DISCRETE",
	}

	// Build job parameters
	jobParams := map[string]string{
		"LogicalPartitionProfileUUID": profileUUID,
	}

	// Create XML payload using the existing job request helper
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_1_0", debug, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create job request payload: %v", err)
	}

	// Create and execute HTTP request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml;type=JobRequest")
	req.Header.Set("Accept", "*/*")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(respBody))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		c.Logger.Error("Request failed", "status", resp.Status, "body", string(respBody))
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(respBody))
	}

	// Parse response to extract JobID
	doc, err := xmlStripNamespace(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	jobIDElem := doc.FindElement("//JobID")
	if jobIDElem == nil {
		return nil, fmt.Errorf("JobID not found in response")
	}
	jobID := jobIDElem.Text()

	if debug {
		c.Logger.Debug("GetNetworkBootDevices job submitted", "jobID", jobID)
	}

	// Wait for job completion
	jobResp, err := c.FetchJobStatus(context.Background(), jobID, false, 5, debug)
	if err != nil {
		return nil, fmt.Errorf("GetNetworkBootDevices job failed: %v", err)
	}

	// Parse job results to extract network boot devices
	var bootDevices []NetworkBootDevice

	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "result" {
			xmlContent := param.ParameterValue

			// 1. Clean the CDATA wrapper and unescape any HTML entities
			xmlContent = strings.TrimSpace(xmlContent)
			xmlContent = strings.TrimPrefix(xmlContent, "<![CDATA[")
			xmlContent = strings.TrimSuffix(xmlContent, "]]>")
			xmlContent = html.UnescapeString(strings.TrimSpace(xmlContent))

			if debug {
				c.Logger.Debug("Parsing cleaned XML content", "bytes", len(xmlContent))
			}

			// 2. Strip Namespaces to make Unmarshaling easy
			cleanDoc, err := xmlStripNamespace([]byte(xmlContent))
			if err != nil {
				if debug {
					c.Logger.Warn("Failed to strip namespaces from result XML", "error", err)
				}
				continue
			}
			strippedXML, _ := cleanDoc.WriteToBytes()

			// 3. Use an inline struct to map IBM's XML directly using standard encoding/xml
			var collection struct {
				Devices []struct {
					BootDevice       string `xml:"BootDevice"`
					IsPhysicalDevice bool   `xml:"IsPhysicalDevice"`
					LocationCode     string `xml:"LocationCode"`
					MACAddressValue  string `xml:"MACAddressValue"`
				} `xml:"NetworkBootDevice"`
			}

			if err := xml.Unmarshal(strippedXML, &collection); err != nil {
				if debug {
					c.Logger.Warn("Failed to unmarshal NetworkBootDevices XML", "error", err)
				}
				continue
			}

			// 4. Map the unmarshaled data to your SDK's return struct
			for _, d := range collection.Devices {
				deviceType := "virtual"
				if d.IsPhysicalDevice {
					deviceType = "physical"
				}

				bootDevices = append(bootDevices, NetworkBootDevice{
					DeviceName:   d.BootDevice,
					DeviceType:   deviceType,
					LocationCode: d.LocationCode,
					MACAddress:   d.MACAddressValue,
				})
			}
		}
	}

	if debug {
		c.Logger.Info("Retrieved network boot device(s)", "count", len(bootDevices))
	}

	return bootDevices, nil
}

// CreateClientNetworkAdapter adds a new Virtual Ethernet Adapter to an LPAR and connects it to a Virtual Switch.
// Returns the complete ClientNetworkAdapter structure with all details.
func (c *HmcRestClient) CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID string, vlanID int, debug bool) (*ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Adding Client Network Adapter", "vlanID", vlanID, "lparUUID", lparUUID)
	}

	// Fix: Changed <VirtualSwitch> to <AssociatedVirtualSwitch> with a <link> child.
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<ClientNetworkAdapter:ClientNetworkAdapter xmlns:ClientNetworkAdapter="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                                           xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" 
                                           schemaVersion="V1_0">
    <Metadata><Atom/></Metadata>
    <PortVLANID>%d</PortVLANID>
    <AssociatedVirtualSwitch>
        <link href="https://%s:443/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s" rel="related"/>
    </AssociatedVirtualSwitch>
</ClientNetworkAdapter:ClientNetworkAdapter>`, 
		vlanID, c.hmcIP, sysUUID, vswitchUUID)

	httpReq, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ClientNetworkAdapter")
	httpReq.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(body))
	
	if debug {
		c.Logger.Debug("CreateClientNetworkAdapter response", "status", resp.Status, "body", string(body))
	}
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.Logger.Error("Adapter creation failed", "status", resp.Status)
		return nil, fmt.Errorf("Adapter creation failed (%s): %s", resp.Status, string(body))
	}

	// Parse the XML response and strip namespaces
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Extract the ClientNetworkAdapter element
	adapterElement := doc.FindElement("//ClientNetworkAdapter")
	if adapterElement == nil {
		return nil, fmt.Errorf("ClientNetworkAdapter element not found in response")
	}

	// Convert the element to a standalone document for unmarshaling
	adapterDoc := etree.NewDocument()
	adapterDoc.SetRoot(adapterElement.Copy())
	adapterBytes, err := adapterDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize adapter element: %v", err)
	}

	// Unmarshal into the ClientNetworkAdapter struct
	var adapter ClientNetworkAdapter
	if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
		return nil, fmt.Errorf("failed to unmarshal adapter XML: %v", err)
	}

	if debug {
		c.Logger.Info("Client Network Adapter created successfully",
			"uuid", adapter.UUID,
			"macAddress", adapter.MACAddress,
			"vlanID", adapter.PortVLANID,
			"virtualSlot", adapter.VirtualSlotNumber,
			"locationCode", adapter.LocationCode)
	}

	return &adapter, nil
}

// DeleteClientNetworkAdapter removes a specific Virtual Ethernet Adapter from an LPAR.
func (c *HmcRestClient) DeleteClientNetworkAdapter(lparUUID, adapterUUID string, debug bool) error {
	// The specific endpoint for the adapter we want to delete
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter/%s", c.hmcIP, lparUUID, adapterUUID)

	if debug {
		c.Logger.Debug("Deleting Client Network Adapter", "adapterUUID", adapterUUID, "lparUUID", lparUUID)
	}

	// Create the DELETE request (No XML body required!)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (DELETE)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(body))
	
	// A successful DELETE operation typically returns 200 OK or 204 No Content
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.Logger.Error("Failed to delete adapter", "status", resp.Status, "body", string(body))
		return fmt.Errorf("failed to delete adapter (%s): %s", resp.Status, string(body))
	}

	if debug {
		c.Logger.Info("Client Network Adapter successfully deleted", "adapterUUID", adapterUUID)
	}

	return nil
}