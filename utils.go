package hmc

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
	"golang.org/x/crypto/ssh"
)

// FormatMACAddress formats a MAC address string by inserting colons every 2 characters
// Example: "DA20D7D4B802" -> "DA:20:D7:D4:B8:02"
func FormatMACAddress(mac string) string {
	if len(mac) != 12 {
		return mac // Return as-is if not standard 12-character format
	}
	var formatted strings.Builder
	for i := 0; i < len(mac); i += 2 {
		if i > 0 {
			formatted.WriteString(":")
		}
		formatted.WriteString(mac[i : i+2])
	}
	return formatted.String()
}


// VolumeConfig defines the configuration for a volume
type VolumeConfig struct {
	ViosName   string // Name of the VIOS managing the volume
	VolumeName string // Name of the volume (e.g., hdisk1)
}

// GetViosID retrieves the UUID of a Virtual I/O Server by its name using the provided rest client
func GetViosID(restClient *HmcRestClient, systemUUID, viosName string, debug bool) (string, error) {
	if debug {
		restClient.Logger.Debug("Retrieving VIOS UUID by name", "viosName", viosName, "systemUUID", systemUUID)
	}

	viosList, err := restClient.GetVirtualIOServersQuick(systemUUID, debug)
	if err != nil {
		return "", fmt.Errorf("failed to get VIOSes: %v", err)
	}

	for _, vios := range viosList {
		if vios.PartitionName == viosName {
			return vios.UUID, nil
		}
	}

	return "", fmt.Errorf("VIOS %s not found", viosName)
}

