package hmc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// VolumeConfig defines the configuration for a volume
type VolumeConfig struct {
	ViosName   string // Name of the VIOS managing the volume
	VolumeName string // Name of the volume (e.g., hdisk1)
}

// GetViosID retrieves the UUID of a Virtual I/O Server by its name using the provided rest client
func GetViosID(restClient *HmcRestClient, systemUUID, viosName string, verbose bool) (string, error) {
	viosList, err := restClient.GetVirtualIOServersQuick(systemUUID, verbose)
	fmt.Printf("VIOS List: %s\n", viosList)
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

func AddVSCSIPayload(volConfig VolumeConfig, volumeName string, verbose bool) string {
	if volumeName == "" {
		if verbose {
			hmcLogger.Printf("VolumeName element not found in physical volume XML")
		}
		return ""
	}
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

// GetLogicalPartition retrieves the details of a logical partition by name or UUID
func (c *HmcRestClient) GetLogicalPartition(systemUUID, partitionName, partitionUUID string, verbose bool) (string, *etree.Element, error) {
	var lparUUID string

	// If partitionUUID is not provided, find it using partitionName
	if partitionUUID == "" && partitionName != "" {
		lparList, err := c.GetLogicalPartitionsQuickAll(systemUUID, verbose)
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


func ParseIOAdapters(adapters []*etree.Element) []IOAdapterInfo {
    var info []IOAdapterInfo

    fmt.Printf("Raw adapters (len: %d):\n", len(adapters))
    for i, adapter := range adapters {
        var builder strings.Builder
        adapter.WriteTo(&builder, &etree.WriteSettings{})
        fmt.Printf("Adapter %d XML:\n%s\n", i+1, builder.String())
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
