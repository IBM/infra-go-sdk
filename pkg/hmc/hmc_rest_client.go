package hmc

import (
	"bytes"
	"context"
	"encoding/json"
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

type VirtualNetworkConfig struct {
	NetworkName       string
	SlotNumber        int
	VirtualSlotNumber int
}

// VolumeConfig defines the configuration for a volume
type VolumeConfig struct {
	ViosName   string // Name of the VIOS managing the volume
	VolumeName string // Name of the volume (e.g., hdisk1)
}

// VIOS represents a Virtual I/O Server
type VIOS struct {
	UUID          string `json:"UUID"`
	PartitionName string `json:"PartitionName"`
	RMCState      string `json:"RMCState"`
}

// PhysicalVolume represents a physical volume
type PhysicalVolume struct {
	VolumeName string `xml:"VolumeName"`
}

// LogicalPartitionQuick represents the structure of a partition in the quick list
type LogicalPartitionQuick struct {
	PartitionName string `json:"PartitionName"`
	UUID          string `json:"UUID"`
}

// xmlStripNamespace removes XML namespaces from the document to simplify XPath queries
func xmlStripNamespace(xmlData []byte) (*etree.Document, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlData); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}
	// Remove namespaces by setting the namespace URI to empty
	for _, elem := range doc.FindElements("//*") {
		elem.Space = ""
	}
	return doc, nil
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

