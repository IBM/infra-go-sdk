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

// LogonRequest represents the XML payload for HMC logon
type LogonRequest struct {
	XMLName       xml.Name `xml:"LogonRequest"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	XMLNS         string   `xml:"xmlns,attr"`
	XMLNSMC       string   `xml:"xmlns:mc,attr"`
	UserID        string   `xml:"UserID"`
	Password      string   `xml:"Password"`
}

// LogonResponse represents the XML response for HMC logon
type LogonResponse struct {
	XMLName xml.Name `xml:"LogonResponse"`
	Session string   `xml:"X-API-Session"`
}

// AtomFeed represents the Atom feed structure for PartitionTemplate
type AtomFeed struct {
	XMLName xml.Name         `xml:"http://www.w3.org/2005/Atom feed"`
	Entries []PartitionEntry `xml:"entry"`
}

// PartitionEntry represents a single PartitionTemplate entry in the feed
type PartitionEntry struct {
	XMLName           xml.Name          `xml:"entry"`
	ID                string            `xml:"id"`
	PartitionTemplate PartitionTemplate `xml:"content>PartitionTemplateSummary"`
}

// PartitionTemplate represents the PartitionTemplateSummary content
type PartitionTemplate struct {
	XMLName xml.Name `xml:"http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/ PartitionTemplateSummary"`
	AtomID  string   `xml:"Metadata>Atom>AtomID"`
	Name    string   `xml:"partitionTemplateName"`
	Content string   `xml:",innerxml"` // Capture full XML content
}

// System represents the ManagedSystem content
type System struct {
	XMLName       xml.Name `xml:"http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/ ManagedSystem"`
	MaxPartitions string   `xml:"MaximumPartitions"`
	SystemName    string   `xml:"SystemName"`
	SerialNumber  string   `xml:"MachineTypeModelAndSerialNumber>SerialNumber"`
}

// JobResponse represents the XML response for a job operation
type JobResponse struct {
	XMLName xml.Name `xml:"JobResponse"`
	JobID   string   `xml:"JobID"`
	Status  string   `xml:"Status"`
}

// Logger with prefix for HMC operations
var hmcLogger = log.New(log.Writer(), "[HMC] ", log.LstdFlags)

// HmcRestClient represents a client for interacting with the HMC REST API
type HmcRestClient struct {
	hmcIP   string
	session string
	client  *http.Client
}

// NewHmcRestClient initializes a new HmcRestClient
func NewHmcRestClient(hmcIP string, client *http.Client) *HmcRestClient {
	return &HmcRestClient{
		hmcIP:  hmcIP,
		client: client,
	}
}

// Session returns the current session token
func (c *HmcRestClient) Session() string {
	return c.session
}

// Login performs the logon operation to the HMC REST API
func (c *HmcRestClient) Login(username, password string, verbose bool) error {
	payload := LogonRequest{
		SchemaVersion: "V1_0",
		XMLNS:         "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		XMLNSMC:       "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/",
		UserID:        username,
		Password:      password,
	}
	xmlData, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("XML marshal failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Sending logon request to https://%s/rest/api/web/Logon", c.hmcIP)
		hmcLogger.Printf("Logon request payload:\n%s", string(xmlData))
	}

	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(xmlData))
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.SetBasicAuth(username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Logon response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Logon response body:\n%s", string(body))
	}

	var logonResp LogonResponse
	if err := xml.Unmarshal(body, &logonResp); err != nil {
		return fmt.Errorf("XML unmarshal failed: %v", err)
	}

	c.session = logonResp.Session
	return nil
}

// Logoff performs the logoff operation from the HMC REST API
func (c *HmcRestClient) Logoff() error {
	if c.session == "" {
		return nil // No session to log off
	}
	url := fmt.Sprintf("https://%s/rest/api/web/Logon", c.hmcIP)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.Header.Set("Authorization", "Basic Og==")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("logoff failed with status: %s", resp.Status)
	}
	c.session = ""
	return nil
}

// GetManagedSystem fetches the managed system UUID and details by name
func (c *HmcRestClient) GetManagedSystem(systemName string, verbose bool) (string, *etree.Element, error) {
	if systemName == "" {
		return "", nil, fmt.Errorf("systemName cannot be empty")
	}
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/search/(SystemName=='%s')", c.hmcIP, systemName)
	if verbose {
		hmcLogger.Printf("Fetching managed system for name: %s, URL: %s", systemName, url)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetManagedSystem response status: %s", resp.Status)
	}

	if resp.StatusCode == 204 {
		if verbose {
			hmcLogger.Printf("No managed system found for name: %s", systemName)
		}
		return "", nil, nil // No content found
	}
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetManagedSystem response body:\n%s", string(body))
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(string(body)); err != nil {
		return "", nil, fmt.Errorf("failed to parse XML response: %v", err)
	}

	uuidElem := doc.FindElement("//AtomID")
	if uuidElem == nil {
		return "", nil, fmt.Errorf("AtomID not found in response")
	}
	uuid := uuidElem.Text()

	msElem := doc.FindElement("//ManagedSystem")
	if msElem == nil {
		return "", nil, fmt.Errorf("ManagedSystem not found in response")
	}

	return uuid, msElem, nil
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

// UpdateLparNameAndIDToDom updates the partition ID, name, and max virtual slots in the XML document
func (c *HmcRestClient) UpdateLparNameAndIDToDom(templateXML *etree.Element, configDict map[string]string) error {
	// Handle partitionId
	lparIDElements := templateXML.FindElements("//partitionId")
	if len(lparIDElements) > 0 {
		if lparID, ok := configDict["lpar_id"]; ok {
			lparIDElements[0].SetText(lparID)
		} else {
			// Remove the partitionId element if lpar_id is not in configDict
			parent := lparIDElements[0].Parent()
			if parent != nil {
				parent.RemoveChild(lparIDElements[0])
			}
		}
	} else {
		return fmt.Errorf("partitionId element not found in XML")
	}

	// Set currMaxVirtualIOSlots
	maxSlotsElements := templateXML.FindElements("//currMaxVirtualIOSlots")
	if len(maxSlotsElements) > 0 {
		if maxSlots, ok := configDict["max_virtual_slots"]; ok {
			maxSlotsElements[0].SetText(maxSlots)
		} else {
			return fmt.Errorf("max_virtual_slots not found in configDict")
		}
	} else {
		return fmt.Errorf("currMaxVirtualIOSlots element not found in XML")
	}

	// Set partitionName
	partitionNameElements := templateXML.FindElements("//partitionName")
	if len(partitionNameElements) > 0 {
		if vmName, ok := configDict["vm_name"]; ok {
			partitionNameElements[0].SetText(vmName)
		} else {
			return fmt.Errorf("vm_name not found in configDict")
		}
	} else {
		return fmt.Errorf("partitionName element not found in XML")
	}

	return nil
}

// UpdateProcMemSettingsToDom updates processor and memory settings in the XML document
// UpdateProcMemSettingsToDom updates processor and memory settings in the XML document
func (c *HmcRestClient) UpdateProcMemSettingsToDom(templateXML *etree.Element, configDict map[string]string) error {
	// Shared processor configuration
	if procUnit, ok := configDict["proc_unit"]; ok && procUnit != "" {
		sharedPayload := fmt.Sprintf(`<sharedProcessorConfiguration kxe="false" kb="CUD" schemaVersion="V1_0">
			<Metadata>
				<Atom/>
			</Metadata>
			<sharedProcessorPoolId kxe="false" kb="CUD">%s</sharedProcessorPoolId>
			<uncappedWeight kxe="false" kb="CUD">%s</uncappedWeight>
			<minProcessingUnits kb="CUD" kxe="false">%s</minProcessingUnits>
			<desiredProcessingUnits kxe="false" kb="CUD">%s</desiredProcessingUnits>
			<maxProcessingUnits kb="CUD" kxe="false">%s</maxProcessingUnits>
			<minVirtualProcessors kb="CUD" kxe="false">%s</minVirtualProcessors>
			<desiredVirtualProcessors kxe="false" kb="CUD">%s</desiredVirtualProcessors>
			<maxVirtualProcessors kxe="false" kb="CUD">%s</maxVirtualProcessors>
		</sharedProcessorConfiguration>`,
			configDict["shared_proc_pool"],
			configDict["weight"],
			configDict["min_proc_unit"],
			configDict["proc_unit"],
			configDict["max_proc_unit"],
			configDict["min_proc"],
			configDict["proc"],
			configDict["max_proc"])

		// Remove existing sharedProcessorConfiguration if present
		sharedConfigTags := templateXML.FindElements("//sharedProcessorConfiguration")
		for _, tag := range sharedConfigTags {
			if parent := tag.Parent(); parent != nil {
				parent.RemoveChild(tag)
			}
		}

		// Add new sharedProcessorConfiguration after sharingMode
		sharingModeTag := templateXML.FindElement("//sharingMode")
		if sharingModeTag == nil {
			return fmt.Errorf("sharingMode element not found in XML")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromString(sharedPayload); err != nil {
			return fmt.Errorf("failed to parse shared processor configuration XML: %v", err)
		}
		sharedConfigElement := doc.Root()
		if sharedConfigElement == nil {
			return fmt.Errorf("failed to parse shared processor configuration XML: no root element")
		}
		if parent := sharingModeTag.Parent(); parent != nil {
			// Loop through the parent's children to find sharingModeTag's index
			for i, child := range parent.Child {
				if child == sharingModeTag {
					// Insert sharedConfigElement immediately after sharingModeTag
					fmt.Printf("Type of sharedConfigElement: %T\n", sharedConfigElement)
					parent.InsertChildAt(i+1, sharedConfigElement)
					break
				}
			}
		} else {
			return fmt.Errorf("sharingMode element has no parent")
		}

		// Remove dedicatedProcessorConfiguration if present
		dediTags := templateXML.FindElements("//dedicatedProcessorConfiguration")
		for _, tag := range dediTags {
			if parent := tag.Parent(); parent != nil {
				parent.RemoveChild(tag)
			}
		}

		// Update currHasDedicatedProcessors and currSharingMode
		currHasDedicatedProcessors := templateXML.FindElement("//currHasDedicatedProcessors")
		if currHasDedicatedProcessors == nil {
			return fmt.Errorf("currHasDedicatedProcessors element not found in XML")
		}
		currHasDedicatedProcessors.SetText("false")

		currSharingMode := templateXML.FindElement("//currSharingMode")
		if currSharingMode == nil {
			return fmt.Errorf("currSharingMode element not found in XML")
		}
		if procMode, ok := configDict["proc_mode"]; ok {
			currSharingMode.SetText(procMode)
		} else {
			return fmt.Errorf("proc_mode not found in configDict")
		}
	} else {
		// Dedicated processor configuration
		minProcs := templateXML.FindElement("//minProcessors")
		if minProcs == nil {
			return fmt.Errorf("minProcessors element not found in XML")
		}
		if minProc, ok := configDict["min_proc"]; ok {
			minProcs.SetText(minProc)
		} else {
			return fmt.Errorf("min_proc not found in configDict")
		}

		desiredProcs := templateXML.FindElement("//desiredProcessors")
		if desiredProcs == nil {
			return fmt.Errorf("desiredProcessors element not found in XML")
		}
		if proc, ok := configDict["proc"]; ok {
			desiredProcs.SetText(proc)
		} else {
			return fmt.Errorf("proc not found in configDict")
		}

		maxProcs := templateXML.FindElement("//maxProcessors")
		if maxProcs == nil {
			return fmt.Errorf("maxProcessors element not found in XML")
		}
		if maxProc, ok := configDict["max_proc"]; ok {
			maxProcs.SetText(maxProc)
		} else {
			return fmt.Errorf("max_proc not found in configDict")
		}
	}

	// Update memory settings
	currMinMemory := templateXML.FindElement("//currMinMemory")
	if currMinMemory == nil {
		return fmt.Errorf("currMinMemory element not found in XML")
	}
	if minMem, ok := configDict["min_mem"]; ok {
		currMinMemory.SetText(minMem)
	} else {
		return fmt.Errorf("min_mem not found in configDict")
	}

	currMemory := templateXML.FindElement("//currMemory")
	if currMemory == nil {
		return fmt.Errorf("currMemory element not found in XML")
	}
	if mem, ok := configDict["mem"]; ok {
		currMemory.SetText(mem)
	} else {
		return fmt.Errorf("mem not found in configDict")
	}

	currMaxMemory := templateXML.FindElement("//currMaxMemory")
	if currMaxMemory == nil {
		return fmt.Errorf("currMaxMemory element not found in XML")
	}
	if maxMem, ok := configDict["max_mem"]; ok {
		currMaxMemory.SetText(maxMem)
	} else {
		return fmt.Errorf("max_mem not found in configDict")
	}

	// Update processor compatibility mode if provided
	if procCompMode, ok := configDict["proc_comp_mode"]; ok && procCompMode != "" {
		currProcCompMode := templateXML.FindElement("//currProcessorCompatibilityMode")
		if currProcCompMode == nil {
			return fmt.Errorf("currProcessorCompatibilityMode element not found in XML")
		}
		currProcCompMode.SetText(procCompMode)
	}

	return nil
}

// CreatePartition creates a partition using a template UUID
func (c *HmcRestClient) CreatePartition(systemUUID, templateUUID, osType string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/do/CreatePartitionFromTemplate", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Creating partition for system %s with template UUID %s and osType %s", systemUUID, templateUUID, osType)
	}

	type Operation struct {
		XMLName       xml.Name `xml:"Operation"`
		OperationName string   `xml:"OperationName"`
		GroupName     string   `xml:"GroupName"`
		ProgressType  string   `xml:"ProgressType"`
	}

	type JobParameter struct {
		XMLName xml.Name `xml:"JobParameter"`
		Name    string   `xml:"name"`
		Value   string   `xml:"value"`
	}

	type JobRequest struct {
		XMLName       xml.Name       `xml:"JobRequest"`
		SchemaVersion string         `xml:"schemaVersion,attr"`
		Operation     Operation      `xml:"RequestedOperation>Operation"`
		Parameters    []JobParameter `xml:"JobParameters>JobParameter"`
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

// FetchJobStatus retrieves the status of a job by its ID
func (c *HmcRestClient) FetchJobStatus(jobID string, template bool, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole/do/GetJobStatus?JobID=%s", c.hmcIP, jobID)
	if verbose {
		hmcLogger.Printf("Fetching job status for JobID: %s, URL: %s", jobID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Job status response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Job status response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var jobResp JobResponse
	if err := xml.Unmarshal(body, &jobResp); err != nil {
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if jobResp.Status == "" {
		return "", fmt.Errorf("no status found for job ID %s", jobID)
	}

	return jobResp.Status, nil
}

// GetMaximumPartitions retrieves the MaximumPartitions for a system by UUID
func (c *HmcRestClient) GetMaximumPartitions(systemUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")
	req.Header.Set("X-API-Session", c.session)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var system System
	if err := xml.Unmarshal(body, &system); err != nil {
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if system.MaxPartitions == "" {
		return "", fmt.Errorf("MaximumPartitions not found for system %s", systemUUID)
	}

	return system.MaxPartitions, nil
}
