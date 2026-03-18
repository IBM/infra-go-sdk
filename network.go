package hmc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

// GetClientNetworkAdapters retrieves all ClientNetworkAdapters (Virtual Ethernet Adapters) for a specific LPAR.
func (c *HmcRestClient) GetClientNetworkAdapters(lparUUID string, verbose bool) ([]ClientNetworkAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, lparUUID)

	if verbose {
		hmcLogger.Printf("Fetching ClientNetworkAdapters for LPAR %s...", lparUUID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

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
		return nil, fmt.Errorf("failed to fetch ClientNetworkAdapters: %s - %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, err
	}

	var adapters []ClientNetworkAdapter

	// Loop through every <entry> in the Atom feed
	for _, entry := range doc.FindElements("//entry") {
		adapter := ClientNetworkAdapter{}

		// 1. Core Properties
		if el := entry.FindElement(".//AtomID"); el != nil { adapter.UUID = el.Text() }
		if el := entry.FindElement(".//DynamicReconfigurationConnectorName"); el != nil { adapter.DynamicReconfigurationConnectorName = el.Text() }
		if el := entry.FindElement(".//LocationCode"); el != nil { adapter.LocationCode = el.Text() }
		if el := entry.FindElement(".//LocalPartitionID"); el != nil { adapter.LocalPartitionID = el.Text() }
		if el := entry.FindElement(".//RequiredAdapter"); el != nil { adapter.RequiredAdapter = el.Text() }
		if el := entry.FindElement(".//VariedOn"); el != nil { adapter.VariedOn = el.Text() }
		if el := entry.FindElement(".//VirtualSlotNumber"); el != nil { adapter.VirtualSlotNumber = el.Text() }
		if el := entry.FindElement(".//AllowedOperatingSystemMACAddresses"); el != nil { adapter.AllowedOperatingSystemMACAddresses = el.Text() }
		if el := entry.FindElement(".//MACAddress"); el != nil { adapter.MACAddress = el.Text() }
		if el := entry.FindElement(".//PortVLANID"); el != nil { adapter.PortVLANID = el.Text() }
		if el := entry.FindElement(".//QualityOfServicePriorityEnabled"); el != nil { adapter.QualityOfServicePriorityEnabled = el.Text() }
		if el := entry.FindElement(".//TaggedVLANSupported"); el != nil { adapter.TaggedVLANSupported = el.Text() }
		if el := entry.FindElement(".//VirtualSwitchID"); el != nil { adapter.VirtualSwitchID = el.Text() }
		if el := entry.FindElement(".//VirtualSwitchName"); el != nil { adapter.VirtualSwitchName = el.Text() }
		if el := entry.FindElement(".//HCNID"); el != nil { adapter.HCNID = el.Text() }

		// 2. Extract AssociatedVirtualSwitch URI
		vSwitchLink := entry.FindElement(".//AssociatedVirtualSwitch/link")
		if vSwitchLink != nil {
			adapter.AssociatedVirtualSwitchURI = vSwitchLink.SelectAttrValue("href", "")
		}

		// 3. Extract VirtualNetwork URIs
		for _, vNetLink := range entry.FindElements(".//VirtualNetworks/link") {
			if href := vNetLink.SelectAttrValue("href", ""); href != "" {
				adapter.VirtualNetworkURIs = append(adapter.VirtualNetworkURIs, href)
			}
		}

		adapters = append(adapters, adapter)
	}

	if verbose {
		hmcLogger.Printf("Found %d ClientNetworkAdapter(s)", len(adapters))
	}

	return adapters, nil
}

// CreateClientNetworkAdapter adds a new Virtual Ethernet Adapter to an LPAR and connects it to a Virtual Switch.
func (c *HmcRestClient) CreateClientNetworkAdapter(sysUUID, lparUUID, vswitchUUID string, vlanID int, verbose bool) (string, error) {
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
		return "", err
	}

	httpReq.Header.Set("X-API-Session", c.session)
	httpReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ClientNetworkAdapter")
	httpReq.Header.Set("Accept", "application/atom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Adapter creation failed (%s): %s", resp.Status, string(body))
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", err
	}

	atomID := doc.FindElement("//AtomID")
	if atomID == nil {
		return "", fmt.Errorf("Adapter created successfully, but failed to extract new UUID")
	}

	newUUID := atomID.Text()
	if verbose {
		hmcLogger.Printf("✅ Client Network Adapter created! UUID: %s", newUUID)
	}

	return newUUID, nil
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