// DeployPartitionTemplate deploys a partition template to a managed system
func (c *HmcRestClient) DeployPartitionTemplate(draftUUID, cecUUID string, verbose bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/templates/PartitionTemplate/%s/do/deploy", c.hmcIP, draftUUID)
	if verbose {
		hmcLogger.Printf("Deploying partition template with UUID %s to system UUID %s, URL: %s", draftUUID, cecUUID, url)
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
	payload, err := createJobRequestPayload(reqdOperation, jobParams, "V1_0", verbose, true)
	if err != nil {
		return "", fmt.Errorf("failed to create job request payload: %v", err)
	}
	if verbose {
		hmcLogger.Printf("Deploy job request payload:\n%s", payload)
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
	if verbose {
		hmcLogger.Printf("Deploy request headers: %+v", req.Header)
	}
	hmcLogger.Printf("DEPLOYTEMPLATE BODY: %s", req.Body)
	hmcLogger.Printf("DEPLOYTEMPLATE PAYLOAD: %s", payload)
	// Set a timeout of 300 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log the response status and body
	if verbose {
		hmcLogger.Printf("DeployPartitionTemplate response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	if verbose {
		hmcLogger.Printf("DeployPartitionTemplate response body:\n%s", string(body))
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
	if verbose {
		hmcLogger.Printf("Extracted JobID: %s", jobID)
	}

	// Fetch and return the job status
	depDoc, err := c.FetchJobStatus(jobID, true, 2, verbose)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %v", err)
	}

	statusElem := depDoc.FindElement("//Status")
	statusE := statusElem.Text()
	if verbose {
		hmcLogger.Printf("Deploy job status: %s", statusE)
	}
	if verbose {
		log.Printf("Job status: %s", statusE)
	}
	//var partUUID string
	if statusE == "COMPLETED_OK" {
		stripNamespace(depDoc.Root())
		jobParams := depDoc.FindElements("//JobParameter")
		for _, param := range jobParams {
			// Look for ParameterName = PartitionUuid
			nameElem := param.FindElement("ParameterName")
			if nameElem != nil && strings.TrimSpace(nameElem.Text()) == "PartitionUuid" {
				partUUID := param.FindElement("ParameterValue")
				if partUUID != nil {
					fmt.Println("PartitionUuid:", partUUID.Text())
					fmt.Printf("Partition creation completed successfully UUID %s\n", partUUID.Text())
					return partUUID.Text(), nil
				}
			}
		}

	}
	if statusE == "FAILED" || statusE == "COMPLETED_WITH_ERROR" {
		log.Fatalf("Partition creation failed with status: %s", statusE)
	}

	return statusE, err
}

// Recursively strip namespace from XML elements
func stripNamespace(elem *etree.Element) {
	elem.Space = ""
	for _, child := range elem.ChildElements() {
		stripNamespace(child)
	}
}

// / GetFreePhyVolume retrieves free physical volumes for a given VIOS UUID
func (c *HmcRestClient) GetFreePhyVolume(viosUUID string, verbose bool) ([]*etree.Element, error) {
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

	// Headers to match Postman
	/* header := map[string]string{
		"X-API-Session": c.session,
		"Content-Type":  "application/vnd.ibm.powervm.web+xml; type=JobRequest",
	}
	*/
	// jobParams := make(map[string]string) // Uncomment to test without FibreChannelBackedOnly

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
	pvDoc, err := c.FetchJobStatus(jobID, false, 2, verbose)
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
	pvDoc, err = xmlStripNamespace([]byte(pvXML))
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from physical volumes XML: %v", err)
	}

	// Find all PhysicalVolume elements
	listPvElem := pvDoc.FindElements("//PhysicalVolume")
	if len(listPvElem) == 0 {
		if verbose {
			hmcLogger.Printf("No free physical volumes found for VIOS UUID %s", viosUUID)
		}
		// Return an empty list instead of an error, as no volumes is a valid case
		return listPvElem, nil
	}
	if verbose {
		hmcLogger.Printf("Found %d free physical volumes for VIOS UUID %s", len(listPvElem), viosUUID)
	}
	return listPvElem, nil
}

// createJobRequestPayload generates the XML payload for a job request
func createJobRequestPayload(operation map[string]string, params map[string]string, schemaVersion string, verbose bool, includeJobParamSchema bool) (string, error) {
	if verbose {
		hmcLogger.Printf("Payload creation: operation=%v, params=%v, schema=%s, includeJobParamSchema=%v", operation, params, schemaVersion, includeJobParamSchema)
	}

	// Create the root element with namespace prefix
	doc := etree.NewDocument()
	root := doc.CreateElement("JobRequest:JobRequest")
	root.CreateAttr("xmlns:JobRequest", "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/")
	root.CreateAttr("xmlns", "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/")
	root.CreateAttr("xmlns:ns2", "http://www.w3.org/XML/1998/namespace/k2")
	root.CreateAttr("schemaVersion", schemaVersion)

	// Add Metadata > Atom
	metadata := root.CreateElement("Metadata")
	metadata.CreateElement("Atom")

	// Add RequestedOperation
	requestedOp := root.CreateElement("RequestedOperation")
	requestedOp.CreateAttr("kb", "CUR")
	requestedOp.CreateAttr("kxe", "false")
	requestedOp.CreateAttr("schemaVersion", schemaVersion)
	requestedOpMetadata := requestedOp.CreateElement("Metadata")
	requestedOpMetadata.CreateElement("Atom")

	// Add OperationName, GroupName, ProgressType
	opName := requestedOp.CreateElement("OperationName")
	opName.CreateAttr("kb", "ROR")
	opName.CreateAttr("kxe", "false")
	opName.SetText(operation["OperationName"])

	groupName := requestedOp.CreateElement("GroupName")
	groupName.CreateAttr("kb", "ROR")
	groupName.CreateAttr("kxe", "false")
	groupName.SetText(operation["GroupName"])

	progressType := requestedOp.CreateElement("ProgressType")
	progressType.CreateAttr("kb", "ROR")
	progressType.CreateAttr("kxe", "false")
	progressType.SetText(operation["ProgressType"])

	// Add JobParameters
	jobParams := root.CreateElement("JobParameters")
	jobParams.CreateAttr("kxe", "false")
	jobParams.CreateAttr("kb", "CUR")
	jobParams.CreateAttr("schemaVersion", schemaVersion)
	jobParamsMetadata := jobParams.CreateElement("Metadata")
	jobParamsMetadata.CreateElement("Atom")

	// Add job parameters if any
	for key, value := range params {
		param := jobParams.CreateElement("JobParameter")
		if includeJobParamSchema {
			param.CreateAttr("schemaVersion", "V1_0")
		}
		paramMetadata := param.CreateElement("Metadata")
		paramMetadata.CreateElement("Atom")
		paramName := param.CreateElement("ParameterName")
		paramName.CreateAttr("kb", "ROR")
		paramName.CreateAttr("kxe", "false")
		paramName.SetText(key)
		paramValue := param.CreateElement("ParameterValue")
		paramValue.CreateAttr("kxe", "false")
		paramValue.CreateAttr("kb", "CUR")
		paramValue.SetText(value)
	}

	// Serialize the XML
	xmlStr, err := doc.WriteToString()
	if err != nil {
		return "", fmt.Errorf("failed to serialize XML: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Generated job request payload:\n%s", xmlStr)
	}
	return xmlStr, nil
}

func AddVSCSIPayload(volConfig VolumeConfig, pv *etree.Element, verbose bool) string {
	volumeNameElem := pv.FindElement("VolumeName")
	if volumeNameElem == nil {
		if verbose {
			hmcLogger.Printf("VolumeName element not found in physical volume XML")
		}
		return ""
	}
	volumeName := volumeNameElem.Text()
	if verbose {
		hmcLogger.Printf("Generating VSCSI payload for volume %s on VIOS %s", volumeName, volConfig.ViosName)
	}
	return fmt.Sprintf(`
        <VirtualSCSIClientAdapter schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            <name kb="CUD" kxe="false"></name>
            <associatedPhysicalVolume kb="CUD" kxe="false" schemaVersion="V1_0">
                <Metadata>
                    <Atom/>
                </Metadata>
                <PhysicalVolume schemaVersion="V1_0">
                    <Metadata>
                        <Atom/>
                    </Metadata>
                    <name kb="CUD" kxe="false">%s</name>
                </PhysicalVolume>
            </associatedPhysicalVolume>
            <connectingPartitionName kxe="false" kb="CUD">%s</connectingPartitionName>
        </VirtualSCSIClientAdapter>`, volumeName, volConfig.ViosName)
}

// AddVSCSI adds the VSCSI client adapters to the partition template XML
func AddVSCSI(templateXML *etree.Element, vscsiClients string) error {
	vscsiClientPayload := fmt.Sprintf(`
        <virtualSCSIClientAdapters kxe="false" kb="CUD" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            %s
        </virtualSCSIClientAdapters>`, vscsiClients)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(vscsiClientPayload); err != nil {
		return fmt.Errorf("failed to parse VSCSI client payload: %v", err)
	}
	vscsiElement := doc.Root()
	if vscsiElement == nil {
		return fmt.Errorf("failed to parse VSCSI client payload: no root element")
	}

	suspendEnableTag := templateXML.FindElement("//suspendEnable")
	if suspendEnableTag == nil {
		return fmt.Errorf("suspendEnable element not found in XML")
	}
	parent := suspendEnableTag.Parent()
	if parent == nil {
		return fmt.Errorf("suspendEnable element has no parent")
	}

	for i, child := range parent.Child {
		if child == suspendEnableTag {
			parent.InsertChildAt(i, vscsiElement)
			break
		}
	}
	return nil
}

// FetchJobResponse retrieves the full job response XML as an etree.Document
func (c *HmcRestClient) FetchJobResponse(jobID string, verbose bool) (*etree.Document, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/jobs/%s", c.hmcIP, jobID)
	if verbose {
		hmcLogger.Printf("Fetching job response for JobID: %s, URL: %s", jobID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Job response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Job response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	return doc, nil
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
func (c *HmcRestClient) UpdateVirtualNWSettingsToDom(templateXML *etree.Element, configDictList []VirtualNetworkConfig) error {
	vnPayload := ""
	for _, eachVN := range configDictList {
		vsnPayload := ""
		if eachVN.VirtualSlotNumber != 0 { // Check for non-zero to mimic Python's 'is not None'
			vsnPayload = fmt.Sprintf(`
                <VirtualSlotNumber kb="CUD" kxe="false">%d</VirtualSlotNumber>`, eachVN.VirtualSlotNumber)
		}
		vnPayload += fmt.Sprintf(`
            <ClientNetworkAdapter schemaVersion="V1_0">
                <Metadata>
                    <Atom/>
                </Metadata>
                %s
                <clientVirtualNetworks kb="CUD" kxe="false" schemaVersion="V1_0">
                    <Metadata>
                        <Atom/>
                    </Metadata>
                    <ClientVirtualNetwork schemaVersion="V1_0">
                        <Metadata>
                            <Atom/>
                        </Metadata>
                        <name kxe="false" kb="CUD">%s</name>
                    </ClientVirtualNetwork>
                </clientVirtualNetworks>
            </ClientNetworkAdapter>`, vsnPayload, eachVN.NetworkName)
	}

	vnwPayload := fmt.Sprintf(`
        <clientNetworkAdapters kb="CUD" kxe="false" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            %s
        </clientNetworkAdapters>`, vnPayload)

	// Parse the XML string into an etree.Document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(vnwPayload); err != nil {
		return fmt.Errorf("failed to parse virtual network XML: %v", err)
	}
	vnwPayloadElement := doc.Root()
	if vnwPayloadElement == nil {
		return fmt.Errorf("failed to parse virtual network XML: no root element")
	}

	// Find the ioConfiguration element
	ioConfigTag := templateXML.FindElement("//ioConfiguration")
	if ioConfigTag == nil {
		return fmt.Errorf("ioConfiguration element not found in XML")
	}

	// Get the parent and insert the new element after ioConfigTag
	parent := ioConfigTag.Parent()
	if parent == nil {
		return fmt.Errorf("ioConfiguration element has no parent")
	}
	for i, child := range parent.Child {
		if child == ioConfigTag {
			parent.InsertChildAt(i+1, vnwPayloadElement)
			break
		}
	}

	return nil
}

// LPAR_TEMPLATE_NS is the namespace for PartitionTemplate as used in the Python code
const LPAR_TEMPLATE_NS = `PartitionTemplate xmlns="http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2"`

// UpdatePartitionTemplate updates an existing partition template with the provided XML
func (c *HmcRestClient) UpdatePartitionTemplate(uuid string, templateXML *etree.Element, verbose bool) error {
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

	if verbose {
		hmcLogger.Printf("Updating partition template XML for UUID %s:\n%s", uuid, xmlStr)
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

	if verbose {
		hmcLogger.Printf("Update partition template response status: %s", resp.Status)
		hmcLogger.Printf("Response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
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

// FetchJobStatus fetches the job status and response, waiting for completion or error
func (c *HmcRestClient) FetchJobStatus(jobID string, template bool, timeoutInMin int, verbose bool) (*etree.Document, error) {
	// Construct URL based on template flag
	var url string
	if template {
		url = fmt.Sprintf("https://%s/rest/api/templates/jobs/%s", c.hmcIP, jobID)
	} else {
		url = fmt.Sprintf("https://%s/rest/api/uom/jobs/%s", c.hmcIP, jobID)
	}

	// Set up headers
	headers := map[string]string{
		"X-API-Session": c.session,
		"Accept":        "application/atom+xml",
	}

	// Set up timeout mechanism
	maxChecks := timeoutInMin * 2 // Check every 30 seconds
	checkInterval := 30 * time.Second
	var jobStatus string // To use in timeout error message
	var doc *etree.Document
	for i := range maxChecks {
		// Sleep between checks, except on the first iteration
		if i > 0 {
			time.Sleep(checkInterval)
		}

		// Create and configure HTTP request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Execute request
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}

		// Parse XML and strip namespaces
		doc, err := xmlStripNamespace(body)
		if err != nil {
			return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
		}

		// Extract job status
		statusElem := doc.FindElement("//Status")
		if statusElem == nil {
			return nil, fmt.Errorf("Status element not found in response")
		}
		jobStatus = statusElem.Text()

		// Log status if verbose
		if verbose {
			hmcLogger.Printf("Job status: %s", jobStatus)
		}

		// Handle different job statuses

		switch jobStatus {
		case "COMPLETED_OK":
			if verbose {
				hmcLogger.Printf("Job completed successfully, response body:\n%s", string(body))
			}
			return doc, nil

		case "COMPLETED_WITH_ERROR":
			if verbose {
				hmcLogger.Printf("Job completed with error")
			}
			resultElem := doc.FindElement("//Results/JobParameter/ParameterValue")
			if resultElem != nil {
				errMsg := strings.TrimSpace(resultElem.Text())
				if verbose {
					hmcLogger.Printf("Error message: %s", errMsg)
				}
				return nil, fmt.Errorf("job completed with error: %s", errMsg)
			}
			return nil, fmt.Errorf("job completed with error, but no result message found")

		default:
			if jobStatus != "RUNNING" {
				if verbose {
					hmcLogger.Printf("Job failed with status: %s", jobStatus)
				}
				errMsgElem := doc.FindElement("//ResponseException//Message")
				if errMsgElem == nil {
					errMsgElem = doc.FindElement("//Results/JobParameter/ParameterValue")
				}
				if errMsgElem != nil {
					errMsg := errMsgElem.Text()
					return nil, fmt.Errorf("job failed: %s", errMsg)
				}
				return nil, fmt.Errorf("job failed with status %s", jobStatus)
			}
			// If status is "RUNNING", continue looping
		}

	}

	// Timeout reached
	operationNameElem := doc.FindElement("//OperationName")
	if operationNameElem != nil {
		operationName := operationNameElem.Text()
		if verbose {
			hmcLogger.Printf("%s job stuck in %s state. Timed out!!", operationName, jobStatus)
		}
		return nil, fmt.Errorf("job %s timed out in state %s", operationName, jobStatus)
	}
	return nil, fmt.Errorf("job timed out")
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

// QuickGetPartition retrieves quick properties of a partition as a map
func (c *HmcRestClient) QuickGetPartition(lparUUID string, verbose bool) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s/quick", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick partition properties for UUID %s, URL: %s", lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("QuickGetPartition response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("QuickGetPartition response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse JSON response
	var partitionProps map[string]interface{}
	if err := json.Unmarshal(body, &partitionProps); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return partitionProps, nil
}

// GetLogicalPartitionsQuick retrieves the quick list of logical partitions for a system
func (c *HmcRestClient) GetLogicalPartitionsQuick(systemUUID string, verbose bool) ([]LogicalPartitionQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/quick/All", c.hmcIP, systemUUID)
	if verbose {
		hmcLogger.Printf("Fetching quick logical partitions for system UUID %s, URL: %s", systemUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionsQuick response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetLogicalPartitionsQuick response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil // No partitions found
		}
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	var lparList []LogicalPartitionQuick
	if err := json.Unmarshal(body, &lparList); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return lparList, nil
}

// GetLogicalPartition retrieves the details of a logical partition by name or UUID
func (c *HmcRestClient) GetLogicalPartition(systemUUID, partitionName, partitionUUID string, verbose bool) (string, *etree.Element, error) {
	var lparUUID string

	// If partitionUUID is not provided, find it using partitionName
	if partitionUUID == "" && partitionName != "" {
		lparList, err := c.GetLogicalPartitionsQuick(systemUUID, verbose)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch logical partitions: %v", err)
		}
		if lparList == nil {
			if verbose {
				hmcLogger.Printf("No logical partitions found for system UUID %s", systemUUID)
			}
			return "", nil, nil
		}

		for _, lpar := range lparList {
			if lpar.PartitionName == partitionName {
				lparUUID = lpar.UUID
				if verbose {
					hmcLogger.Printf("Found partition %s with UUID %s", partitionName, lparUUID)
				}
				break
			}
		}

		if lparUUID == "" {
			if verbose {
				hmcLogger.Printf("Partition %s not found on system UUID %s", partitionName, systemUUID)
			}
			return "", nil, nil
		}
	} else if partitionUUID != "" {
		lparUUID = partitionUUID
	} else {
		return "", nil, fmt.Errorf("either partitionName or partitionUUID must be provided")
	}

	// Fetch partition details
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching logical partition details for UUID %s, URL: %s", lparUUID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("GetLogicalPartition response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if verbose {
		hmcLogger.Printf("GetLogicalPartition response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if verbose {
			hmcLogger.Printf("Get of Logical Partition failed. Response code: %d", resp.StatusCode)
		}
		return "", nil, nil
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	partitionElem := doc.FindElement("//LogicalPartition")
	if partitionElem == nil {
		return "", nil, fmt.Errorf("LogicalPartition element not found in response")
	}

	return lparUUID, partitionElem, nil
}

// GetClientNetworkAdapter retrieves the ClientNetworkAdapter details for a partition
func (c *HmcRestClient) GetClientNetworkAdapter(systemUUID, lparUUID string, verbose bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/LogicalPartition/%s/ClientNetworkAdapter", c.hmcIP, systemUUID, lparUUID)
	if verbose {
		hmcLogger.Printf("Fetching ClientNetworkAdapter for system UUID %s, partition UUID %s, URL: %s", systemUUID, lparUUID, url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml;type=ClientNetworkAdapter")
	req.Header.Set("Accept", "*/*")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Log response status if verbose
	if verbose {
		hmcLogger.Printf("GetClientNetworkAdapter response status: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Log response body if verbose
	if verbose {
		hmcLogger.Printf("GetClientNetworkAdapter response body:\n%s", string(body))
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	clientNetworkAdapter := doc.FindElement("//ClientNetworkAdapter")
	if clientNetworkAdapter == nil {
		return nil, fmt.Errorf("ClientNetworkAdapter element not found in response")
	}

	return clientNetworkAdapter, nil
}
