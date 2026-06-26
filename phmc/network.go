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
func (c *RestClient) GetVirtualSwitchQuickAll(ctx context.Context, sysUUID string, debug bool) ([]VirtualSwitchQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/quick/All", c.hmcIP, sysUUID)

	if debug {
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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
		if debug {
			return nil, fmt.Errorf("failed to fetch Virtual Switches Quick/All: %s - %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("failed to fetch Virtual Switches Quick/All: %s. Enable debug mode to see full response", resp.Status)
	}

	var switches []VirtualSwitchQuick
	if err := json.Unmarshal(body, &switches); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	if debug {
	}

	return switches, nil
}

// GetVirtualSwitchQuick retrieves JSON details for a specific Virtual Switch.
func (c *RestClient) GetVirtualSwitchQuick(ctx context.Context, sysUUID, switchUUID string, debug bool) (*VirtualSwitchQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s/quick", c.hmcIP, sysUUID, switchUUID)

	if debug {
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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
		if debug {
			return nil, fmt.Errorf("failed to fetch Virtual Switch Quick: %s - %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("failed to fetch Virtual Switch Quick: %s. Enable debug mode to see full response", resp.Status)
	}

	var vSwitch VirtualSwitchQuick
	if err := json.Unmarshal(body, &vSwitch); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// The singular /quick endpoint omits the UUID, so we inject it manually
	vSwitch.UUID = switchUUID

	if debug {
	}

	return &vSwitch, nil
}

// GetVirtualSwitches retrieves the comprehensive XML feed of Virtual Switches on a Managed System.
func (c *RestClient) GetVirtualSwitches(ctx context.Context, sysUUID string, debug bool) ([]VirtualSwitch, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch", c.hmcIP, sysUUID)

	if debug {
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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
		if debug {
			return nil, fmt.Errorf("failed to fetch Virtual Switches XML: %s - %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("failed to fetch Virtual Switches XML: %s. Enable debug mode to see full response", resp.Status)
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
	}

	return switches, nil
}

// GetClientNetworkAdapters retrieves all ClientNetworkAdapter details for a partition as parsed Go structs.
func (c *RestClient) GetClientNetworkAdapters(ctx context.Context, systemUUID, lparUUID string, debug bool) ([]ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, systemUUID, lparUUID)

	if debug {
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}


	// ✨ THE FIX: Handle the 204 No Content status cleanly
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			if debug {
			}
			return []ClientNetworkAdapter{}, nil
		}
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
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

	if debug {
	}

	return adapters, nil
}

// GetNetworkBootDevicesForLpar retrieves network boot devices from an LPAR's profile using the HMC REST API job.
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
func (c *RestClient) GetNetworkBootDevicesForLpar(ctx context.Context, lparUUID, profileUUID string, debug bool) ([]NetworkBootDevice, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/do/GetNetworkBootDevices", c.hmcIP, lparUUID)

	if debug {
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

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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

	if debug {
	}

	// Wait for job completion
	jobResp, err := c.FetchJobStatus(ctx, jobID, false, 5, debug)
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
			}

			// 2. Strip Namespaces to make Unmarshaling easy
			cleanDoc, err := xmlStripNamespace([]byte(xmlContent))
			if err != nil {
				if debug {
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
	}

	return bootDevices, nil
}

// GetNetworkBootDevicesForVios retrieves network boot devices from a VIOS profile
// using the HMC REST API job.
//
// WARNING: This operation will power off the VIOS if it is currently running.
// The HMC GetNetworkBootDevices job requires the partition to be in a powered-off state.
//
// Parameters:
//   - viosUUID: UUID of the Virtual I/O Server
//   - profileUUID: UUID of the partition profile to query
//   - debug: Enable detailed logging
//
// Returns:
//   - []NetworkBootDevice: List of network boot devices configured in the profile
//   - error: Error if the operation fails
func (c *RestClient) GetNetworkBootDevicesForVios(ctx context.Context, viosUUID, profileUUID string, debug bool) ([]NetworkBootDevice, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/GetNetworkBootDevices", c.hmcIP, viosUUID)

	if debug {
	}

	// Payload constructed exactly matching the verified working configuration
	// Using schemaVersion V1_0 and clean namespace architecture
	payload := fmt.Sprintf(`<JobRequest
  xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
  xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
  <Metadata>
    <Atom/>
  </Metadata>
  <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
    <Metadata>
      <Atom/>
    </Metadata>
    <OperationName kxe="false" kb="ROR">GetNetworkBootDevices</OperationName>
    <GroupName kxe="false" kb="ROR">VirtualIOServer</GroupName>
  </RequestedOperation>
  <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
    <Metadata>
      <Atom/>
    </Metadata>
    <JobParameter schemaVersion="V1_0">
      <Metadata>
        <Atom/>
      </Metadata>
      <ParameterName kb="ROR" kxe="false">LogicalPartitionProfileUUID</ParameterName>
      <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
    </JobParameter>
  </JobParameters>
</JobRequest>`, profileUUID)

	// Create and execute HTTP request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("X-API-Session", c.session)
	// Notice the spacing: 'web+xml; type=JobRequest' exactly as the curl command requires
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("Accept", "*/*")
	
	// Instruct HMC to process using the compatible V1_2_0 schema rules matching the curl command
	req.Header.Set("x-hmc-schema-version", "V1_2_0")

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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

	if debug {
	}

	// Wait for job completion
	jobResp, err := c.FetchJobStatus(ctx, jobID, false, 5, debug)
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
			}

			// 2. Strip Namespaces to make Unmarshaling easy
			cleanDoc, err := xmlStripNamespace([]byte(xmlContent))
			if err != nil {
				if debug {
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
	}

	return bootDevices, nil
}

// CreateClientNetworkAdapter adds a new Virtual Ethernet Adapter to an LPAR and connects it to a Virtual Switch.
// Returns the complete ClientNetworkAdapter structure with all details.
func (c *RestClient) CreateClientNetworkAdapter(ctx context.Context, sysUUID, lparUUID, vswitchUUID string, vlanID int, debug bool) (*ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, lparUUID)

	if debug {
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

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(timeoutCtx)


	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)


	if debug {
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("Adapter creation failed (%s): %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("Adapter creation failed (%s). Enable debug mode to see full response", resp.Status)
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
	}

	return &adapter, nil
}

// DeleteClientNetworkAdapter removes a specific Virtual Ethernet Adapter from an LPAR.
func (c *RestClient) DeleteClientNetworkAdapter(ctx context.Context, lparUUID, adapterUUID string, debug bool) error {
	// The specific endpoint for the adapter we want to delete
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter/%s", c.hmcIP, lparUUID, adapterUUID)

	if debug {
	}

	// Create the DELETE request (No XML body required!)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)


	// A successful DELETE operation typically returns 200 OK or 204 No Content
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if debug {
			return fmt.Errorf("failed to delete adapter (%s): %s", resp.Status, string(body))
		}
		return fmt.Errorf("failed to delete adapter (%s). Enable debug mode to see full response", resp.Status)
	}

	if debug {
	}

	return nil
}
// GetVirtualNetworks retrieves all Virtual Networks configured on a Managed System.
func (c *RestClient) GetVirtualNetworks(ctx context.Context, sysUUID string, debug bool) ([]VirtualNetwork, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualNetwork", c.hmcIP, sysUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode == http.StatusNoContent {
		if debug {
		}
		return []VirtualNetwork{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("failed to fetch Virtual Networks: %s - %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("failed to fetch Virtual Networks: %s. Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces: %v", err)
	}

	var networks []VirtualNetwork

	// 1. Locate all <entry> tags in the Atom feed
	entries := doc.FindElements("//entry")
	
	// 2. Loop through each entry to extract the nested VirtualNetwork payload
	for _, entry := range entries {
		vnetElem := entry.FindElement(".//VirtualNetwork")
		if vnetElem == nil {
			continue
		}

		// Create a new document with the VirtualNetwork element as the root
		entryDoc := etree.NewDocument()
		entryDoc.SetRoot(vnetElem.Copy())
		entryBytes, _ := entryDoc.WriteToBytes()

		var vnet VirtualNetwork
		if err := xml.Unmarshal(entryBytes, &vnet); err != nil {
			continue
		}
		networks = append(networks, vnet)
	}

	if debug {
	}

	return networks, nil
}

// GetVirtualNetwork retrieves the detailed configuration of a specific Virtual Network.
func (c *RestClient) GetVirtualNetwork(ctx context.Context, sysUUID, vnetUUID string, debug bool) (*VirtualNetwork, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualNetwork/%s", c.hmcIP, sysUUID, vnetUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Virtual Network: %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	vnetElem := doc.FindElement("//VirtualNetwork")
	if vnetElem == nil {
		return nil, fmt.Errorf("VirtualNetwork element not found in XML response")
	}

	vnetDoc := etree.NewDocument()
	vnetDoc.SetRoot(vnetElem.Copy())
	vnetBytes, _ := vnetDoc.WriteToBytes()

	var vnet VirtualNetwork
	if err := xml.Unmarshal(vnetBytes, &vnet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VirtualNetwork: %v", err)
	}

	return &vnet, nil
}

// CreateVirtualNetwork provisions a new Virtual Network (VLAN) on the Managed System.
// IBM documentation dictates a PUT method to the base VirtualNetwork endpoint for creation.
func (c *RestClient) CreateVirtualNetwork(ctx context.Context, sysUUID string, req CreateVirtualNetworkRequest, debug bool) (*VirtualNetwork, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualNetwork", c.hmcIP, sysUUID)

	if debug {
	}

	// The HMC strictly enforces XML element order (AssociatedSwitch -> NetworkName -> NetworkVLANID -> TaggedNetwork)
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VirtualNetwork:VirtualNetwork xmlns:VirtualNetwork="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
                               xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
                               schemaVersion="V1_1_0">
    <Metadata><Atom/></Metadata>
    <AssociatedSwitch href="https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s" rel="related"/>
    <NetworkName>%s</NetworkName>
    <NetworkVLANID>%d</NetworkVLANID>
    <TaggedNetwork>%t</TaggedNetwork>
</VirtualNetwork:VirtualNetwork>`, c.hmcIP, sysUUID, req.VSwitchUUID, req.NetworkName, req.NetworkVLANID, req.TaggedNetwork)
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualNetwork")
	httpReq.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("creation failed (%s): %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("creation failed (%s). Enable debug for full response", resp.Status)
	}

	// Parse the response to get the new configuration
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces: %v", err)
	}

	vnetElem := doc.FindElement("//VirtualNetwork")
	if vnetElem == nil {
		return nil, fmt.Errorf("failed to extract VirtualNetwork from response")
	}

	vnetDoc := etree.NewDocument()
	vnetDoc.SetRoot(vnetElem.Copy())
	vnetBytes, _ := vnetDoc.WriteToBytes()

	var vnet VirtualNetwork
	xml.Unmarshal(vnetBytes, &vnet)

	if debug {
	}

	return &vnet, nil
}

// UpdateVirtualNetwork modifies the properties of an existing Virtual Network.
// Note: Per IBM documentation, only the NetworkName property can be modified.
func (c *RestClient) UpdateVirtualNetwork(ctx context.Context, sysUUID, vnetUUID, newName string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualNetwork/%s", c.hmcIP, sysUUID, vnetUUID)

	if debug {
	}

	// 1. Pristine GET to preserve all immutable configuration parameters
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != 200 {
		return fmt.Errorf("pre-flight GET failed: %s", string(rawXML))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	vnetElem := doc.FindElement(".//*[local-name()='VirtualNetwork']")
	if vnetElem == nil {
		return fmt.Errorf("VirtualNetwork element not found in pristine XML")
	}

	// 2. Modify the permitted element
	nameElem := vnetElem.FindElement(".//*[local-name()='NetworkName']")
	if nameElem == nil {
		nameElem = vnetElem.CreateElement("NetworkName")
	}
	nameElem.SetText(newName)

	// 3. POST the update back
	postDoc := etree.NewDocument()
	postDoc.SetRoot(vnetElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualNetwork")
	postReq.Header.Set("Accept", "application/atom+xml")


	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	if postResp.StatusCode >= 400 {
		return fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	if debug {
	}

	return nil
}

// DeleteVirtualNetwork removes a Virtual Network from the Managed System.
// Note: Deletion will fail if the Virtual Network is equivalent to the Trunk Adapter PVID, 
// or if it is still attached to a NetworkBridge.
func (c *RestClient) DeleteVirtualNetwork(ctx context.Context, sysUUID, vnetUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualNetwork/%s", c.hmcIP, sysUUID, vnetUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %v", err)
	}
	
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if debug {
			return fmt.Errorf("failed to delete Virtual Network (%s): %s", resp.Status, string(body))
		}
		return fmt.Errorf("failed to delete Virtual Network (%s). Note: NetworkBridge dependencies must be removed first", resp.Status)
	}

	if debug {
	}

	return nil
}

// GetVirtualSwitch retrieves the detailed XML configuration of a specific Virtual Switch.
func (c *RestClient) GetVirtualSwitch(ctx context.Context, sysUUID, switchUUID string, debug bool) (*VirtualSwitch, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s", c.hmcIP, sysUUID, switchUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Virtual Switch: %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	vswitchElem := doc.FindElement("//VirtualSwitch")
	if vswitchElem == nil {
		return nil, fmt.Errorf("VirtualSwitch element not found in XML response")
	}

	vSwitch := VirtualSwitch{}

	// Extract Core properties using etree to match your existing struct
	if atomID := vswitchElem.FindElement(".//AtomID"); atomID != nil {
		vSwitch.UUID = atomID.Text()
	}
	if switchID := vswitchElem.FindElement(".//SwitchID"); switchID != nil {
		vSwitch.SwitchID = switchID.Text()
	}
	if switchName := vswitchElem.FindElement(".//SwitchName"); switchName != nil {
		vSwitch.SwitchName = switchName.Text()
	}
	if switchMode := vswitchElem.FindElement(".//SwitchMode"); switchMode != nil {
		vSwitch.SwitchMode = switchMode.Text()
	}

	// Extract attached VirtualNetwork Links
	for _, link := range vswitchElem.FindElements(".//VirtualNetworks/link") {
		if href := link.SelectAttrValue("href", ""); href != "" {
			vSwitch.VirtualNetworks = append(vSwitch.VirtualNetworks, href)
		}
	}

	if debug {
	}

	return &vSwitch, nil
}

// CreateVirtualSwitch provisions a new Virtual Switch on the Managed System.
func (c *RestClient) CreateVirtualSwitch(ctx context.Context, sysUUID string, req CreateVirtualSwitchRequest, debug bool) (*VirtualSwitch, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch", c.hmcIP, sysUUID)

	if debug {
	}

	// Default to Veb if not specified
	mode := "Veb"
	if req.SwitchMode != "" {
		mode = req.SwitchMode
	}

	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<VirtualSwitch:VirtualSwitch xmlns:VirtualSwitch="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
                             xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
                             schemaVersion="V1_1_0">
    <Metadata><Atom/></Metadata>
    <SwitchName>%s</SwitchName>
    <SwitchMode>%s</SwitchMode>
</VirtualSwitch:VirtualSwitch>`, req.SwitchName, mode)

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualSwitch")
	httpReq.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("creation failed (%s): %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("creation failed (%s). Enable debug for full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces: %v", err)
	}

	vswitchElem := doc.FindElement("//VirtualSwitch")
	if vswitchElem == nil {
		return nil, fmt.Errorf("failed to extract VirtualSwitch from response")
	}

	vSwitch := VirtualSwitch{}

	if atomID := vswitchElem.FindElement(".//AtomID"); atomID != nil {
		vSwitch.UUID = atomID.Text()
	}
	if switchID := vswitchElem.FindElement(".//SwitchID"); switchID != nil {
		vSwitch.SwitchID = switchID.Text()
	}
	if switchName := vswitchElem.FindElement(".//SwitchName"); switchName != nil {
		vSwitch.SwitchName = switchName.Text()
	}
	if switchMode := vswitchElem.FindElement(".//SwitchMode"); switchMode != nil {
		vSwitch.SwitchMode = switchMode.Text()
	}

	if debug {
	}

	return &vSwitch, nil
}

// UpdateVirtualSwitch modifies the properties of an existing Virtual Switch.
// According to IBM docs, only SwitchName and SwitchMode are modifiable.
func (c *RestClient) UpdateVirtualSwitch(ctx context.Context, sysUUID, switchUUID, newName, newMode string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s", c.hmcIP, sysUUID, switchUUID)

	if debug {
	}

	// 1. Pristine GET to preserve all immutable configuration parameters
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != http.StatusOK {
		return fmt.Errorf("pre-flight GET failed: %s", string(rawXML))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	vswitchElem := doc.FindElement(".//*[local-name()='VirtualSwitch']")
	if vswitchElem == nil {
		return fmt.Errorf("VirtualSwitch element not found in pristine XML")
	}

	// 2. Modify the permitted elements
	if newName != "" {
		nameElem := vswitchElem.FindElement(".//*[local-name()='SwitchName']")
		if nameElem == nil {
			nameElem = vswitchElem.CreateElement("SwitchName")
		}
		nameElem.SetText(newName)
	}

	if newMode != "" {
		modeElem := vswitchElem.FindElement(".//*[local-name()='SwitchMode']")
		if modeElem == nil {
			modeElem = vswitchElem.CreateElement("SwitchMode")
		}
		modeElem.SetText(newMode)
	}

	// 3. POST the update back
	postDoc := etree.NewDocument()
	postDoc.SetRoot(vswitchElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=VirtualSwitch")
	postReq.Header.Set("Accept", "application/atom+xml")


	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	if postResp.StatusCode >= 400 {
		return fmt.Errorf("POST failed (%s): %s", postResp.Status, string(body))
	}

	if debug {
	}

	return nil
}

// DeleteVirtualSwitch removes a Virtual Switch from the Managed System.
func (c *RestClient) DeleteVirtualSwitch(ctx context.Context, sysUUID, switchUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualSwitch/%s", c.hmcIP, sysUUID, switchUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if debug {
			return fmt.Errorf("failed to delete Virtual Switch (%s): %s", resp.Status, string(body))
		}
		return fmt.Errorf("failed to delete Virtual Switch (%s). Ensure it has no dependent Virtual Networks", resp.Status)
	}

	if debug {
	}

	return nil
}


// GetNetworkBridges retrieves all Network Bridges configured on a Managed System.
func (c *RestClient) GetNetworkBridges(ctx context.Context, sysUUID string, debug bool) ([]NetworkBridge, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/NetworkBridge", c.hmcIP, sysUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode == http.StatusNoContent {
		if debug {
		}
		return []NetworkBridge{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("failed to fetch Network Bridges: %s - %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("failed to fetch Network Bridges: %s. Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces: %v", err)
	}

	var bridges []NetworkBridge

	// 1. Locate all <entry> tags in the Atom feed
	entries := doc.FindElements("//entry")
	
	// 2. Loop through each entry to extract the nested NetworkBridge payload
	for _, entry := range entries {
		bridgeElem := entry.FindElement(".//NetworkBridge")
		if bridgeElem == nil {
			continue
		}

		entryDoc := etree.NewDocument()
		entryDoc.SetRoot(bridgeElem.Copy())
		entryBytes, _ := entryDoc.WriteToBytes()

		var bridge NetworkBridge
		if err := xml.Unmarshal(entryBytes, &bridge); err != nil {
			continue
		}
		bridges = append(bridges, bridge)
	}

	if debug {
	}

	return bridges, nil
}

// GetNetworkBridge retrieves the detailed configuration of a specific Network Bridge.
func (c *RestClient) GetNetworkBridge(ctx context.Context, sysUUID, bridgeUUID string, debug bool) (*NetworkBridge, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/NetworkBridge/%s", c.hmcIP, sysUUID, bridgeUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Network Bridge: %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	bridgeElem := doc.FindElement("//NetworkBridge")
	if bridgeElem == nil {
		return nil, fmt.Errorf("NetworkBridge element not found in XML response")
	}

	bridgeDoc := etree.NewDocument()
	bridgeDoc.SetRoot(bridgeElem.Copy())
	bridgeBytes, _ := bridgeDoc.WriteToBytes()

	var bridge NetworkBridge
	if err := xml.Unmarshal(bridgeBytes, &bridge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NetworkBridge: %v", err)
	}

	return &bridge, nil
}


// CreateNetworkBridge provisions a new Network Bridge (Shared Ethernet Adapter wrapper).
// It automatically supports both standard Active/Standby HA Failover and Active/Active Load Balancing layouts.
func (c *RestClient) CreateNetworkBridge(ctx context.Context, sysUUID string, req CreateNetworkBridgeRequest, debug bool) (*NetworkBridge, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/NetworkBridge", c.hmcIP, sysUUID)

	if debug {
	}

	// 1. Setup the collection of Load Group VLANs (default to root PortVLANID if none provided)
	vlans := req.LoadGroupVLANs
	if len(vlans) == 0 {
		vlans = []int{req.PortVLANID}
	}

	if req.LoadBalancingEnabled && len(vlans) < 2 {
		return nil, fmt.Errorf("at least two data load group VLAN IDs must be provided via LoadGroupVLANs when LoadBalancingEnabled is true")
	}

	// 2. Query all Virtual Networks once from the HMC to optimize dynamic identifier matching
	vNets, err := c.GetVirtualNetworks(ctx, sysUUID, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Virtual Networks for infrastructure alignment: %v", err)
	}

	// 3. Dynamically assemble the ordered sequential LoadGroups child elements
	var loadGroupsSB strings.Builder
	loadGroupsSB.WriteString("<LoadGroups schemaVersion=\"V1_0\">")
	
	for _, vlan := range vlans {
		var vNetUUID string
		for _, vn := range vNets {
			if vn.NetworkVLANID == vlan {
				vNetUUID = vn.UUID
				break
			}
		}
		if vNetUUID == "" {
			return nil, fmt.Errorf("failed to find an existing Virtual Network matching VLAN ID %d; please create it first", vlan)
		}

		loadGroupsSB.WriteString(fmt.Sprintf(`
        <LoadGroup schemaVersion="V1_0">
            <PortVLANID>%d</PortVLANID>
            <VirtualNetworks>
                <link href="https://%s:443/rest/api/uom/ManagedSystem/%s/VirtualNetwork/%s" rel="related"/>
            </VirtualNetworks>
        </LoadGroup>`, vlan, c.hmcIP, sysUUID, vNetUUID))
	}
	loadGroupsSB.WriteString("\n    </LoadGroups>")
	loadGroupsXML := loadGroupsSB.String()

	// 4. Construct optional Failover HA payload blocks (Strict JAXB Sorting Rules)
	controlChannelXML := ""
	secondarySEAXML := ""
	if req.FailoverEnabled {
		if req.ControlChannelID <= 0 {
			return nil, fmt.Errorf("ControlChannelID must be specified and > 0 when FailoverEnabled is true")
		}
		if req.SecondaryViosUUID == "" || req.SecondaryBackingDevice == "" {
			return nil, fmt.Errorf("Secondary VIOS UUID and Backing Device are required for Failover configurations")
		}
		controlChannelXML = fmt.Sprintf("<ControlChannelID>%d</ControlChannelID>", req.ControlChannelID)
		
		// Order sequence: AssignedVirtualIOServer -> BackingDeviceChoice -> JumboFramesEnabled -> IsPrimary -> LargeSend
		secondarySEAXML = fmt.Sprintf(`
        <SharedEthernetAdapter schemaVersion="V1_0">
            <AssignedVirtualIOServer href="https://%s:443/rest/api/uom/VirtualIOServer/%s" rel="related"/>
            <BackingDeviceChoice>
                <EthernetBackingDevice schemaVersion="V1_0">
                    <DeviceName>%s</DeviceName>
                </EthernetBackingDevice>
            </BackingDeviceChoice>
            <JumboFramesEnabled>%t</JumboFramesEnabled>
            <IsPrimary>false</IsPrimary>
            <LargeSend>%t</LargeSend>
        </SharedEthernetAdapter>`, c.hmcIP, req.SecondaryViosUUID, req.SecondaryBackingDevice, req.JumboFramesEnabled, req.LargeSend)
	}

	// 5. Construct Final Structural XML Representation Payload (Root Alphabetical Sequence)
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<NetworkBridge:NetworkBridge xmlns:NetworkBridge="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
                             xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/"
                             schemaVersion="V1_1_0">
    <Metadata><Atom/></Metadata>
    %s
    <FailoverEnabled>%t</FailoverEnabled>
    <LoadBalancingEnabled>%t</LoadBalancingEnabled>
    %s
    <PortVLANID>%d</PortVLANID>
    <SharedEthernetAdapters schemaVersion="V1_0">
        <SharedEthernetAdapter schemaVersion="V1_0">
            <AssignedVirtualIOServer href="https://%s:443/rest/api/uom/VirtualIOServer/%s" rel="related"/>
            <BackingDeviceChoice>
                <EthernetBackingDevice schemaVersion="V1_0">
                    <DeviceName>%s</DeviceName>
                </EthernetBackingDevice>
            </BackingDeviceChoice>
            <JumboFramesEnabled>%t</JumboFramesEnabled>
            <IsPrimary>true</IsPrimary>
            <LargeSend>%t</LargeSend>
        </SharedEthernetAdapter>
        %s
    </SharedEthernetAdapters>
</NetworkBridge:NetworkBridge>`,
		controlChannelXML, req.FailoverEnabled, req.LoadBalancingEnabled,
		loadGroupsXML,
		req.PortVLANID,
		c.hmcIP, req.PrimaryViosUUID, req.PrimaryBackingDevice, req.JumboFramesEnabled, req.LargeSend,
		secondarySEAXML)

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=NetworkBridge")
	httpReq.Header.Set("Accept", "application/atom+xml")


	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		if debug {
			return nil, fmt.Errorf("NetworkBridge creation failed (%s): %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("NetworkBridge creation failed (%s)", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	bridgeElem := doc.FindElement("//NetworkBridge")
	if bridgeElem == nil {
		return nil, fmt.Errorf("failed to extract NetworkBridge structural entity from response payload")
	}

	bridgeDoc := etree.NewDocument()
	bridgeDoc.SetRoot(bridgeElem.Copy())
	bridgeBytes, _ := bridgeDoc.WriteToBytes()

	var bridge NetworkBridge
	if err := xml.Unmarshal(bridgeBytes, &bridge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provisioned configuration profile mappings: %v", err)
	}

	return &bridge, nil
}

// UpdateNetworkBridge modifies properties of an active asset utilizing a pristine GET-POST workflow transaction pattern.
func (c *RestClient) UpdateNetworkBridge(ctx context.Context, sysUUID, bridgeUUID string, failoverEnabled, loadBalancingEnabled, largeSend, jumboFrames bool, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/NetworkBridge/%s", c.hmcIP, sysUUID, bridgeUUID)

	if debug {
	}

	// 1. Fetch current layout definition state
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()
	rawXML, _ := io.ReadAll(getResp.Body)

	if getResp.StatusCode != http.StatusOK {
		return fmt.Errorf("pre-flight GET configuration tree inquiry failed: %s", string(rawXML))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return fmt.Errorf("failed to digest active response mapping tree configurations: %v", err)
	}

	bridgeElem := doc.FindElement(".//*[local-name()='NetworkBridge']")
	if bridgeElem == nil {
		return fmt.Errorf("NetworkBridge context tag node tracking reference could not be recovered")
	}

	// 2. Adjust root scalar settings
	if foElem := bridgeElem.FindElement(".//*[local-name()='FailoverEnabled']"); foElem != nil {
		foElem.SetText(fmt.Sprintf("%t", failoverEnabled))
	}
	if lbElem := bridgeElem.FindElement(".//*[local-name()='LoadBalancingEnabled']"); lbElem != nil {
		lbElem.SetText(fmt.Sprintf("%t", loadBalancingEnabled))
	}

	// 3. Update nested loops elements
	seas := bridgeElem.FindElements(".//*[local-name()='SharedEthernetAdapter']")
	for _, sea := range seas {
		if lsElem := sea.FindElement(".//*[local-name()='LargeSend']"); lsElem != nil {
			lsElem.SetText(fmt.Sprintf("%t", largeSend))
		} else {
			sea.CreateElement("LargeSend").SetText(fmt.Sprintf("%t", largeSend))
		}

		if jfElem := sea.FindElement(".//*[local-name()='JumboFramesEnabled']"); jfElem != nil {
			jfElem.SetText(fmt.Sprintf("%t", jumboFrames))
		} else {
			sea.CreateElement("JumboFramesEnabled").SetText(fmt.Sprintf("%t", jumboFrames))
		}
	}

	// 4. Dispatch the updated data tree
	postDoc := etree.NewDocument()
	postDoc.SetRoot(bridgeElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=NetworkBridge")
	postReq.Header.Set("Accept", "application/atom+xml")


	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	if postResp.StatusCode >= 400 {
		return fmt.Errorf("POST layout specification modifications rejected (%s): %s", postResp.Status, string(body))
	}

	return nil
}
// DeleteNetworkBridge removes a Network Bridge from the Managed System.
// This automatically breaks down the underlying SEA on the VIOS.
func (c *RestClient) DeleteNetworkBridge(ctx context.Context, sysUUID, bridgeUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/NetworkBridge/%s", c.hmcIP, sysUUID, bridgeUUID)

	if debug {
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %v", err)
	}
	
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")


	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if debug {
			return fmt.Errorf("failed to delete Network Bridge (%s): %s", resp.Status, string(body))
		}
		return fmt.Errorf("failed to delete Network Bridge (%s)", resp.Status)
	}

	if debug {
	}

	return nil
}

// GetNetworkBootDevicesForViosImmediate retrieves network boot devices from a VIOS profile
// using the HMC REST API job parameterized by system name, lpar name, and profile name.
// This executes the job using the "withImmediate" flag.
//
// Parameters:
//   - viosUUID: UUID of the Virtual I/O Server (used for the endpoint URL)
//   - sysName: The managed system name (e.g., "hmc-denali5")
//   - viosName: The VIOS LPAR name (e.g., "denali5-vios1")
//   - profileName: The profile name (e.g., "default")
//   - loggedInUser: The HMC user executing the request (e.g., "REDACTED_HMC_USER<==")
//   - debug: Enable detailed logging
//
// Returns:
//   - []NetworkBootDevice: List of network boot devices configured in the profile
//   - error: Error if the operation fails
func (c *RestClient) GetNetworkBootDevicesForViosImmediate(ctx context.Context, viosUUID, sysName, viosName, profileName, loggedInUser string, debug bool) ([]NetworkBootDevice, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s/do/GetNetworkBootDevices", c.hmcIP, viosUUID)

	if debug {
	}

	// Payload constructed exactly matching your working curl command
	payload := fmt.Sprintf(`<JobRequest
  xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
  xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
  <Metadata>
    <Atom/>
  </Metadata>
  <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
    <Metadata>
      <Atom/>
    </Metadata>
    <OperationName kxe="false" kb="ROR">GetNetworkBootDevices</OperationName>
    <GroupName kxe="false" kb="ROR">VirtualIOServer</GroupName>
  </RequestedOperation>
  <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
    <Metadata>
      <Atom/>
    </Metadata>
    <JobParameter schemaVersion="V1_0">
      <Metadata>
        <Atom/>
      </Metadata>
      <ParameterName kb="ROR" kxe="false">managedSystemName</ParameterName>
      <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
    </JobParameter>
    <JobParameter schemaVersion="V1_0">
      <Metadata>
        <Atom/>
      </Metadata>
      <ParameterName kb="ROR" kxe="false">laparName</ParameterName>
      <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
    </JobParameter>
    <JobParameter schemaVersion="V1_0">
      <Metadata>
        <Atom/>
      </Metadata>
      <ParameterName kb="ROR" kxe="false">profileName</ParameterName>
      <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
    </JobParameter>
    <JobParameter schemaVersion="V1_0">
      <Metadata>
        <Atom/>
      </Metadata>
      <ParameterName kb="ROR" kxe="false">loggedinuser</ParameterName>
      <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
    </JobParameter>
  </JobParameters>
</JobRequest>`, sysName, viosName, profileName, loggedInUser)

	// Create and execute HTTP request
	req, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("Accept", "*/*")
	// Intentionally omitting x-hmc-schema-version to match working curl

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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

	if debug {
	}

	// Wait for job completion
	jobResp, err := c.FetchJobStatus(ctx, jobID, false, 5, debug)
	if err != nil {
		return nil, fmt.Errorf("GetNetworkBootDevices job failed: %v", err)
	}

	// Parse job results to extract network boot devices
	var bootDevices []NetworkBootDevice

	for _, param := range jobResp.Results.Parameters {
		if param.ParameterName == "result" {
			xmlContent := param.ParameterValue

			// Clean the CDATA wrapper and unescape any HTML entities
			xmlContent = strings.TrimSpace(xmlContent)
			xmlContent = strings.TrimPrefix(xmlContent, "<![CDATA[")
			xmlContent = strings.TrimSuffix(xmlContent, "]]>")
			xmlContent = html.UnescapeString(strings.TrimSpace(xmlContent))

			cleanDoc, err := xmlStripNamespace([]byte(xmlContent))
			if err != nil {
				continue
			}
			strippedXML, _ := cleanDoc.WriteToBytes()

			var collection struct {
				Devices []struct {
					BootDevice       string `xml:"BootDevice"`
					IsPhysicalDevice bool   `xml:"IsPhysicalDevice"`
					LocationCode     string `xml:"LocationCode"`
					MACAddressValue  string `xml:"MACAddressValue"`
				} `xml:"NetworkBootDevice"`
			}

			if err := xml.Unmarshal(strippedXML, &collection); err != nil {
				continue
			}

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

	return bootDevices, nil
}