// createJobRequestPayload generates the XML payload for a job request
func createJobRequestPayload(operation map[string]string, params map[string]string, schemaVersion string, debug bool, includeJobParamSchema bool) (string, error) {
	if debug {
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

	if debug {
		hmcLogger.Printf("Generated job request payload:\n%s", xmlStr)
	}
	return xmlStr, nil
}

func AddVSCSIPayload(volConfig VolumeConfig, volumeName string, debug bool) string {
	if volumeName == "" {
		if debug {
			hmcLogger.Printf("VolumeName element not found in physical volume XML")
		}
		return ""
	}
	if debug {
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
		                      <vlanId kxe="false" kb="CUD">1</vlanId>
		                      <isTagged kxe="false" kb="CUD">false</isTagged>
		                      <associatedSwitchName kxe="false" kb="CUD">%s</associatedSwitchName>
		                  </ClientVirtualNetwork>
		              </clientVirtualNetworks>
		          </ClientNetworkAdapter>`, vsnPayload, eachVN.NetworkName, eachVN.NetworkName)
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

// GetLogicalPartition retrieves the details of a logical partition by name or UUID
func (c *HmcRestClient) GetLogicalPartition(systemUUID, partitionName, partitionUUID string, debug bool) (string, *etree.Element, error) {
	var lparUUID string

	// If partitionUUID is not provided, find it using partitionName
	if partitionUUID == "" && partitionName != "" {
		lparList, err := c.GetLogicalPartitionsQuickAll(systemUUID, debug)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch logical partitions: %v", err)
		}
		if lparList == nil {
			if debug {
				c.Logger.Debug("No logical partitions found for system", "systemUUID", systemUUID)
			}
			return "", nil, nil
		}

		for _, lpar := range lparList {
			if lpar.PartitionName == partitionName {
				lparUUID = lpar.UUID
				if debug {
					c.Logger.Debug("Found partition", "partitionName", partitionName, "lparUUID", lparUUID)
				}
				break
			}
		}

		if lparUUID == "" {
			if debug {
				c.Logger.Warn("Partition not found on system", "partitionName", partitionName, "systemUUID", systemUUID)
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
	if debug {
		c.Logger.Debug("Fetching logical partition details", "lparUUID", lparUUID, "url", url)
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

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetLogicalPartition response status", "status", resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		if debug {
			c.Logger.Error("Get of Logical Partition failed", "statusCode", resp.StatusCode)
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


func ParseIOAdapters(adapters []*etree.Element) []IOAdapterInfo {
	var info []IOAdapterInfo

	hmcLogger.Printf("Raw adapters (len: %d):\n", len(adapters))
	for i, adapter := range adapters {
		var builder strings.Builder
		adapter.WriteTo(&builder, &etree.WriteSettings{})
		hmcLogger.Printf("Adapter %d XML:\n%s\n", i+1, builder.String())
	}

	for _, adapter := range adapters {
		// First find the nested IOAdapter element
		ioAdapterElem := adapter.FindElement("IOAdapter")
		if ioAdapterElem == nil {
			continue
		}

		// Extract child elements under IOAdapter
		desc := textOrEmpty(ioAdapterElem.FindElement("Description"))
		devName := textOrEmpty(ioAdapterElem.FindElement("DeviceName"))

		lpaElem := ioAdapterElem.FindElement("LogicalPartitionAssignmentCapable")
		lpa := false
		if lpaElem != nil {
			lpa = strings.EqualFold(lpaElem.Text(), "true")
		}

		info = append(info, IOAdapterInfo{
			Description:                     desc,
			LogicalPartitionAssignmentCapable: lpa,
			DeviceName:                      devName,
		})
	}

	return info
}

func textOrEmpty(elem *etree.Element) string {
	if elem == nil {
		return ""
	}
	return elem.Text()
}


// fetchAndParseHMCXML is a private helper to reuse standard HTTP and XML stripping logic
func (c *HmcRestClient) fetchAndParseHMCXML(url string, debug bool) (*etree.Document, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	
	c.logRawTraffic("RESPONSE", url, string(body))

	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return xmlStripNamespace(body)
}
// GetAttachedVolumes traces vSCSI mappings on all VIOSes to find backing storage for an LPAR
func (c *HmcRestClient) GetAttachedVolumes(systemUUID, lparUUID string, debug bool) ([]StorageMap, error) {
	var attachedStorage []StorageMap

	if debug {
		c.Logger.Info("Scanning all VIOSes for storage attached to LPAR", "lparUUID", lparUUID)
	}

	// 1. Fetch the list of ALL VIOSes on the Managed System
	viosListURL := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/VirtualIOServer", c.hmcIP, systemUUID)
	viosListDoc, err := c.fetchAndParseHMCXML(viosListURL, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VIOS list: %v", err)
	}

	viosElements := viosListDoc.FindElements(".//*[local-name()='VirtualIOServer']")
	if len(viosElements) == 0 {
		if debug {
			c.Logger.Warn("No Virtual I/O Servers found on system", "systemUUID", systemUUID)
		}
		return attachedStorage, nil
	}

	targetLparLower := strings.ToLower(lparUUID)

	// 2. Loop through every VIOS
	for _, vios := range viosElements {
		viosName := "unknown-vios"
		if nameElem := vios.FindElement(".//*[local-name()='PartitionName']"); nameElem != nil {
			viosName = nameElem.Text()
		}

		viosUUID := "unknown-uuid"
		if uuidElem := vios.FindElement(".//*[local-name()='PartitionUUID']"); uuidElem != nil {
			viosUUID = uuidElem.Text()
		}

		if viosUUID == "unknown-uuid" {
			continue
		}

		// 3. Query the VIOS with ViosSCSIMapping group
		mappingsURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosSCSIMapping", c.hmcIP, viosUUID)
		mappingsDoc, err := c.fetchAndParseHMCXML(mappingsURL, debug)
		if err != nil {
			continue
		}

		// 4. Search the mappings for our target LPAR
		mappings := mappingsDoc.FindElements(".//*[local-name()='VirtualSCSIMapping']")
		
		for _, mapping := range mappings {
			
			assocLpar := mapping.FindElement(".//*[local-name()='AssociatedLogicalPartition']")
			if assocLpar == nil {
				continue
			}

			href := strings.ToLower(assocLpar.SelectAttrValue("href", ""))
			
			// Does this mapping belong to our target LPAR?
			if strings.HasSuffix(href, targetLparLower) {
				
				backingDevice := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='BackingDeviceName']")
				storageVolName := mapping.FindElement(".//*[local-name()='Storage']/*[local-name()='PhysicalVolume']/*[local-name()='VolumeName']")
				storageUDID := mapping.FindElement(".//*[local-name()='Storage']/*[local-name()='PhysicalVolume']/*[local-name()='VolumeUniqueID']")

				// Extract adapter information
				serverAdapterElem := mapping.FindElement(".//*[local-name()='ServerAdapter']/*[local-name()='AdapterName']")
				serverAdapter := ""
				if serverAdapterElem != nil {
					serverAdapter = serverAdapterElem.Text()
				}

				// Extract client adapter name from TargetDevice/PhysicalVolumeVirtualTargetDevice/TargetName (e.g., vtscsi0)
				clientAdapterElem := mapping.FindElement("TargetDevice/PhysicalVolumeVirtualTargetDevice/TargetName")
				clientAdapter := ""
				if clientAdapterElem != nil {
					clientAdapter = clientAdapterElem.Text()
				}

				clientSlotElem := mapping.FindElement(".//*[local-name()='ClientAdapter']/*[local-name()='VirtualSlotNumber']")
				clientSlot := "unknown"
				if clientSlotElem != nil {
					clientSlot = clientSlotElem.Text()
				}

				vName := ""
				vUDID := "unknown"

				if backingDevice != nil && backingDevice.Text() != "" {
					vName = backingDevice.Text()
				} else if storageVolName != nil && storageVolName.Text() != "" {
					vName = storageVolName.Text()
				}

				// FIX: Instead of skipping empty adapters, tag them so they get deleted!
				if vName == "" {
					vName = "EMPTY_VSCSI_SLOT_" + clientSlot
					if debug {
						c.Logger.Debug("Found empty virtual adapter with no disk on VIOS. Tagging for cleanup.", "clientSlot", clientSlot, "viosName", viosName)
					}
				} else {
					if storageUDID != nil && storageUDID.Text() != "" {
						vUDID = storageUDID.Text()
					} else {
						// Fallback to fetch UDID from the Storage group
						pvURL := fmt.Sprintf("https://%s/rest/api/uom/VirtualIOServer/%s?group=ViosStorage", c.hmcIP, viosUUID)
						viosStorageDoc, _ := c.fetchAndParseHMCXML(pvURL, false)
						if viosStorageDoc != nil {
							pvs := viosStorageDoc.FindElements(".//*[local-name()='PhysicalVolume']")
							for _, pv := range pvs {
								pvName := pv.FindElement(".//*[local-name()='VolumeName']")
								if pvName != nil && pvName.Text() == vName {
									pvUDID := pv.FindElement(".//*[local-name()='VolumeUniqueID']")
									if pvUDID != nil {
										vUDID = pvUDID.Text()
									}
									break
								}
							}
						}
					}
				}

				attachedStorage = append(attachedStorage, StorageMap{
					ViosUUID:         viosUUID,
					ViosName:         viosName,
					VolumeName:       vName,
					VolumeUDID:       vUDID,
					ServerAdapter:    serverAdapter,
					ClientAdapter:    clientAdapter,
					ClientSlotNumber: clientSlot,
				})
			}
		}
	}

	return attachedStorage, nil
}

func (c *HmcRestClient) GetSvcUidFixed(viosId string) string {
	// Logic for 33213: Header is 5 chars, 32-char UID follows
	if len(viosId) >= 37 && viosId[0:5] == "33213" {
		return strings.ToUpper(viosId[5 : 5+32])
	}
	return ""
}

// DeleteHMCResource executes a DELETE request against a specific HMC REST API URL.
func (c *HmcRestClient) DeleteHMCResource(resourceURL string, debug bool) error {
	if debug { 
		c.Logger.Debug("Executing DELETE on Resource", "url", resourceURL) 
	}
	
	req, err := http.NewRequest("DELETE", resourceURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Session", c.session)
	
	c.logRawTraffic("REQUEST (DELETE)", resourceURL, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	c.logRawTraffic("RESPONSE", resourceURL, string(body))

	if debug {
		c.Logger.Debug("Resource DELETE Status", "status", resp.Status)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete resource. Status: %s, Response: %s", resp.Status, string(body))
	}
	return nil
}

// GetLogicalPartitionByName resolves a logical partition name to its full Quick details and UUID on a specific Managed System.
// It uses the quick (JSON) endpoint for high performance.
func (c *HmcRestClient) GetLogicalPartitionByName(systemUUID, partitionName string, debug bool) (*LogicalPartitionQuick, string, error) {
	if debug {
		c.Logger.Debug("Resolving LPAR name to UUID on system", "partitionName", partitionName, "systemUUID", systemUUID)
	}

	// Fetch all partitions using the lightweight JSON endpoint 
	lpars, err := c.GetLogicalPartitionsQuickAll(systemUUID, debug)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve logical partitions: %v", err)
	}

	// Iterate through the slice to find the matching name 
	for _, lpar := range lpars {
		if lpar.PartitionName == partitionName {
			if debug {
				c.Logger.Info("Found LPAR", "partitionName", partitionName, "lparUUID", lpar.UUID)
			}
			// Use '&lpar' to return a pointer to the struct, rather than '*lpar'
			// Create a local copy to ensure safe pointer escape
			matchedLpar := lpar 
			return &matchedLpar, matchedLpar.UUID, nil
		}
	}

	return nil, "", fmt.Errorf("logical partition '%s' not found on system %s", partitionName, systemUUID)
}
// GetManagedSystemByNameQuick resolves a system name to its full Quick details and UUID 
// by scanning the high-performance JSON inventory.
func (c *HmcRestClient) GetManagedSystemByNameQuick(systemName string, debug bool) (*ManagedSystemQuick, string, error) {
	if debug {
		c.Logger.Debug("Resolving Managed System name via Quick inventory", "systemName", systemName)
	}

	// 1. Fetch the high-performance JSON list of all systems
	systems, err := c.GetManagedSystemQuickAll(debug)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve systems inventory: %v", err)
	}

	// 2. Iterate to find a case-insensitive name match
	for _, s := range systems {
		if strings.EqualFold(s.SystemName, systemName) {
			if debug {
				c.Logger.Info("System resolved", "systemName", s.SystemName, "systemUUID", s.UUID)
			}
			return &s, s.UUID, nil
		}
	}

	return nil, "", fmt.Errorf("managed system '%s' not found in HMC inventory", systemName)
}
// parseLogicalPartitionElements converts raw XML elements into a typed slice of LogicalPartitionDetailed
func parseLogicalPartitionElements(elements []*etree.Element, debug bool) ([]LogicalPartitionDetailed, error) {
	var detailedPartitions []LogicalPartitionDetailed
	for _, lparElem := range elements {
		lparDoc := etree.NewDocument()
		lparDoc.SetRoot(lparElem.Copy())
		
		lparBytes, err := lparDoc.WriteToBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize isolated LogicalPartition element: %v", err)
		}

		var detailedLpar LogicalPartitionDetailed
		if err := xml.Unmarshal(lparBytes, &detailedLpar); err != nil {
			return nil, fmt.Errorf("failed to unmarshal XML into LogicalPartitionDetailed struct: %v", err)
		}
		detailedPartitions = append(detailedPartitions, detailedLpar)
	}
	return detailedPartitions, nil
}


// GetLocationCodeByMac calls GetClientNetworkAdapters and finds the matching LocationCode for a MAC address.
func (c *HmcRestClient) GetLocationCodeByMac(sysUUID, lparUUID, targetMac string, debug bool) (string, error) {
	if debug {
		c.Logger.Debug("Translating MAC using ClientNetworkAdapters endpoint", "targetMac", targetMac)
	}

	// 1. Fetch all adapters using your existing, robust function!
	adapters, err := c.GetClientNetworkAdapters(sysUUID, lparUUID, debug)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve adapters for MAC translation: %v", err)
	}

	// Clean the target MAC (strip colons, uppercase)
	cleanTargetMac := strings.ToUpper(strings.ReplaceAll(targetMac, ":", ""))
	var availableMacs []string

	// 2. Iterate through the cleanly parsed Go structs
	for _, adapter := range adapters {
		// Clean the adapter's MAC for a safe comparison
		cleanAdapterMac := strings.ToUpper(strings.ReplaceAll(adapter.MACAddress, ":", ""))
		
		// Format the found MAC nicely for error logging
		displayMac := cleanAdapterMac
		if len(cleanAdapterMac) == 12 {
			displayMac = fmt.Sprintf("%s:%s:%s:%s:%s:%s", 
				cleanAdapterMac[0:2], cleanAdapterMac[2:4], cleanAdapterMac[4:6], 
				cleanAdapterMac[6:8], cleanAdapterMac[8:10], cleanAdapterMac[10:12])
		}
		availableMacs = append(availableMacs, displayMac)

		// 3. Match found!
		if cleanAdapterMac == cleanTargetMac {
			if adapter.LocationCode != "" {
				return adapter.LocationCode, nil
			}
			
			// Fallback warning if the HMC returned the adapter but hid the LocationCode
			if debug {
				c.Logger.Warn("Matched MAC, but LocationCode is empty", "mac", displayMac, "virtualSlotNumber", adapter.VirtualSlotNumber)
			}
			return "", fmt.Errorf("MAC %s found, but the HMC did not provide a LocationCode for it", displayMac)
		}
	}

	return "", fmt.Errorf("MAC %s not found. Available MACs on this LPAR: %v", targetMac, availableMacs)
}

// MountNFS mounts an NFS export on a VIOS using the mount command via CliRunner.
// Parameters:
//   - restClient: The HMC REST client instance
//   - sysName: The managed system name
//   - viosName: The VIOS partition name
//   - nfsServer: The NFS server hostname or IP address
//   - exportPath: The NFS export path on the server (e.g., /export/data)
//   - mountPoint: The local mount point on VIOS (e.g., /mnt/nfs)
//   - options: NFS version to use (e.g., "3" or "4"). Use empty string for default
//   - verbose: Enable verbose logging
//
// Returns the command output and any error encountered.
//
// Note: Uses AIX mount command. The mount point directory must exist before mounting.
// Syntax: mount [-nfsvers version] Node:Directory Directory
//
// Example:
//   output, err := MountNFS(client, "sys1", "vios1", "192.168.1.100", "/export/iso", "/mnt/iso", "3", true)
func MountNFS(restClient *HmcRestClient, sysName, viosName, nfsServer, exportPath, mountPoint, options string, debug bool) (string, error) {
	if sysName == "" || viosName == "" || nfsServer == "" || exportPath == "" || mountPoint == "" {
		return "", fmt.Errorf("sysName, viosName, nfsServer, exportPath, and mountPoint are required")
	}

	if debug {
		restClient.Logger.Info("Mounting NFS", "nfsServer", nfsServer, "exportPath", exportPath, "mountPoint", mountPoint, "viosName", viosName, "sysName", sysName)
	}

	// Build the mount command using AIX syntax
	// Syntax: mount [-nfsvers version] Node:Directory Directory
	var cmd string
	if options != "" {
		cmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mount %s:%s %s"`,
			sysName, viosName, nfsServer, exportPath, mountPoint)
	} else {
		cmd = fmt.Sprintf(`viosvrcmd -m %s -p %s -c "mount %s:%s %s"`,
			sysName, viosName, nfsServer, exportPath, mountPoint)
	}

	if debug {
		restClient.Logger.Debug("Executing NFS mount command", "cmd", cmd)
	}

	output, err := restClient.CliRunner(cmd, debug)
	if err != nil {
		return output, fmt.Errorf("failed to mount NFS: %v\nOutput: %s", err, output)
	}

	if debug {
		restClient.Logger.Info("NFS mounted successfully", "output", strings.TrimSpace(output))
	}

	return output, nil
}

