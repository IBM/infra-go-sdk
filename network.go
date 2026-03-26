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