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
func (c *HmcRestClient) GetVirtualSwitchQuickAll(sysUUID string, verbose bool) ([]VirtualSwitchQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/quick/All", c.hmcIP, sysUUID)

	if verbose {
		hmcLogger.Printf("Fetching all Virtual Switches (Quick) for system %s...", sysUUID)
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
		return nil, fmt.Errorf("failed to fetch Virtual Switches Quick/All: %s - %s", resp.Status, string(body))
	}

	var switches []VirtualSwitchQuick
	if err := json.Unmarshal(body, &switches); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return switches, nil
}

// GetVirtualSwitchQuick retrieves JSON details for a specific Virtual Switch.
func (c *HmcRestClient) GetVirtualSwitchQuick(sysUUID, switchUUID string, verbose bool) (*VirtualSwitchQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s/quick", c.hmcIP, sysUUID, switchUUID)

	if verbose {
		hmcLogger.Printf("Fetching Virtual Switch (Quick) for UUID %s...", switchUUID)
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
		return nil, fmt.Errorf("failed to fetch Virtual Switch Quick: %s - %s", resp.Status, string(body))
	}

	var vSwitch VirtualSwitchQuick
	if err := json.Unmarshal(body, &vSwitch); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// The singular /quick endpoint omits the UUID, so we inject it manually
	vSwitch.UUID = switchUUID

	return &vSwitch, nil
}

// GetVirtualSwitches retrieves the comprehensive XML feed of Virtual Switches on a Managed System.
func (c *HmcRestClient) GetVirtualSwitches(sysUUID string, verbose bool) ([]VirtualSwitch, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch", c.hmcIP, sysUUID)

	if verbose {
		hmcLogger.Printf("Fetching Virtual Switches (XML) for system %s...", sysUUID)
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

	return switches, nil
}

// GetClientNetworkAdapters retrieves all ClientNetworkAdapter details for a partition as parsed Go structs.
func (c *HmcRestClient) GetClientNetworkAdapters(systemUUID, lparUUID string, verbose bool) ([]ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, systemUUID, lparUUID)
	
	if verbose {
		hmcLogger.Printf("Fetching ClientNetworkAdapters for system UUID %s, partition UUID %s, URL: %s", systemUUID, lparUUID, url)
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

	if resp.StatusCode != http.StatusOK {
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
			return nil, fmt.Errorf("failed to serialize adapter element: %v", err)
		}

		var adapter ClientNetworkAdapter
		if err := xml.Unmarshal(adapterBytes, &adapter); err != nil {
			return nil, fmt.Errorf("failed to unmarshal adapter XML: %v", err)
		}
		
		adapters = append(adapters, adapter)
	}

	if verbose {
		hmcLogger.Printf("Successfully parsed %d ClientNetworkAdapter(s)", len(adapters))
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
//   - verbose: Enable detailed logging
//
// Returns:
//   - []NetworkBootDevice: List of network boot devices configured in the profile
//   - error: Error if the operation fails
//
// Reference: HMC REST API - GetNetworkBootDevices job operation
func (c *HmcRestClient) GetNetworkBootDevices(lparUUID, profileUUID string, verbose bool) ([]NetworkBootDevice, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/GetNetworkBootDevices", c.hmcIP, lparUUID)

	if verbose {
		hmcLogger.Printf("Getting network boot devices for LPAR %s, profile %s", lparUUID, profileUUID)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_1_0", verbose, true)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
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

	if verbose {
		hmcLogger.Printf("GetNetworkBootDevices job submitted, JobID: %s", jobID)
	}

	// Wait for job completion
	jobResp, err := c.FetchJobStatus(jobID, false, 5, verbose)
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

			if verbose {
				hmcLogger.Printf("Parsing cleaned XML content (%d bytes)", len(xmlContent))
			}

			// 2. Strip Namespaces to make Unmarshaling easy
			cleanDoc, err := xmlStripNamespace([]byte(xmlContent))
			if err != nil {
				if verbose {
					hmcLogger.Printf("Failed to strip namespaces from result XML: %v", err)
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
				if verbose {
					hmcLogger.Printf("Failed to unmarshal NetworkBootDevices XML: %v", err)
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

	if verbose {
		hmcLogger.Printf("Retrieved %d network boot device(s)", len(bootDevices))
	}

	return bootDevices, nil
}

// CreateClientNetworkAdapter adds a new Virtual Ethernet Adapter to an LPAR and connects it to a Virtual Switch.
// Returns the complete ClientNetworkAdapter structure with all details.
func (c *HmcRestClient) CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID string, vlanID int, verbose bool) (*ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, lparUUID)

	if verbose {
		hmcLogger.Printf("Adding Client Network Adapter (VLAN: %d) to LPAR %s...", vlanID, lparUUID)
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

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if verbose {
		hmcLogger.Printf("CreateClientNetworkAdapter response status: %s", resp.Status)
		hmcLogger.Printf("CreateClientNetworkAdapter response body:\n%s", string(body))
	}
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
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

	if verbose {
		hmcLogger.Printf("✅ Client Network Adapter created successfully!")
		hmcLogger.Printf("   UUID: %s", adapter.UUID)
		hmcLogger.Printf("   MAC Address: %s", adapter.MACAddress)
		hmcLogger.Printf("   VLAN ID: %s", adapter.PortVLANID)
		hmcLogger.Printf("   Virtual Slot: %s", adapter.VirtualSlotNumber)
		hmcLogger.Printf("   Location Code: %s", adapter.LocationCode)
	}

	return &adapter, nil
}
// DeleteClientNetworkAdapter removes a specific Virtual Ethernet Adapter from an LPAR.
func (c *HmcRestClient) DeleteClientNetworkAdapter(lparUUID, adapterUUID string, verbose bool) error {
	// The specific endpoint for the adapter we want to delete
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter/%s", c.hmcIP, lparUUID, adapterUUID)

	if verbose {
		hmcLogger.Printf("Deleting Client Network Adapter %s from LPAR %s...", adapterUUID, lparUUID)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	// A successful DELETE operation typically returns 200 OK or 204 No Content
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete adapter (%s): %s", resp.Status, string(body))
	}

	if verbose {
		hmcLogger.Printf("✅ Client Network Adapter %s successfully deleted.", adapterUUID)
	}

	return nil
}