// UnmountNFS unmounts an NFS mount point on a VIOS using the unmount command via CliRunner.
// Parameters:
//   - restClient: The HMC REST client instance
//   - sysName: The managed system name
//   - viosName: The VIOS partition name
//   - mountPoint: The local mount point to unmount (e.g., /mnt/nfs)
//   - verbose: Enable verbose logging
//
// Returns the command output and any error encountered.
//
// Example:
//   output, err := UnmountNFS(client, "sys1", "vios1", "/mnt/iso", true)
func UnmountNFS(restClient *HmcRestClient, sysName, viosName, mountPoint string, debug bool) (string, error) {
	if sysName == "" || viosName == "" || mountPoint == "" {
		return "", fmt.Errorf("sysName, viosName, and mountPoint are required")
	}

	if debug {
		restClient.Logger.Info("Unmounting NFS", "mountPoint", mountPoint, "viosName", viosName, "sysName", sysName)
	}

	// Build the umount command
	// Syntax: umount Directory
	cmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "unmount %s"`,
		sysName, viosName, mountPoint)

	if debug {
		restClient.Logger.Debug("Executing NFS unmount command", "cmd", cmd)
	}

	output, err := restClient.CliRunner(cmd, debug)
	if err != nil {
		return output, fmt.Errorf("failed to unmount NFS: %v\nOutput: %s", err, output)
	}

	if debug {
		restClient.Logger.Info("NFS unmounted successfully", "output", strings.TrimSpace(output))
	}

	return output, nil
}

// CloseVirtualTerminal forcefully closes an open console session on an LPAR using the HMC CLIRunner.
func (c *HmcRestClient) CloseVirtualTerminal(sysName, lparName string, debug bool) error {
	if debug {
		c.Logger.Debug("Forcing closure of virtual terminal", "lparName", lparName, "sysName", sysName)
	}

	// The native HMC CLI command to kill a vterm session
	cliCmd := fmt.Sprintf("rmvterm -m %s -p %s", sysName, lparName)

	// Execute it using the CLIRunner function
	output, err := c.CliRunner(cliCmd, debug)
	if err != nil {
		return fmt.Errorf("failed to close terminal: %v (Output: %s)", err, output)
	}

	if debug {
		c.Logger.Info("Virtual terminal closed successfully", "output", output)
	}
	
	return nil
}

// CliRunnerViaSsh executes HMC CLI commands directly via SSH instead of using the REST API.
// This bypasses REST API limitations and allows running any HMC CLI command.
//
// Parameters:
//   - hmcIP: HMC IP address or hostname
//   - username: HMC username (typically "REDACTED_HMC_USER<==")
//   - password: HMC password
//   - command: The CLI command to execute (e.g., "rmvterm -m system -p lpar")
//   - verbose: Enable verbose logging
//
// Returns:
//   - output: Command output as string
//   - error: Error if command fails
//
// Example:
//   output, err := CliRunnerViaSsh("192.0.2.2", "REDACTED_HMC_USER<==", "password", "lshmc -V", true)
func CliRunnerViaSsh(hmcIP, username, password, command string, debug bool) (string, error) {
	if debug {
		hmcLogger.Printf("Executing HMC CLI command via SSH: %s", command)
	}

	// Configure SSH client
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, verify host key
		Timeout:         30 * time.Second,
	}

	// Connect to HMC via SSH
	addr := fmt.Sprintf("%s:22", hmcIP)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to HMC via SSH: %w", err)
	}
	defer client.Close()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Execute command
	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w (output: %s)", err, string(output))
	}

	if debug {
		hmcLogger.Printf("Command output: %s", string(output))
	}

	return string(output), nil
}

// CloseVirtualTerminalViaSsh closes a virtual terminal using direct SSH connection to HMC.
// This is an alternative to CloseVirtualTerminal that bypasses REST API limitations.
func (c *HmcRestClient) CloseVirtualTerminalViaSsh(hmcIP, username, password, sysName, lparName string, debug bool) error {
	if debug {
		c.Logger.Debug("Closing virtual terminal via SSH", "lparName", lparName, "sysName", sysName)
	}

	// Build rmvterm command
	command := fmt.Sprintf("rmvterm -m %s -p %s", sysName, lparName)

	// Execute via SSH
	output, err := CliRunnerViaSsh(hmcIP, username, password, command, debug)
	if err != nil {
		return fmt.Errorf("failed to close terminal via SSH: %w", err)
	}

	if debug {
		c.Logger.Info("Virtual terminal closed successfully via SSH", "output", output)
	}

	return nil
}

// GetActiveVIOSServers filters and returns only the active VIOS servers from a list of VIOS UUIDs.
// It fetches detailed information for each VIOS and checks if ResourceMonitoringControlState is "active".
// Returns a map where:
//   - KEY: VIOS UUID
//   - VALUE: VirtualIOServerDetailed (complete VIOS details)
// 
// Note: In PowerVM environments, multiple VIOS servers can be active simultaneously for redundancy.
// This function returns ALL active VIOS servers, allowing the caller to choose which one to use.
func (c *HmcRestClient) GetActiveVIOSServers(systemUUID string, viosUUIDs []string, debug bool) (map[string]*VirtualIOServerDetailed, error) {
	activeVIOSServers := make(map[string]*VirtualIOServerDetailed)
	
	if debug {
		c.Logger.Debug("Checking VIOS servers for active state", "count", len(viosUUIDs))
	}
	
	for _, viosUUID := range viosUUIDs {
		// Get detailed VIOS information
		viosDetails, err := c.GetVirtualIOServer(viosUUID, debug)
		if err != nil {
			if debug {
				c.Logger.Warn("Failed to get details for VIOS", "viosUUID", viosUUID, "error", err)
			}
			continue
		}
		
		// Check if VIOS is in active state
		if viosDetails.ResourceMonitoringControlState == "active" {
			activeVIOSServers[viosUUID] = viosDetails
			if debug {
				c.Logger.Info("VIOS is active", "viosName", viosDetails.PartitionName, "viosUUID", viosUUID)
			}
		} else {
			if debug {
				c.Logger.Debug("VIOS is not active", "viosName", viosDetails.PartitionName, "viosUUID", viosUUID, "state", viosDetails.ResourceMonitoringControlState)
			}
		}
	}
	
	if len(activeVIOSServers) == 0 {
		return nil, fmt.Errorf("no active VIOS servers found among %d VIOS(s)", len(viosUUIDs))
	}
	
	if debug {
		c.Logger.Info("Active VIOS servers found", "activeCount", len(activeVIOSServers), "totalCount", len(viosUUIDs))
	}
	
	return activeVIOSServers, nil
}