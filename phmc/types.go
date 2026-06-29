package hmc

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/beevik/etree"
)

// LparTemplateNS is the namespace for PartitionTemplate as used in the Python code
const LparTemplateNS = `PartitionTemplate xmlns="http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2"`

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
	XMLName            xml.Name            `xml:"JobResponse"`
	RequestURL         JobResponseURL      `xml:"RequestURL"`
	TargetUUID         string              `xml:"TargetUuid"`
	JobID              string              `xml:"JobID"`
	TimeStarted        string              `xml:"TimeStarted"`
	TimeCompleted      string              `xml:"TimeCompleted"`
	Status             string              `xml:"Status"`
	JobRequestInstance JobResponseRequest  `xml:"JobRequestInstance"`
	Progress           JobResponseProgress `xml:"Progress"`
	Results            JobResponseResults  `xml:"Results"`
}

// JobResponseURL represents the URL to which the JobRequest was submitted
type JobResponseURL struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr"`
	Title string `xml:"title,attr"`
}

// JobResponseRequest represents the job request instance details in a response
type JobResponseRequest struct {
	RequestedOperation JobResponseOperation  `xml:"RequestedOperation"`
	JobParameters      JobResponseParameters `xml:"JobParameters"`
}

// JobResponseOperation represents the operation being performed
type JobResponseOperation struct {
	OperationName string `xml:"OperationName"`
	GroupName     string `xml:"GroupName"`
}

// JobResponseParameters represents the collection of job parameters in a response
type JobResponseParameters struct {
	Parameters []JobResponseParameter `xml:"JobParameter"`
}

// JobResponseParameter represents a single job parameter in a response
type JobResponseParameter struct {
	ParameterName  string `xml:"ParameterName"`
	ParameterValue string `xml:"ParameterValue"`
}

// JobResponseProgress represents the progress information for a job
type JobResponseProgress struct {
	// Progress fields can be added here as needed
}

// JobResponseResults represents the results of a completed job
type JobResponseResults struct {
	Parameters []JobResponseParameter `xml:"JobParameter"`
}

// RestClient represents the REST client for HMC operations
type RestClient struct {
	hmcIP   string
	session string
	client  *http.Client
}

// NewRestClient initializes a new RestClient.
// By default the client uses the standard TLS verification. Call
// WithTLSInsecure() to skip certificate verification (e.g. for lab HMCs).
//
// Deprecated insecure-by-default behaviour has been removed; callers that
// previously relied on it should chain .WithTLSInsecure():
//
//	restClient := hmc.NewRestClient(hmcIP).WithTLSInsecure()
func NewRestClient(hmcIP string) *RestClient {
	return &RestClient{
		hmcIP: hmcIP,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// WithTLSInsecure disables TLS certificate verification for all requests.
// It clones the current transport (or http.DefaultTransport) to avoid
// mutating the shared default, then sets InsecureSkipVerify.
//
// Call this before WithTransport if you need both options so that the inner
// transport already has the correct TLS config when you wrap it.
func (c *RestClient) WithTLSInsecure() *RestClient {
	base := c.client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	if t, ok := base.(*http.Transport); ok {
		cloned := t.Clone()
		cloned.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		c.client.Transport = cloned
	}
	return c
}

// WithTransport replaces the HTTP transport used for all HMC requests.
// Use this to inject middleware such as logging, metrics, retries, or
// custom proxy/mTLS transports.
//
// Call WithTLSInsecure *before* WithTransport if you need both — the wrapped
// transport will already have InsecureSkipVerify set on the inner layer.
//
//	restClient := hmc.NewRestClient(hmcIP).
//	    WithTLSInsecure().
//	    WithTransport(myLoggingTransport(restClient.HTTPTransport()))
func (c *RestClient) WithTransport(rt http.RoundTripper) *RestClient {
	c.client.Transport = rt
	return c
}

// HTTPTransport returns the http.RoundTripper currently configured on the
// client. This is useful when composing a wrapper transport:
//
//	base := restClient.HTTPTransport()  // grab the (possibly TLS-patched) transport
//	restClient.WithTransport(myWrapper(base))
func (c *RestClient) HTTPTransport() http.RoundTripper {
	if c.client.Transport == nil {
		return http.DefaultTransport
	}
	return c.client.Transport
}

// Session returns the current session token
func (c *RestClient) Session() string {
	return c.session
}

// VirtualNetworkConfig represents the virtual network configuration for a partition
type VirtualNetworkConfig struct {
	NetworkName       string
	SlotNumber        int
	VirtualSlotNumber int
}

// VolumeMappingInfo represents information about a volume mapping that needs to be removed
type VolumeMappingInfo struct {
	VolumeName             string
	VTDName                string
	ServerAdapterDeleteURL string
	ClientSlotNumber       string
	ServerSlotNumber       string
}

// VIOS represents a Virtual I/O Server
type VIOS struct {
	UUID          string `json:"UUID"`
	PartitionName string `json:"PartitionName"`
	RMCState      string `json:"RMCState"`
}

// PhysicalVolume represents a physical volume
type PhysicalVolume struct {
	XMLName                   xml.Name `xml:"PhysicalVolume"`
	Description               string   `xml:"Description"`
	LocationCode              string   `xml:"LocationCode"`
	PersistentReserveKeyValue string   `xml:"PersistentReserveKeyValue"`
	ReservePolicy             string   `xml:"ReservePolicy"`
	ReservePolicyAlgorithm    string   `xml:"ReservePolicyAlgorithm"`
	UniqueDeviceID            string   `xml:"UniqueDeviceID"`
	AvailableForUsage         bool     `xml:"AvailableForUsage"`
	VolumeCapacity            int64    `xml:"VolumeCapacity"`
	VolumeName                string   `xml:"VolumeName"`
	VolumeState               string   `xml:"VolumeState"`
	VolumeUniqueID            string   `xml:"VolumeUniqueID"`
	IsFibreChannelBacked      bool     `xml:"IsFibreChannelBacked"`
	IsISCSIBacked             bool     `xml:"IsISCSIBacked"`
	StorageLabel              string   `xml:"StorageLabel"`
	DescriptorPage83          string   `xml:"DescriptorPage83"`
}

// LogicalPartitionQuick represents the structure of a partition in the quick list
// LogicalPartitionQuick represents the structure of a partition in the quick list
type LogicalPartitionQuick struct {
	ProgressState                  *string `json:"ProgressState"`
	Description                    *string `json:"Description"`
	MemoryMode                     string  `json:"MemoryMode"`
	MigrationState                 string  `json:"MigrationState"`
	PowerManagementMode            *string `json:"PowerManagementMode"`
	OperatingSystemVersion         string  `json:"OperatingSystemVersion"`
	PartitionID                    int     `json:"PartitionID"` // IDs are safe as int
	IsVirtualServiceAttentionLEDOn string  `json:"IsVirtualServiceAttentionLEDOn"`

	// CHANGED TO float64 to handle HMC scientific notation (e.g., 1.024E+4)
	AllocatedVirtualProcessors float64 `json:"AllocatedVirtualProcessors"`

	PartitionState              string  `json:"PartitionState"`
	ResourceMonitoringIPAddress *string `json:"ResourceMonitoringIPAddress"`
	HasPhysicalIO               string  `json:"HasPhysicalIO"`
	SystemName                  string  `json:"SystemName"`
	SharingMode                 string  `json:"SharingMode"`
	MigrationDisable            bool    `json:"MigrationDisable"`

	// CHANGED TO float64
	CurrentProcessors float64 `json:"CurrentProcessors"`

	LastActivatedProfile  string `json:"LastActivatedProfile"`
	CurrentUncappedWeight int    `json:"CurrentUncappedWeight"`
	RemoteRestartState    string `json:"RemoteRestartState"`
	PartitionType         string `json:"PartitionType"`
	PartitionName         string `json:"PartitionName"`
	RMCState              string `json:"RMCState"`
	OperatingSystemType   string `json:"OperatingSystemType"`

	// CHANGED TO float64
	CurrentMemory float64 `json:"CurrentMemory"`

	HasDedicatedProcessors  string  `json:"HasDedicatedProcessors"`
	AssociatedManagedSystem string  `json:"AssociatedManagedSystem"`
	ReferenceCode           string  `json:"ReferenceCode"`
	CurrentProcessingUnits  float64 `json:"CurrentProcessingUnits"`
	UUID                    string  // Manually set, not from JSON
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

// Recursively strip namespace from XML elements
func stripNamespace(elem *etree.Element) {
	elem.Space = ""
	for _, child := range elem.ChildElements() {
		stripNamespace(child)
	}
}

// PartitionProfileQuick represents the structure of a partition profile in the quick list
type PartitionProfileQuick struct {
	ProfileName string `json:"ProfileName"`
	UUID        string `json:"UUID"`
}

// ManagedSystemQuick represents the complete flattened JSON structure
// from the HMC /rest/api/uom/ManagedSystem/quick/All endpoint.
type ManagedSystemQuick struct {
	SystemName     string  `json:"SystemName"`
	UUID           string  `json:"UUID"`
	State          string  `json:"State"`
	StateDetail    string  `json:"StateDetail"`
	IPAddress      string  `json:"IPAddress"`
	MTMS           string  `json:"MTMS"`
	SystemType     string  `json:"SystemType"`
	SystemFirmware string  `json:"SystemFirmware"`
	SystemLocation *string `json:"SystemLocation"` // Using pointer for null values
	Description    *string `json:"Description"`

	// Memory Metrics (MB)
	ConfigurableSystemMemory     float64 `json:"ConfigurableSystemMemory"`
	CurrentAvailableSystemMemory float64 `json:"CurrentAvailableSystemMemory"`
	InstalledSystemMemory        float64 `json:"InstalledSystemMemory"`
	PermanentSystemMemory        float64 `json:"PermanentSystemMemory"`
	MemoryDefragmentationState   string  `json:"MemoryDefragmentationState"`

	// Processor Metrics
	ConfigurableSystemProcessorUnits     float64 `json:"ConfigurableSystemProcessorUnits"`
	CurrentAvailableSystemProcessorUnits float64 `json:"CurrentAvailableSystemProcessorUnits"`
	InstalledSystemProcessorUnits        float64 `json:"InstalledSystemProcessorUnits"`
	PermanentSystemProcessors            float64 `json:"PermanentSystemProcessors"`
	ProcessorThrottling                  string  `json:"ProcessorThrottling"` // String "true"/"false"

	// Versioning & Levels
	ActivatedLevel                   string  `json:"ActivatedLevel"`
	ActivatedServicePackNameAndLevel string  `json:"ActivatedServicePackNameAndLevel"`
	DeferredLevel                    *string `json:"DeferredLevel"`
	DeferredServicePackNameAndLevel  *string `json:"DeferredServicePackNameAndLevel"`
	ServiceProcessorVersion          string  `json:"ServiceProcessorVersion"`
	BMCVersion                       *string `json:"BMCVersion"`
	PNORVersion                      *string `json:"PNORVersion"`

	// Capabilities & Flags (All reported as strings "true"/"false")
	CapacityOnDemandProcessorCapable         string `json:"CapacityOnDemandProcessorCapable"`
	CapacityOnDemandMemoryCapable            string `json:"CapacityOnDemandMemoryCapable"`
	ManufacturingDefaultConfigurationEnabled string `json:"ManufacturingDefaultConfigurationEnabled"`
	PhysicalSystemAttentionLEDState          string `json:"PhysicalSystemAttentionLEDState"`
	IsClassicHMCManagement                   string `json:"IsClassicHMCManagement"`
	IsPowerVMManagementController            string `json:"IsPowerVMManagementController"`
	IsNotPowerVMManagementController         string `json:"IsNotPowerVMManagementController"`
	IsPowerVMManagementMaster                string `json:"IsPowerVMManagementMaster"`
	IsNotPowerVMManagementMaster             string `json:"IsNotPowerVMManagementMaster"`

	// Miscellaneous
	MaximumPartitions   int     `json:"MaximumPartitions"`
	ReferenceCode       string  `json:"ReferenceCode"`
	MergedReferenceCode string  `json:"MergedReferenceCode"`
	MeteredPoolID       *string `json:"MeteredPoolID"`
}
// Operation represents a job operation request
type Operation struct {
	XMLName       xml.Name `xml:"Operation"`
	OperationName string   `xml:"OperationName"`
	GroupName     string   `xml:"GroupName"`
	ProgressType  string   `xml:"ProgressType"`
}

// JobParameter represents a parameter for a job request
type JobParameter struct {
	XMLName xml.Name `xml:"JobParameter"`
	Name    string   `xml:"name"`
	Value   string   `xml:"value"`
}

// JobRequest represents a job request to the HMC
type JobRequest struct {
	XMLName       xml.Name       `xml:"JobRequest"`
	SchemaVersion string         `xml:"schemaVersion,attr"`
	Operation     Operation      `xml:"RequestedOperation>Operation"`
	Parameters    []JobParameter `xml:"JobParameters>JobParameter"`
}

// PhysicalVolumeCollection represents a collection of physical volumes for unmarshaling
type PhysicalVolumeCollection struct {
	XMLName         xml.Name         `xml:"PhysicalVolume_Collection"`
	PhysicalVolumes []PhysicalVolume `xml:"PhysicalVolume"`
}

// IOAdapterInfo represents information details
type IOAdapterInfo struct {
	Description                       string
	LogicalPartitionAssignmentCapable bool
	DeviceName                        string
}

// --- Append Below to logicalpartition.go ---

// StorageMap holds the dynamically discovered VIOS and Volume details for an LPAR
type StorageMap struct {
	ViosUUID         string
	ViosName         string
	VolumeName       string
	VolumeUDID       string // Very useful for matching against SVC VDisk UID
	ServerAdapter    string // Virtual SCSI adapter on VIOS side (e.g., vhost0)
	ClientAdapter    string // Virtual SCSI adapter on client/LPAR side (e.g., vtscsi0)
	ClientSlotNumber string // Client adapter slot number
}

// VIOSQuick represents the exhaustive flattened JSON structure from the HMC /quick/All endpoint.
type VIOSQuick struct {
	Description                    *string  `json:"Description"`
	OperatingSystemVersion         string   `json:"OperatingSystemVersion"`
	PartitionID                    int      `json:"PartitionID"`
	APICapable                     string   `json:"APICapable"`
	IsVirtualServiceAttentionLEDOn string   `json:"IsVirtualServiceAttentionLEDOn"`
	AllocatedVirtualProcessors     *float64 `json:"AllocatedVirtualProcessors"` // Pointer for null safety
	PartitionState                 string   `json:"PartitionState"`
	ResourceMonitoringIPAddress    *string  `json:"ResourceMonitoringIPAddress"` // Pointer for null safety
	HasPhysicalIO                  string   `json:"HasPhysicalIO"`
	SystemName                     string   `json:"SystemName"`
	SharingMode                    string   `json:"SharingMode"`
	UUID                           string   `json:"UUID"`
	CurrentProcessors              float64  `json:"CurrentProcessors"`
	LastActivatedProfile           string   `json:"LastActivatedProfile"`
	CurrentUncappedWeight          *int     `json:"CurrentUncappedWeight"` // Pointer for null safety
	PartitionType                  string   `json:"PartitionType"`
	VirtualIOServerLicenseAccepted *string  `json:"VirtualIOServerLicenseAccepted"` // Pointer for null safety
	PartitionName                  string   `json:"PartitionName"`
	RMCState                       string   `json:"RMCState"`
	OperatingSystemType            string   `json:"OperatingSystemType"`
	CurrentMemory                  float64  `json:"CurrentMemory"`
	HasDedicatedProcessors         string   `json:"HasDedicatedProcessors"`
	AssociatedManagedSystem        string   `json:"AssociatedManagedSystem"`
	ReferenceCode                  string   `json:"ReferenceCode"`
	CurrentProcessingUnits         *float64 `json:"CurrentProcessingUnits"` // Pointer for null safety
}

// =====================================================================
// EXHAUSTIVE VIRTUAL I/O SERVER (VIOS) XML DATA STRUCTURES
// =====================================================================

// VirtualIOServerDetailed represents the complete XML payload of a Virtual I/O Server.
type VirtualIOServerDetailed struct {
	XMLName       xml.Name `xml:"VirtualIOServer"`
	SchemaVersion string   `xml:"schemaVersion,attr"`

	// --- Metadata ---
	Metadata struct {
		AtomID      string `xml:"Atom>AtomID"`
		AtomCreated string `xml:"Atom>AtomCreated"`
	} `xml:"Metadata"`

	// --- Basic Info & Identifiers ---
	PartitionUUID              string  `xml:"PartitionUUID"`
	PartitionID                int     `xml:"PartitionID"`
	PartitionName              string  `xml:"PartitionName"`
	PartitionType              string  `xml:"PartitionType"`
	SystemName                 string  `xml:"SystemName"`
	LogicalSerialNumber        string  `xml:"LogicalSerialNumber"`
	OperatingSystemType        string  `xml:"OperatingSystemType"`
	OperatingSystemVersion     string  `xml:"OperatingSystemVersion"`
	ReferenceCode              string  `xml:"ReferenceCode"`
	LastActivatedProfile       string  `xml:"LastActivatedProfile"`
	DefaultProfileName         string  `xml:"DefaultProfileName"`
	AssociatedPartitionProfile LinkXML `xml:"AssociatedPartitionProfile"`

	// --- State & Uptime ---
	PartitionState                 string `xml:"PartitionState"`
	ResourceMonitoringControlState string `xml:"ResourceMonitoringControlState"`
	ResourceMonitoringIPAddress    string `xml:"ResourceMonitoringIPAddress"`
	Uptime                         struct {
		Value float64 `xml:",chardata"`
		Group string  `xml:"group,attr"`
	} `xml:"Uptime"`
	ProgressPartitionDataRemaining float64 `xml:"ProgressPartitionDataRemaining"`
	ProgressPartitionDataTotal     float64 `xml:"ProgressPartitionDataTotal"`

	// --- Flags & Capabilities ---
	APICapable                            bool   `xml:"APICapable"`
	AllowPerformanceDataCollection        bool   `xml:"AllowPerformanceDataCollection"`
	AvailabilityPriority                  int    `xml:"AvailabilityPriority"`
	BootMode                              string `xml:"BootMode"`
	CurrentProcessorCompatibilityMode     string `xml:"CurrentProcessorCompatibilityMode"`
	CurrentProfileSync                    string `xml:"CurrentProfileSync"`
	CurrentSecureBoot                     int    `xml:"CurrentSecureBoot"`
	FibreChannelPortLabelCapable          bool   `xml:"FibreChannelPortLabelCapable"`
	HasPhysicalIO                         bool   `xml:"HasPhysicalIO"`
	IsBootable                            bool   `xml:"IsBootable"`
	IsOperationInProgress                 bool   `xml:"IsOperationInProgress"`
	IsRedundantErrorPathReportingEnabled  bool   `xml:"IsRedundantErrorPathReportingEnabled"`
	IsServicePartition                    bool   `xml:"IsServicePartition"`
	IsTimeReferencePartition              bool   `xml:"IsTimeReferencePartition"`
	IsVNICCapable                         bool   `xml:"IsVNICCapable"`
	IsVirtualServiceAttentionLEDOn        bool   `xml:"IsVirtualServiceAttentionLEDOn"`
	IsVirtualTrustedPlatformModuleEnabled bool   `xml:"IsVirtualTrustedPlatformModuleEnabled"`
	KeyStoreSize                          int    `xml:"KeyStoreSize"`
	KeylockPosition                       string `xml:"KeylockPosition"`
	ManagerPassthroughCapable             bool   `xml:"ManagerPassthroughCapable"`
	MoverServicePartition                 bool   `xml:"MoverServicePartition"`
	PendingProcessorCompatibilityMode     string `xml:"PendingProcessorCompatibilityMode"`
	PendingSecureBoot                     int    `xml:"PendingSecureBoot"`
	PowerOnWithHypervisor                 bool   `xml:"PowerOnWithHypervisor"`
	PowerVMManagementCapable              bool   `xml:"PowerVMManagementCapable"`
	TCCSlotID                             int    `xml:"TCCSlotID"`
	VNICFailOverCapable                   bool   `xml:"VNICFailOverCapable"`
	VTPMVersion                           string `xml:"VTPMVersion"`
	VirtualIOServerLicenseAccepted        string `xml:"VirtualIOServerLicenseAccepted"`

	// --- Nested Configurations ---
	PartitionCapabilities                   PartitionCapabilities                   `xml:"PartitionCapabilities"`
	PartitionMemoryConfiguration            PartitionMemoryConfiguration            `xml:"PartitionMemoryConfiguration"`
	PartitionProcessorConfiguration         PartitionProcessorConfiguration         `xml:"PartitionProcessorConfiguration"`
	PartitionIOConfiguration                PartitionIOConfiguration                `xml:"PartitionIOConfiguration"`
	AssociatedPersistentMemoryConfiguration AssociatedPersistentMemoryConfiguration `xml:"AssociatedPersistentMemoryConfiguration"`
	HardwareAcceleratorQoS                  HardwareAcceleratorQoS                  `xml:"HardwareAcceleratorQoS"`
	VirtualIOServerCapabilities             VirtualIOServerCapabilities             `xml:"VirtualIOServerCapabilities"`

	// --- Links ---
	AssociatedManagedSystem         LinkXML   `xml:"AssociatedManagedSystem"`
	ClientNetworkAdapters           []LinkXML `xml:"ClientNetworkAdapters>link"`
	HostEthernetAdapterLogicalPorts []LinkXML `xml:"HostEthernetAdapterLogicalPorts>link"`
	LinkAggregations                []LinkXML `xml:"LinkAggregations>link"`
	SRIOVEthernetLogicalPorts       []LinkXML `xml:"SRIOVEthernetLogicalPorts>link"`
	SRIOVRoCELogicalPorts           []LinkXML `xml:"SRIOVRoCELogicalPorts>link"`
	StoragePools                    []LinkXML `xml:"StoragePools>link"`
	VirtualNICBackingDevices        []LinkXML `xml:"VirtualNICBackingDevices>link"`

	// --- Exhaustive Storage & Networking Collections ---
	PhysicalVolumes                  []PhysicalVolume             `xml:"PhysicalVolumes>PhysicalVolume"`
	MediaRepositories                []VirtualMediaRepository     `xml:"MediaRepositories>VirtualMediaRepository"`
	SharedEthernetAdapters           []SharedEthernetAdapter      `xml:"SharedEthernetAdapters>SharedEthernetAdapter"`
	TrunkAdapters                    []TrunkAdapter               `xml:"TrunkAdapters>TrunkAdapter"`
	VirtualFibreChannelMappings      []VirtualFibreChannelMapping `xml:"VirtualFibreChannelMappings>VirtualFibreChannelMapping"`
	VirtualSCSIMappings              []VirtualSCSIMapping         `xml:"VirtualSCSIMappings>VirtualSCSIMapping"`
	FreeEthenetBackingDevicesForSEA  []EthernetBackingDevice      `xml:"FreeEthenetBackingDevicesForSEA>IOAdapterChoice>EthernetBackingDevice"`
	FreeIOAdaptersForLinkAggregation []IOAdapter                  `xml:"FreeIOAdaptersForLinkAggregation>IOAdapterChoice>IOAdapter"`
}

// PartitionCapabilities RENAMED from LparCapabilities
type PartitionCapabilities struct {
	DynamicLogicalPartitionIOCapable                        bool `xml:"DynamicLogicalPartitionIOCapable"`
	DynamicLogicalPartitionMemoryCapable                    bool `xml:"DynamicLogicalPartitionMemoryCapable"`
	DynamicLogicalPartitionProcessorCapable                 bool `xml:"DynamicLogicalPartitionProcessorCapable"`
	DynamicLogicalPartitionVIOSCapable                      bool `xml:"DynamicLogicalPartitionVIOSCapable"`
	InternalAndExternalIntrusionDetectionCapable            bool `xml:"InternalAndExternalIntrusionDetectionCapable"`
	ResourceMonitoringControlOperatingSystemShutdownCapable bool `xml:"ResourceMonitoringControlOperatingSystemShutdownCapable"`
}

// PartitionMemoryConfiguration RENAMED from LparMemoryConfiguration
type PartitionMemoryConfiguration struct {
	ActiveMemoryExpansionEnabled          bool    `xml:"ActiveMemoryExpansionEnabled"`
	ActiveMemorySharingEnabled            bool    `xml:"ActiveMemorySharingEnabled"`
	CurrentExpansionFactor                float64 `xml:"CurrentExpansionFactor"`
	CurrentHardwarePageTableRatio         string  `xml:"CurrentHardwarePageTableRatio"`
	CurrentHugePageCount                  int     `xml:"CurrentHugePageCount"`
	CurrentMaximumHugePageCount           int     `xml:"CurrentMaximumHugePageCount"`
	CurrentMaximumMemory                  float64 `xml:"CurrentMaximumMemory"`
	CurrentMemory                         float64 `xml:"CurrentMemory"`
	CurrentMinimumHugePageCount           int     `xml:"CurrentMinimumHugePageCount"`
	CurrentMinimumMemory                  float64 `xml:"CurrentMinimumMemory"`
	DesiredMemory                         float64 `xml:"DesiredMemory"`
	ExpansionFactor                       float64 `xml:"ExpansionFactor"`
	HardwarePageTableRatio                string  `xml:"HardwarePageTableRatio"`
	MaximumMemory                         float64 `xml:"MaximumMemory"`
	MemoryEncryptionHardwareAccessEnabled bool    `xml:"MemoryEncryptionHardwareAccessEnabled"`
	MemoryExpansionEnabled                bool    `xml:"MemoryExpansionEnabled"`
	MemoryExpansionHardwareAccessEnabled  bool    `xml:"MemoryExpansionHardwareAccessEnabled"`
	MinimumMemory                         float64 `xml:"MinimumMemory"`
	PhysicalPageTableRatio                string  `xml:"PhysicalPageTableRatio"`
	RedundantErrorPathReportingEnabled    bool    `xml:"RedundantErrorPathReportingEnabled"`
	RuntimeHugePageCount                  int     `xml:"RuntimeHugePageCount"`
	RuntimeMemory                         float64 `xml:"RuntimeMemory"`
	RuntimeMinimumMemory                  float64 `xml:"RuntimeMinimumMemory"`
	SharedMemoryEnabled                   bool    `xml:"SharedMemoryEnabled"`
}

// PartitionProcessorConfiguration RENAMED from LparProcessorConfiguration
type PartitionProcessorConfiguration struct {
	HasDedicatedProcessors                 bool                                   `xml:"HasDedicatedProcessors"`
	SharingMode                            string                                 `xml:"SharingMode"`
	CurrentHasDedicatedProcessors          bool                                   `xml:"CurrentHasDedicatedProcessors"`
	CurrentSharingMode                     string                                 `xml:"CurrentSharingMode"`
	RuntimeHasDedicatedProcessors          bool                                   `xml:"RuntimeHasDedicatedProcessors"`
	DedicatedProcessorConfiguration        DedicatedProcessorConfiguration        `xml:"DedicatedProcessorConfiguration"`
	CurrentDedicatedProcessorConfiguration CurrentDedicatedProcessorConfiguration `xml:"CurrentDedicatedProcessorConfiguration"`
}

// DedicatedProcessorConfiguration represents configuration settings
type DedicatedProcessorConfiguration struct {
	DesiredProcessors float64 `xml:"DesiredProcessors"`
	MaximumProcessors float64 `xml:"MaximumProcessors"`
	MinimumProcessors float64 `xml:"MinimumProcessors"`
}

// CurrentDedicatedProcessorConfiguration represents configuration settings
type CurrentDedicatedProcessorConfiguration struct {
	CurrentMaximumProcessors int     `xml:"CurrentMaximumProcessors"`
	CurrentMinimumProcessors int     `xml:"CurrentMinimumProcessors"`
	CurrentProcessors        float64 `xml:"CurrentProcessors"`
	RunProcessors            int     `xml:"RunProcessors"`
}

// PartitionIOConfiguration RENAMED from LparIOConfiguration
type PartitionIOConfiguration struct {
	CurrentMaximumVirtualIOSlots int             `xml:"CurrentMaximumVirtualIOSlots"`
	MaximumVirtualIOSlots        int             `xml:"MaximumVirtualIOSlots"`
	ProfileIOSlots               []ProfileIOSlot `xml:"ProfileIOSlots>ProfileIOSlot"`
}

// ProfileIOSlot represents profile information
type ProfileIOSlot struct {
	AssociatedIOSlot AssociatedIOSlot `xml:"AssociatedIOSlot"`
}

// AssociatedIOSlot represents data structure
type AssociatedIOSlot struct {
	BusGroupingRequired                      bool             `xml:"BusGroupingRequired"`
	Description                              string           `xml:"Description"`
	FeatureCodes                             []string         `xml:"FeatureCodes"`
	IOBusID                                  int              `xml:"IOBusID"`
	IOUnitPhysicalLocation                   string           `xml:"IOUnitPhysicalLocation"`
	PCAdapterID                              string           `xml:"PCAdapterID"`
	PCIClass                                 string           `xml:"PCIClass"`
	PCIDeviceID                              string           `xml:"PCIDeviceID"`
	PCIManufacturerID                        string           `xml:"PCIManufacturerID"`
	PCIRevisionID                            string           `xml:"PCIRevisionID"`
	PCISubsystemDeviceID                     string           `xml:"PCISubsystemDeviceID"`
	PCISubsystemVendorID                     string           `xml:"PCISubsystemVendorID"`
	PCIVendorID                              string           `xml:"PCIVendorID"`
	PartitionID                              int              `xml:"PartitionID"`
	PartitionName                            string           `xml:"PartitionName"`
	PartitionType                            string           `xml:"PartitionType"`
	SRIOVCapableDevice                       bool             `xml:"SRIOVCapableDevice"`
	SRIOVCapableSlot                         bool             `xml:"SRIOVCapableSlot"`
	SRIOVLogicalPortsLimit                   int              `xml:"SRIOVLogicalPortsLimit"`
	SlotDynamicReconfigurationConnectorIndex string           `xml:"SlotDynamicReconfigurationConnectorIndex"`
	SlotDynamicReconfigurationConnectorName  string           `xml:"SlotDynamicReconfigurationConnectorName"`
	SlotPhysicalLocationCode                 string           `xml:"SlotPhysicalLocationCode"`
	RelatedIBMiIOSlot                        IBMiIOSlot       `xml:"RelatedIBMiIOSlot"`
	RelatedIOAdapter                         RelatedIOAdapter `xml:"RelatedIOAdapter"`
}

// RelatedIOAdapter handles the complex physical adapters, including Fibre Channel
type RelatedIOAdapter struct {
	IOAdapter                   IOAdapter                   `xml:"IOAdapter"`
	PhysicalFibreChannelAdapter PhysicalFibreChannelAdapter `xml:"PhysicalFibreChannelAdapter"`
}

// PhysicalFibreChannelAdapter captures the deeply nested Fibre Channel properties
type PhysicalFibreChannelAdapter struct {
	AdapterID                           string                     `xml:"AdapterID"`
	Description                         string                     `xml:"Description"`
	DeviceName                          string                     `xml:"DeviceName"`
	DynamicReconfigurationConnectorName string                     `xml:"DynamicReconfigurationConnectorName"`
	PhysicalLocation                    string                     `xml:"PhysicalLocation"`
	PhysicalFibreChannelPorts           []PhysicalFibreChannelPort `xml:"PhysicalFibreChannelPorts>PhysicalFibreChannelPort"`
}

// PhysicalFibreChannelPort represents a port
type PhysicalFibreChannelPort struct {
	AvailablePorts  string           `xml:"AvailablePorts"`
	LocationCode    string           `xml:"LocationCode"`
	PortName        string           `xml:"PortName"`
	TotalPorts      string           `xml:"TotalPorts"`
	UniqueDeviceID  string           `xml:"UniqueDeviceID"`
	WWNN            string           `xml:"WWNN"`
	WWPN            string           `xml:"WWPN"`
	PhysicalVolumes []PhysicalVolume `xml:"PhysicalVolumes>PhysicalVolume"`
}

// IOAdapter RENAMED from IOAdapterXML
type IOAdapter struct {
	AdapterID                           string `xml:"AdapterID"`
	Description                         string `xml:"Description"`
	DeviceName                          string `xml:"DeviceName"`
	DeviceType                          string `xml:"DeviceType"`
	DynamicPartitionAssignmentCapable   bool   `xml:"DynamicPartitionAssignmentCapable"`
	DynamicReconfigurationConnectorName string `xml:"DynamicReconfigurationConnectorName"`
	LogicalPartitionAssignmentCapable   bool   `xml:"LogicalPartitionAssignmentCapable"`
	PhysicalLocation                    string `xml:"PhysicalLocation"`
	UniqueDeviceID                      string `xml:"UniqueDeviceID"`
}

// AssociatedPersistentMemoryConfiguration RENAMED from LparPersistentMemoryConfiguration
type AssociatedPersistentMemoryConfiguration struct {
	CurrentDramPersistentMemoryVolumes int `xml:"CurrentDramPersistentMemoryVolumes"`
	CurrentPersistentMemoryVolumes     int `xml:"CurrentPersistentMemoryVolumes"`
	MaximumDramPersistentMemoryVolumes int `xml:"MaximumDramPersistentMemoryVolumes"`
	MaximumPersistentMemoryVolumes     int `xml:"MaximumPersistentMemoryVolumes"`
}

// HardwareAcceleratorQoS represents data structure
type HardwareAcceleratorQoS struct {
	Metadata struct {
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
}

// VirtualIOServerCapabilities represents data structure
type VirtualIOServerCapabilities struct {
	GPFSCapable         bool `xml:"GPFSCapable"`
	IsTierCapable       bool `xml:"IsTierCapable"`
	IsTierMirrorCapable bool `xml:"IsTierMirrorCapable"`
}

// VirtualMediaRepository represents data structure
type VirtualMediaRepository struct {
	RepositoryName      string                `xml:"RepositoryName"`
	RepositorySize      float64               `xml:"RepositorySize"`
	VirtualOpticalMedia []VirtualOpticalMedia `xml:"OpticalMedia>VirtualOpticalMedia"`
}

// VirtualOpticalMedia applies to both Media Repositories and Virtual SCSI Mappings
type VirtualOpticalMedia struct {
	MediaName string `xml:"MediaName"`
	MediaUDID string `xml:"MediaUDID"`
	MountType string `xml:"MountType"`
	Size      string `xml:"Size"`
}

// SharedEthernetAdapter represents an adapter configuration
type SharedEthernetAdapter struct {
	ConfigurationState   string `xml:"ConfigurationState"`
	DeviceName           string `xml:"DeviceName"`
	HighAvailabilityMode string `xml:"HighAvailabilityMode"`
	JumboFramesEnabled   bool   `xml:"JumboFramesEnabled"`
	LargeSend            bool   `xml:"LargeSend"`
	PortVLANID           int    `xml:"PortVLANID"`
	QualityOfServiceMode string `xml:"QualityOfServiceMode"`
	QueueSize            int    `xml:"QueueSize"`
	ThreadModeEnabled    bool   `xml:"ThreadModeEnabled"`
	UniqueDeviceID       string `xml:"UniqueDeviceID"`
	IPInterface          struct {
		InterfaceName string `xml:"InterfaceName"`
		State         string `xml:"State"`
	} `xml:"IPInterface"`
	BackingDeviceChoice struct {
		EthernetBackingDevice EthernetBackingDevice `xml:"EthernetBackingDevice"`
	} `xml:"BackingDeviceChoice"`
	TrunkAdapters []TrunkAdapter `xml:"TrunkAdapters>TrunkAdapter"`
}

// EthernetBackingDevice represents a device
type EthernetBackingDevice struct {
	AdapterID        string `xml:"AdapterID"`
	Description      string `xml:"Description"`
	DeviceName       string `xml:"DeviceName"`
	DeviceType       string `xml:"DeviceType"`
	PhysicalLocation string `xml:"PhysicalLocation"`
	UniqueDeviceID   string `xml:"UniqueDeviceID"`
	IPInterface      struct {
		InterfaceName string `xml:"InterfaceName"`
		IPAddress     string `xml:"IPAddress"`
		SubnetMask    string `xml:"SubnetMask"`
		State         string `xml:"State"`
	} `xml:"IPInterface"`
}

// TrunkAdapter represents an adapter configuration
type TrunkAdapter struct {
	AllowedOperatingSystemMACAddresses  string  `xml:"AllowedOperatingSystemMACAddresses"`
	AssociatedVirtualSwitch             LinkXML `xml:"AssociatedVirtualSwitch>link"`
	DeviceName                          string  `xml:"DeviceName"`
	DynamicReconfigurationConnectorName string  `xml:"DynamicReconfigurationConnectorName"`
	HCNID                               string  `xml:"HCNID"`
	LocalPartitionID                    int     `xml:"LocalPartitionID"`
	LocationCode                        string  `xml:"LocationCode"`
	MACAddress                          string  `xml:"MACAddress"`
	PortVLANID                          int     `xml:"PortVLANID"`
	QualityOfServicePriorityEnabled     string  `xml:"QualityOfServicePriorityEnabled"` // Often represented as string "true"/"false"
	RequiredAdapter                     string  `xml:"RequiredAdapter"`
	TaggedVLANIDs                       string  `xml:"TaggedVLANIDs"`
	TaggedVLANSupported                 string  `xml:"TaggedVLANSupported"`
	TrunkPriority                       int     `xml:"TrunkPriority"`
	VariedOn                            string  `xml:"VariedOn"`
	VirtualSlotNumber                   int     `xml:"VirtualSlotNumber"`
	VirtualSwitchID                     string  `xml:"VirtualSwitchID"`
	VirtualSwitchName                   string  `xml:"VirtualSwitchName"`
}

// --- VIRTUAL FIBRE CHANNEL MAPPINGS ---

// VirtualFibreChannelMapping represents data structure
type VirtualFibreChannelMapping struct {
	AssociatedLogicalPartition LinkXML       `xml:"AssociatedLogicalPartition"`
	ClientAdapter              ClientAdapter `xml:"ClientAdapter"`
	ServerAdapter              ServerAdapter `xml:"ServerAdapter"`
	Port                       Port          `xml:"Port"`
}

// ClientAdapter represents an adapter configuration
type ClientAdapter struct {
	AdapterType                         string `xml:"AdapterType"`
	ConnectingPartitionID               int    `xml:"ConnectingPartitionID"`
	ConnectingVirtualSlotNumber         int    `xml:"ConnectingVirtualSlotNumber"`
	DynamicReconfigurationConnectorName string `xml:"DynamicReconfigurationConnectorName"`
	LocalPartitionID                    int    `xml:"LocalPartitionID"`
	LocationCode                        string `xml:"LocationCode"`
	RemoteLogicalPartitionID            int    `xml:"RemoteLogicalPartitionID"`
	RemoteSlotNumber                    int    `xml:"RemoteSlotNumber"`
	RequiredAdapter                     string `xml:"RequiredAdapter"`
	ServerLocationCode                  string `xml:"ServerLocationCode"`
	VariedOn                            string `xml:"VariedOn"`
	VirtualSlotNumber                   int    `xml:"VirtualSlotNumber"`
	WWPNs                               string `xml:"WWPNs"`
}

// ServerAdapter represents an adapter configuration
type ServerAdapter struct {
	AdapterName                         string `xml:"AdapterName"`
	AdapterType                         string `xml:"AdapterType"`
	BackingDeviceName                   string `xml:"BackingDeviceName"`
	ConnectingPartitionID               int    `xml:"ConnectingPartitionID"`
	ConnectingVirtualSlotNumber         int    `xml:"ConnectingVirtualSlotNumber"`
	DynamicReconfigurationConnectorName string `xml:"DynamicReconfigurationConnectorName"`
	LocalPartitionID                    int    `xml:"LocalPartitionID"`
	LocationCode                        string `xml:"LocationCode"`
	MapPort                             string `xml:"MapPort"`
	RemoteLogicalPartitionID            int    `xml:"RemoteLogicalPartitionID"`
	RemoteSlotNumber                    int    `xml:"RemoteSlotNumber"`
	RequiredAdapter                     string `xml:"RequiredAdapter"`
	ServerLocationCode                  string `xml:"ServerLocationCode"`
	UniqueDeviceID                      string `xml:"UniqueDeviceID"`
	VariedOn                            string `xml:"VariedOn"`
	VirtualSlotNumber                   int    `xml:"VirtualSlotNumber"`
	PhysicalPort                        Port   `xml:"PhysicalPort"` // Found inside ServerAdapter for VFC
}

// Port represents a port
type Port struct {
	AvailablePorts string `xml:"AvailablePorts"`
	LocationCode   string `xml:"LocationCode"`
	PortName       string `xml:"PortName"`
	TotalPorts     string `xml:"TotalPorts"`
	UniqueDeviceID string `xml:"UniqueDeviceID"`
	WWNN           string `xml:"WWNN"`
	WWPN           string `xml:"WWPN"`
}

// --- VIRTUAL SCSI MAPPINGS ---

// VirtualSCSIMapping represents data structure
type VirtualSCSIMapping struct {
	AssociatedLogicalPartition LinkXML       `xml:"AssociatedLogicalPartition"`
	ClientAdapter              ClientAdapter `xml:"ClientAdapter"` // Reusing ClientAdapter Struct
	ServerAdapter              ServerAdapter `xml:"ServerAdapter"` // Reusing ServerAdapter Struct
	Storage                    Storage       `xml:"Storage"`
	TargetDevice               TargetDevice  `xml:"TargetDevice"`
}

// Storage represents storage information
type Storage struct {
	PhysicalVolume      PhysicalVolume      `xml:"PhysicalVolume"`
	VirtualOpticalMedia VirtualOpticalMedia `xml:"VirtualOpticalMedia"`
	VirtualDisk         VirtualDisk         `xml:"VirtualDisk"`
}

// TargetDevice represents a device
type TargetDevice struct {
	PhysicalVolumeVirtualTargetDevice PhysicalVolumeVirtualTargetDevice `xml:"PhysicalVolumeVirtualTargetDevice"`
	VirtualOpticalTargetDevice        VirtualOpticalTargetDevice        `xml:"VirtualOpticalTargetDevice"`
	LogicalVolumeVirtualTargetDevice  LogicalVolumeVirtualTargetDevice  `xml:"LogicalVolumeVirtualTargetDevice"`
}

// LogicalVolumeVirtualTargetDevice represents a device
type LogicalVolumeVirtualTargetDevice struct {
	LogicalUnitAddress string `xml:"LogicalUnitAddress"`
	TargetName         string `xml:"TargetName"`
	UniqueDeviceID     string `xml:"UniqueDeviceID"`
}

// PhysicalVolumeVirtualTargetDevice represents a device
type PhysicalVolumeVirtualTargetDevice struct {
	LogicalUnitAddress string `xml:"LogicalUnitAddress"`
	TargetName         string `xml:"TargetName"`
	UniqueDeviceID     string `xml:"UniqueDeviceID"`
}

// VirtualOpticalTargetDevice represents a device
type VirtualOpticalTargetDevice struct {
	LogicalUnitAddress string `xml:"LogicalUnitAddress"`
	TargetName         string `xml:"TargetName"`
	UniqueDeviceID     string `xml:"UniqueDeviceID"`
}

// VirtualSCSIServerAdapterEntry represents a single entry in the feed
type VirtualSCSIServerAdapterEntry struct {
	XMLName xml.Name `xml:"entry"`
	ID      string   `xml:"id"`
	Link    struct {
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Content struct {
		Adapter VirtualSCSIServerAdapter `xml:"VirtualSCSIServerAdapter"`
	} `xml:"content"`
}

// VirtualSCSIServerAdapter represents a Virtual SCSI Server Adapter (vhost) on a VIOS.
type VirtualSCSIServerAdapter struct {
	XMLName xml.Name `xml:"VirtualSCSIServerAdapter"`

	// Metadata - UUID populated directly from XML
	UUID string `xml:"Metadata>Atom>AtomID"`

	// Adapter Properties
	AdapterType                         string `xml:"AdapterType"`
	DynamicReconfigurationConnectorName string `xml:"DynamicReconfigurationConnectorName"`
	LocationCode                        string `xml:"LocationCode"`
	LocalPartitionID                    string `xml:"LocalPartitionID"`
	RequiredAdapter                     string `xml:"RequiredAdapter"`
	VariedOn                            string `xml:"VariedOn"`
	VirtualSlotNumber                   string `xml:"VirtualSlotNumber"`
	RemoteLogicalPartitionID            string `xml:"RemoteLogicalPartitionID"`
	RemoteSlotNumber                    string `xml:"RemoteSlotNumber"`

	// Entry-level fields (populated from parent entry when using VirtualSCSIServerAdapterEntry)
	ID   string `xml:"-"` // From entry/id (deprecated, use UUID instead)
	Link string `xml:"-"` // From entry/link/@href
}

// =====================================================================
// VOLUME GROUP DATA STRUCTURES
// =====================================================================

// VolumeGroup represents a Volume Group configured on a Virtual I/O Server.
type VolumeGroup struct {
	XMLName xml.Name `xml:"VolumeGroup"`

	// Metadata - UUID populated directly from XML
	UUID string `xml:"Metadata>Atom>AtomID"`

	// Volume Group Properties
	AvailableSize         string `xml:"AvailableSize"`
	FreeSpace             string `xml:"FreeSpace"`
	GroupCapacity         string `xml:"GroupCapacity"`
	GroupName             string `xml:"GroupName"`
	GroupSerialID         string `xml:"GroupSerialID"`
	MaximumLogicalVolumes string `xml:"MaximumLogicalVolumes"`
	UniqueDeviceID        string `xml:"UniqueDeviceID"`

	// Collections
	PhysicalVolumes []PhysicalVolume      `xml:"PhysicalVolumes>PhysicalVolume"`
	VirtualDisks    []VirtualDisk         `xml:"VirtualDisks>VirtualDisk"`
	OpticalMedia    []VirtualOpticalMedia `xml:"MediaRepositories>VirtualMediaRepository>OpticalMedia>VirtualOpticalMedia"`

	// Media Repository fields
	MediaRepositoryName string `xml:"MediaRepositories>VirtualMediaRepository>RepositoryName"`
	MediaRepositorySize string `xml:"MediaRepositories>VirtualMediaRepository>RepositorySize"`
}

// VirtualDisk represents a Logical Volume created inside the Volume Group.
type VirtualDisk struct {
	XMLName        xml.Name `xml:"VirtualDisk"`
	DiskName       string   `xml:"DiskName"`
	DiskCapacity   string   `xml:"DiskCapacity"`
	DiskLabel      string   `xml:"DiskLabel"`
	UniqueDeviceID string   `xml:"UniqueDeviceID"`
}

// CreateLparRequest defines the parameters for a new LPAR creation.
type CreateLparRequest struct {
	Name             string
	OsType           string  // e.g., "AIX/Linux", "OS400", or "Virtual IO Server"
	MinMem           int     // MB
	DesiredMem       int     // MB
	MaxMem           int     // MB
	MinProcUnits     float64 // For shared: processing units; For dedicated: number of processors
	DesiredProcUnits float64 // For shared: processing units; For dedicated: number of processors
	MaxProcUnits     float64 // For shared: processing units; For dedicated: number of processors
	MinVcpus         int     // For shared: virtual processors; For dedicated: ignored
	DesiredVcpus     int     // For shared: virtual processors; For dedicated: ignored
	MaxVcpus         int     // For shared: virtual processors; For dedicated: ignored
	SharingMode      string  // For shared: "uncapped" or "capped"; For dedicated: "keep idle procs" or "share idle procs"
	UncappedWeight   int     // For shared uncapped mode: processor weight (0-255, default 128 if not specified)
	MaxVirtualSlots  int     // Maximum number of virtual I/O adapter slots (default 200 if not specified)
	ResourceGroupID  string  // Resource group association (default "0" for Default group if not specified)
	DedicatedProc    bool    // true = dedicated processors, false = shared processors (default)
}

// CreateViosRequest defines the parameters for a new Virtual I/O Server (VIOS) creation.
type CreateViosRequest struct {
	Name             string
	MinMem           int     // MB
	DesiredMem       int     // MB
	MaxMem           int     // MB
	MinProcUnits     float64 // For shared: processing units; For dedicated: number of processors
	DesiredProcUnits float64 // For shared: processing units; For dedicated: number of processors
	MaxProcUnits     float64 // For shared: processing units; For dedicated: number of processors
	MinVcpus         int     // For shared: virtual processors; For dedicated: ignored
	DesiredVcpus     int     // For shared: virtual processors; For dedicated: ignored
	MaxVcpus         int     // For shared: virtual processors; For dedicated: ignored
	SharingMode      string  // For shared: "uncapped" or "capped"; For dedicated: "keep idle procs" or "share idle procs"
	UncappedWeight   int     // For shared uncapped mode: processor weight (0-255, default 128 if not specified)
	MaxVirtualSlots  int     // Maximum number of virtual I/O adapter slots (default 500 for VIOS if not specified)
	ResourceGroupID  string  // Resource group association (default "0" for Default group if not specified)
	DedicatedProc    bool    // true = dedicated processors, false = shared processors (default)
}

// VirtualSwitchQuick represents the JSON response for quick switch details.
type VirtualSwitchQuick struct {
	UUID       string `json:"UUID"` // Note: Only present in /quick/All, we inject it for single queries
	SwitchName string `json:"SwitchName"`
	SwitchMode string `json:"SwitchMode"`
}

// VirtualSwitch represents the comprehensive XML details of a switch.
type VirtualSwitch struct {
	UUID            string
	SwitchID        string
	SwitchName      string
	SwitchMode      string
	VirtualNetworks []string // Slice to hold the href links to attached VirtualNetworks
}

// ClientNetworkAdapter represents the comprehensive XML details of a Virtual Ethernet Adapter.
type ClientNetworkAdapter struct {
	XMLName                             xml.Name `xml:"ClientNetworkAdapter"`
	UUID                                string   `xml:"Metadata>Atom>AtomID"`
	DynamicReconfigurationConnectorName string   `xml:"DynamicReconfigurationConnectorName"`
	LocationCode                        string   `xml:"LocationCode"`
	LocalPartitionID                    string   `xml:"LocalPartitionID"`
	RequiredAdapter                     string   `xml:"RequiredAdapter"`
	VariedOn                            string   `xml:"VariedOn"`
	VirtualSlotNumber                   string   `xml:"VirtualSlotNumber"`
	AllowedOperatingSystemMACAddresses  string   `xml:"AllowedOperatingSystemMACAddresses"`
	MACAddress                          string   `xml:"MACAddress"`
	PortVLANID                          string   `xml:"PortVLANID"`
	QualityOfServicePriorityEnabled     string   `xml:"QualityOfServicePriorityEnabled"`
	TaggedVLANSupported                 string   `xml:"TaggedVLANSupported"`
	VirtualSwitchID                     string   `xml:"VirtualSwitchID"`
	VirtualSwitchName                   string   `xml:"VirtualSwitchName"`
	HCNID                               string   `xml:"HCNID"`

	// Kept your original naming, but mapped to LinkXML to capture the 'href' attribute
	AssociatedVirtualSwitchURI LinkXML   `xml:"AssociatedVirtualSwitch>link"`
	VirtualNetworkURIs         []LinkXML `xml:"VirtualNetworks>link"`
}

// NetworkBootDeviceCollection represents the collection of network boot devices returned by HMC
type NetworkBootDeviceCollection struct {
	XMLName       xml.Name `xml:"NetworkBootDevice_Collection"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	Metadata      struct {
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	Devices []NetworkBootDeviceXML `xml:"NetworkBootDevice"`
}

// NetworkBootDeviceXML represents a network boot device as returned by HMC REST API
type NetworkBootDeviceXML struct {
	XMLName       xml.Name `xml:"NetworkBootDevice"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	Metadata      struct {
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	BootDevice       string `xml:"BootDevice"`       // Device name (e.g., "ent")
	IsPhysicalDevice string `xml:"IsPhysicalDevice"` // "true" or "false" as string
	LocationCode     string `xml:"LocationCode"`     // Physical location code with port suffix
	MACAddressValue  string `xml:"MACAddressValue"`  // MAC address
}

// NetworkBootDevice represents a network boot device from an LPAR profile.
type NetworkBootDevice struct {
	DeviceName   string `json:"device_name"`   // Name of the network device (e.g., "ent")
	DeviceType   string `json:"device_type"`   // Type: "physical" or "virtual"
	LocationCode string `json:"location_code"` // Physical location code of the adapter
	MACAddress   string `json:"mac_address"`   // MAC address of the adapter
	AdapterID    string `json:"adapter_id"`    // Adapter identifier (if available)
	BootPriority int    `json:"boot_priority"` // Boot priority order (if available)
}

// =====================================================================
// LOGICAL PARTITION PROFILE DATA STRUCTURES
// =====================================================================

// AssociatedPartitionLink represents the link to the associated partition
type AssociatedPartitionLink struct {
	Href string `xml:"href,attr"`
}

// LogicalPartitionProfile represents a complete LPAR profile configuration
type LogicalPartitionProfile struct {
	XMLName xml.Name `xml:"LogicalPartitionProfile"`

	// Metadata
	UUID        string `xml:"Metadata>Atom>AtomID"`
	AtomCreated string `xml:"Metadata>Atom>AtomCreated"`

	// Basic Profile Information
	ProfileName string `xml:"ProfileName"`
	ProfileType string `xml:"ProfileType"`
	SettingID   string `xml:"SettingID"`

	// Partition Association
	AssociatedPartition AssociatedPartitionLink `xml:"AssociatedPartition"`

	// Configuration Flags
	AffinityGroupID                    string `xml:"AffinityGroupID"`
	AssignAllResources                 string `xml:"AssignAllResources"`
	AutoStart                          string `xml:"AutoStart"`
	BootMode                           string `xml:"BootMode"`
	ConnectionMonitoringEnabled        string `xml:"ConnectionMonitoringEnabled"`
	DesiredProcessorCompatibilityMode  string `xml:"DesiredProcessorCompatibilityMode"`
	RedundantErrorPathReportingEnabled string `xml:"RedundantErrorPathReportingEnabled"`

	// I/O Configuration
	MaximumVirtualIOSlots string `xml:"IOConfigurationInstance>MaximumVirtualIOSlots"`

	// Processor Configuration
	ProcessorConfig ProfileProcessorConfig `xml:"ProcessorAttributes"`

	// Memory Configuration
	MemoryConfig ProfileMemoryConfig `xml:"ProfileMemory"`
}

// ProfileProcessorConfig holds processor-related configuration for a profile
type ProfileProcessorConfig struct {
	HasDedicatedProcessors string `xml:"HasDedicatedProcessors"`
	SharingMode            string `xml:"SharingMode"`

	// Shared Processor Configuration
	SharedConfig SharedProcessorConfig `xml:"SharedProcessorConfiguration"`

	// Dedicated Processor Configuration
	DedicatedConfig DedicatedProcessorConfig `xml:"DedicatedProcessorConfiguration"`
}

// SharedProcessorConfig holds shared processor configuration
type SharedProcessorConfig struct {
	DesiredProcessingUnits   string `xml:"DesiredProcessingUnits"`
	DesiredVirtualProcessors string `xml:"DesiredVirtualProcessors"`
	MaximumProcessingUnits   string `xml:"MaximumProcessingUnits"`
	MaximumVirtualProcessors string `xml:"MaximumVirtualProcessors"`
	MinimumProcessingUnits   string `xml:"MinimumProcessingUnits"`
	MinimumVirtualProcessors string `xml:"MinimumVirtualProcessors"`
	SharedProcessorPoolID    string `xml:"SharedProcessorPoolID"`
	SharedProcessorPoolName  string `xml:"SharedProcessorPoolName"`
	UncappedWeight           string `xml:"UncappedWeight"`
}

// DedicatedProcessorConfig holds dedicated processor configuration
type DedicatedProcessorConfig struct {
	DesiredProcessors string `xml:"DesiredProcessors"`
	MaximumProcessors string `xml:"MaximumProcessors"`
	MinimumProcessors string `xml:"MinimumProcessors"`
}

// ProfileMemoryConfig holds memory-related configuration for a profile
type ProfileMemoryConfig struct {
	DesiredMemory string `xml:"DesiredMemory"`
	MaximumMemory string `xml:"MaximumMemory"`
	MinimumMemory string `xml:"MinimumMemory"`

	// Advanced Memory Features
	ActiveMemoryExpansionEnabled string `xml:"ActiveMemoryExpansionEnabled"`
	ActiveMemorySharingEnabled   string `xml:"ActiveMemorySharingEnabled"`

	// Huge Pages
	DesiredHugePageCount string `xml:"DesiredHugePageCount"`
	MaximumHugePageCount string `xml:"MaximumHugePageCount"`
	MinimumHugePageCount string `xml:"MinimumHugePageCount"`

	// Page Table Configuration
	ExpansionFactor               string `xml:"ExpansionFactor"`
	HardwarePageTableRatio        string `xml:"HardwarePageTableRatio"`
	DesiredPhysicalPageTableRatio string `xml:"DesiredPhysicalPageTableRatio"`
}

// =====================================================================
// EXHAUSTIVE MANAGED SYSTEM XML STRUCTURES
// =====================================================================

// LinkXML represents a link
type LinkXML struct {
	Href string `xml:"href,attr"`
}

// MachineTypeModelAndSerialNumber represents data structure
type MachineTypeModelAndSerialNumber struct {
	MachineType  string `xml:"MachineType"`
	Model        string `xml:"Model"`
	SerialNumber string `xml:"SerialNumber"`
}

// ManagedSystemDetailed represents detailed information
type ManagedSystemDetailed struct {
	XMLName                                          xml.Name                            `xml:"ManagedSystem"`
	MetadataID                                       string                              `xml:"Metadata>Atom>AtomID"`
	ActivatedLevel                                   string                              `xml:"ActivatedLevel"`
	ActivatedServicePackNameAndLevel                 string                              `xml:"ActivatedServicePackNameAndLevel"`
	IPLConfig                                        SystemIPLConfiguration              `xml:"AssociatedIPLConfiguration"`
	AssociatedLogicalPartitions                      []LinkXML                           `xml:"AssociatedLogicalPartitions>link"`
	Capabilities                                     SystemCapabilities                  `xml:"AssociatedSystemCapabilities"`
	IOConfig                                         SystemIOConfiguration               `xml:"AssociatedSystemIOConfiguration"`
	MemoryConfig                                     SystemMemoryConfiguration           `xml:"AssociatedSystemMemoryConfiguration"`
	ProcessorConfig                                  SystemProcessorConfiguration        `xml:"AssociatedSystemProcessorConfiguration"`
	SecurityConfig                                   SystemSecurityConfiguration         `xml:"AssociatedSystemSecurity"`
	AssociatedVirtualIOServers                       []LinkXML                           `xml:"AssociatedVirtualIOServers>link"`
	DetailedState                                    string                              `xml:"DetailedState"`
	MTMS                                             MachineTypeModelAndSerialNumber     `xml:"MachineTypeModelAndSerialNumber"`
	ManufacturingDefaultConfigurationEnabled         bool                                `xml:"ManufacturingDefaultConfigurationEnabled"`
	MaximumPartitions                                float64                             `xml:"MaximumPartitions"`
	MaximumPowerControlPartitions                    float64                             `xml:"MaximumPowerControlPartitions"`
	SRIOVAdapters                                    []SRIOVAdapter                      `xml:"SRIOVAdapters>IOAdapterChoice>SRIOVAdapter"`
	MaximumRemoteRestartPartitions                   string                              `xml:"MaximumRemoteRestartPartitions"`
	MaximumSharedProcessorCapablePartitionID         string                              `xml:"MaximumSharedProcessorCapablePartitionID"`
	MaximumSuspendablePartitions                     string                              `xml:"MaximumSuspendablePartitions"`
	MaximumBackingDevicesPerVNIC                     string                              `xml:"MaximumBackingDevicesPerVNIC"`
	PhysicalSystemAttentionLEDState                  bool                                `xml:"PhysicalSystemAttentionLEDState"`
	VirtualSystemAttentionLEDState                   bool                                `xml:"VirtualSystemAttentionLEDState"`
	PrimaryIPAddress                                 string                              `xml:"PrimaryIPAddress"`
	Hostname                                         string                              `xml:"Hostname"`
	ServiceProcessorFailoverEnabled                  bool                                `xml:"ServiceProcessorFailoverEnabled"`
	ServiceProcessorFailoverReason                   string                              `xml:"ServiceProcessorFailoverReason"`
	ServiceProcessorFailoverState                    string                              `xml:"ServiceProcessorFailoverState"`
	ServiceProcessorVersion                          string                              `xml:"ServiceProcessorVersion"`
	State                                            string                              `xml:"State"`
	StateDetail                                      string                              `xml:"StateDetail"`
	SystemName                                       string                              `xml:"SystemName"`
	SystemTime                                       string                              `xml:"SystemTime"`
	MigrationInfo                                    SystemMigrationInformation          `xml:"SystemMigrationInformation"`
	ReferenceCode                                    string                              `xml:"ReferenceCode"`
	MergedReferenceCode                              string                              `xml:"MergedReferenceCode"`
	SystemFirmware                                   string                              `xml:"SystemFirmware"`
	EnergyManagementConfig                           SystemEnergyManagementConfiguration `xml:"EnergyManagementConfiguration"`
	IsPowerVMManagementMaster                        bool                                `xml:"IsPowerVMManagementMaster"`
	IsPowerVMManagementController                    bool                                `xml:"IsPowerVMManagementController"`
	IsClassicHMCManagement                           bool                                `xml:"IsClassicHMCManagement"`
	IsPowerVMManagementWithoutMaster                 bool                                `xml:"IsPowerVMManagementWithoutMaster"`
	IsPowerVMManagementWithoutController             bool                                `xml:"IsPowerVMManagementWithoutController"`
	IsManagementPartitionPowerVMManagementMaster     bool                                `xml:"IsManagementPartitionPowerVMManagementMaster"`
	IsManagementPartitionPowerVMManagementController bool                                `xml:"IsManagementPartitionPowerVMManagementController"`
	IsHMCPowerVMManagementMaster                     bool                                `xml:"IsHMCPowerVMManagementMaster"`
	IsHMCPowerVMManagementController                 bool                                `xml:"IsHMCPowerVMManagementController"`
	IsNotPowerVMManagementMaster                     bool                                `xml:"IsNotPowerVMManagementMaster"`
	IsNotPowerVMManagementController                 bool                                `xml:"IsNotPowerVMManagementController"`
	IsPowerVMManagementNormalMaster                  bool                                `xml:"IsPowerVMManagementNormalMaster"`
	IsPowerVMManagementNormalController              bool                                `xml:"IsPowerVMManagementNormalController"`
	IsPowerVMManagementPersistentMaster              bool                                `xml:"IsPowerVMManagementPersistentMaster"`
	IsPowerVMManagementPersistentController          bool                                `xml:"IsPowerVMManagementPersistentController"`
	IsPowerVMManagementTemporaryMaster               bool                                `xml:"IsPowerVMManagementTemporaryMaster"`
	IsPowerVMManagementTemporaryController           bool                                `xml:"IsPowerVMManagementTemporaryController"`
	IsPowerVMManagementPartitionEnabled              bool                                `xml:"IsPowerVMManagementPartitionEnabled"`
	HardwareAccelerators                             []HardwareAcceleratorType           `xml:"SupportedHardwareAcceleratorTypes>HardwareAcceleratorType"`
	CurrentStealableProcUnits                        float64                             `xml:"CurrentStealableProcUnits"`
	CurrentStealableMemory                           float64                             `xml:"CurrentStealableMemory"`
	MinimumKeyStoreSize                              string                              `xml:"MinimumKeyStoreSize"`
	MaximumkeyStoreSize                              string                              `xml:"MaximumkeyStoreSize"`
	Uptime                                           string                              `xml:"Uptime"`
	Description                                      string                              `xml:"Description"`
	SystemType                                       string                              `xml:"SystemType"`
	ProcessorThrottling                              bool                                `xml:"ProcessorThrottling"`
	SupportedVTPMVersions                            string                              `xml:"SupportedVTPMVersions"`
	PersistentMemoryConfig                           SystemPersistentMemoryConfiguration `xml:"AssociatedPersistentMemoryConfiguration"`
	ASMGeneralPasswordUpdatedRequired                bool                                `xml:"ASMGeneralPasswordUpdatedRequired"`
	ASMAdminPasswordUpdatedRequired                  bool                                `xml:"ASMAdminPasswordUpdatedRequired"`
	PlatformPasswordUpdateRequired                   bool                                `xml:"PlatformPasswordUpdateRequired"`
}

// SystemIPLConfiguration represents configuration settings
type SystemIPLConfiguration struct {
	CurrentManufacturingDefaulConfigurationtBootMode string `xml:"CurrentManufacturingDefaulConfigurationtBootMode"`
	CurrentPowerOnSide                               string `xml:"CurrentPowerOnSide"`
	CurrentSystemKeylock                             string `xml:"CurrentSystemKeylock"`
	MajorBootType                                    string `xml:"MajorBootType"`
	MinorBootType                                    string `xml:"MinorBootType"`
	PendingManufacturingDefaulConfigurationtBootMode string `xml:"PendingManufacturingDefaulConfigurationtBootMode"`
	PendingPowerOnSide                               string `xml:"PendingPowerOnSide"`
	PendingSystemKeylock                             string `xml:"PendingSystemKeylock"`
	PowerOnLogicalPartitionStartPolicy               string `xml:"PowerOnLogicalPartitionStartPolicy"`
	PowerOnOption                                    string `xml:"PowerOnOption"`
	PowerOnSpeed                                     string `xml:"PowerOnSpeed"`
	PowerOnSpeedOverride                             string `xml:"PowerOnSpeedOverride"`
	PowerOffWhenLastLogicalPartitionIsShutdown       bool   `xml:"PowerOffWhenLastLogicalPartitionIsShutdown"`
	CurrentManufacturingDefaultConfigurationSource   string `xml:"CurrentManufacturingDefaultConfigurationSource"`
	PendingManufacturingDefaultConfigurationSource   string `xml:"PendingManufacturingDefaultConfigurationSource"`
	PendingPowerOnLogicalPartitionStartPolicy        string `xml:"PendingPowerOnLogicalPartitionStartPolicy"`
	PowerOnSource                                    string `xml:"PowerOnSource"`
}

// SystemCapabilities represents data structure
type SystemCapabilities struct {
	ActiveLogicalPartitionMobilityCapable                 bool   `xml:"ActiveLogicalPartitionMobilityCapable"`
	ActiveLogicalPartitionSharedIdeProcessorsCapable      bool   `xml:"ActiveLogicalPartitionSharedIdeProcessorsCapable"`
	ActiveMemoryDeduplicationCapable                      bool   `xml:"ActiveMemoryDeduplicationCapable"`
	ActiveMemoryExpansionCapable                          bool   `xml:"ActiveMemoryExpansionCapable"`
	ActiveMemoryMirroringCapable                          bool   `xml:"ActiveMemoryMirroringCapable"`
	ActiveMemorySharingCapable                            bool   `xml:"ActiveMemorySharingCapable"`
	AddressBroadcastPolicyCapable                         bool   `xml:"AddressBroadcastPolicyCapable"`
	AIXCapable                                            string `xml:"AIXCapable"`
	AutorecoveryPowerOnCapable                            bool   `xml:"AutorecoveryPowerOnCapable"`
	BarrierSynchronizationRegisterCapable                 bool   `xml:"BarrierSynchronizationRegisterCapable"`
	CapacityOnDemandMemoryCapable                         bool   `xml:"CapacityOnDemandMemoryCapable"`
	CapacityOnDemandProcessorCapable                      bool   `xml:"CapacityOnDemandProcessorCapable"`
	CapacityOnDemandOnOffProcessorCapable                 bool   `xml:"CapacityOnDemandOnOffProcessorCapable"`
	CapacityOnDemandOnOffMemoryCapable                    bool   `xml:"CapacityOnDemandOnOffMemoryCapable"`
	CapacityOnDemandTrialProcessorCapable                 bool   `xml:"CapacityOnDemandTrialProcessorCapable"`
	CapacityOnDemandTrialMemoryCapable                    bool   `xml:"CapacityOnDemandTrialMemoryCapable"`
	CapacityOnDemandUtilityProcessorCapable               bool   `xml:"CapacityOnDemandUtilityProcessorCapable"`
	CAPICapable                                           bool   `xml:"CAPICapable"`
	CustomLogicalPartitionPlacementCapable                bool   `xml:"CustomLogicalPartitionPlacementCapable"`
	ElectronicErrorReportingCapable                       bool   `xml:"ElectronicErrorReportingCapable"`
	ExternalIntrusionDetectionCapable                     bool   `xml:"ExternalIntrusionDetectionCapable"`
	FirmwarePowerSaverCapable                             bool   `xml:"FirmwarePowerSaverCapable"`
	HardwareDiscoveryCapable                              bool   `xml:"HardwareDiscoveryCapable"`
	HardwareMemoryCompressionCapable                      bool   `xml:"HardwareMemoryCompressionCapable"`
	HardwareMemoryEncryptionCapable                       bool   `xml:"HardwareMemoryEncryptionCapable"`
	HardwarePowerSaverCapable                             bool   `xml:"HardwarePowerSaverCapable"`
	HostChannelAdapterCapable                             bool   `xml:"HostChannelAdapterCapable"`
	HugePageMemoryCapable                                 bool   `xml:"HugePageMemoryCapable"`
	HugePageMemoryOverrideCapable                         bool   `xml:"HugePageMemoryOverrideCapable"`
	IBMiCapable                                           bool   `xml:"IBMiCapable"`
	IBMiLogicalPartitionMobilityCapable                   bool   `xml:"IBMiLogicalPartitionMobilityCapable"`
	IBMiLogicalPartitionSuspendCapable                    bool   `xml:"IBMiLogicalPartitionSuspendCapable"`
	IBMiNetworkInstallCapable                             bool   `xml:"IBMiNetworkInstallCapable"`
	IBMiRestrictedIOModeCapable                           bool   `xml:"IBMiRestrictedIOModeCapable"`
	IBMiNetworkInstallVlanCapable                         bool   `xml:"IBMiNetworkInstallVlanCapable"`
	InactiveLogicalPartitionMobilityCapable               bool   `xml:"InactiveLogicalPartitionMobilityCapable"`
	IntelligentPlatformManagementInterfaceCapable         bool   `xml:"IntelligentPlatformManagementInterfaceCapable"`
	LinuxCapable                                          bool   `xml:"LinuxCapable"`
	LogicalHostEthernetAdapterCapable                     bool   `xml:"LogicalHostEthernetAdapterCapable"`
	LogicalPartitionAffinityGroupCapable                  bool   `xml:"LogicalPartitionAffinityGroupCapable"`
	LogicalPartitionAvailabilityPriorityCapable           bool   `xml:"LogicalPartitionAvailabilityPriorityCapable"`
	LogicalPartitionEnergyManagementCapable               bool   `xml:"LogicalPartitionEnergyManagementCapable"`
	LogicalPartitionProcessorCompatibilityModeCapable     bool   `xml:"LogicalPartitionProcessorCompatibilityModeCapable"`
	LogicalPartitionRemoteRestartCapable                  bool   `xml:"LogicalPartitionRemoteRestartCapable"`
	LogicalPartitionSuspendCapable                        bool   `xml:"LogicalPartitionSuspendCapable"`
	MemoryMirroringCapable                                bool   `xml:"MemoryMirroringCapable"`
	MicroLogicalPartitionCapable                          bool   `xml:"MicroLogicalPartitionCapable"`
	PowerVMLogicalPartitionSimplifiedRemoteRestartCapable bool   `xml:"PowerVMLogicalPartitionSimplifiedRemoteRestartCapable"`
	RedundantErrorPathReportingCapable                    bool   `xml:"RedundantErrorPathReportingCapable"`
	RemoteRestartToggleCapable                            bool   `xml:"RemoteRestartToggleCapable"`
	ServiceProcessorConcurrentMaintenanceCapable          bool   `xml:"ServiceProcessorConcurrentMaintenanceCapable"`
	ServiceProcessorFailoverCapable                       bool   `xml:"ServiceProcessorFailoverCapable"`
	ServiceProcessorAutonomicIPLCapable                   bool   `xml:"ServiceProcessorAutonomicIPLCapable"`
	SharedEthernetFailoverCapable                         bool   `xml:"SharedEthernetFailoverCapable"`
	SharedProcessorPoolCapable                            bool   `xml:"SharedProcessorPoolCapable"`
	SRIOVCapable                                          bool   `xml:"SRIOVCapable"`
	SRIOVRoCECapable                                      bool   `xml:"SRIOVRoCECapable"`
	SwitchNetworkInterfaceMessagePassingCapable           bool   `xml:"SwitchNetworkInterfaceMessagePassingCapable"`
	SystemPartitionProcessorLimitCapable                  bool   `xml:"SystemPartitionProcessorLimitCapable"`
	Telnet5250ApplicationCapable                          bool   `xml:"Telnet5250ApplicationCapable"`
	TurboCoreCapable                                      bool   `xml:"TurboCoreCapable"`
	VirtualEthernetAdapterDynamicLogicalPartitionCapable  bool   `xml:"VirtualEthernetAdapterDynamicLogicalPartitionCapable"`
	VirtualEthernetQualityOfServiceCapable                bool   `xml:"VirtualEthernetQualityOfServiceCapable"`
	VirtualFiberChannelCapable                            bool   `xml:"VirtualFiberChannelCapable"`
	VirtualIOServerCapable                                bool   `xml:"VirtualIOServerCapable"`
	VirtualizationEngineTechnologiesActivationCapable     bool   `xml:"VirtualizationEngineTechnologiesActivationCapable"`
	VirtualServerNetworkingPhase2Capable                  bool   `xml:"VirtualServerNetworkingPhase2Capable"`
	VirtualSwitchCapable                                  bool   `xml:"VirtualSwitchCapable"`
	VirtualTrustedPlatformModuleCapable                   bool   `xml:"VirtualTrustedPlatformModuleCapable"`
	VirtualTrustedPlatformModule20Capable                 bool   `xml:"VirtualTrustedPlatformModule20Capable"`
	VLANStatisticsCapable                                 bool   `xml:"VLANStatisticsCapable"`
	VirtualEthernetCustomMACAddressCapable                bool   `xml:"VirtualEthernetCustomMACAddressCapable"`
	ManagementVLANForControlChannelCapable                bool   `xml:"ManagementVLANForControlChannelCapable"`
	VirtualNICDedicatedSRIOVCapable                       bool   `xml:"VirtualNICDedicatedSRIOVCapable"`
	VirtualNICSharedSRIOVCapable                          bool   `xml:"VirtualNICSharedSRIOVCapable"`
	DynamicPlatformOptimizationCapable                    bool   `xml:"DynamicPlatformOptimizationCapable"`
	VirtualNICFailOverCapable                             bool   `xml:"VirtualNICFailOverCapable"`
	AdvancedBootListSupportCapable                        bool   `xml:"AdvancedBootListSupportCapable"`
	DynamicSimplifiedRemoteRestartToggleCapable           bool   `xml:"DynamicSimplifiedRemoteRestartToggleCapable"`
	IBMiNativeIOCapable                                   bool   `xml:"IBMiNativeIOCapable"`
	CustomPhysicalPageTableRatioCapable                   bool   `xml:"CustomPhysicalPageTableRatioCapable"`
	HardwareAcceleratorCapable                            bool   `xml:"HardwareAcceleratorCapable"`
	PlatformMemoryMirroringCapableIfLicensed              bool   `xml:"PlatformMemoryMirroringCapableIfLicensed"`
	PlatformMemoryMirroringLicensed                       bool   `xml:"PlatformMemoryMirroringLicensed"`
	PlatformMemoryMirroringCapabilityKnown                bool   `xml:"PlatformMemoryMirroringCapabilityKnown"`
	PartitionSecureBootCapable                            bool   `xml:"PartitionSecureBootCapable"`
	DedicatedProcessorPartitionCapable                    bool   `xml:"DedicatedProcessorPartitionCapable"`
	PersistentMemoryCapable                               bool   `xml:"PersistentMemoryCapable"`
	SRIOVMigrationCapable                                 bool   `xml:"SRIOVMigrationCapable"`
	VirtualSerialNumberCapable                            bool   `xml:"VirtualSerialNumberCapable"`
	CoDVSNCoExistCapable                                  bool   `xml:"CoDVSNCoExistCapable"`
	PartitionKeyStoreCapable                              bool   `xml:"PartitionKeyStoreCapable"`
	IBMiHardwareAcceleratorCapable                        bool   `xml:"IBMiHardwareAcceleratorCapable"`
	AIXUpdateAccessKeyCapable                             bool   `xml:"AIXUpdateAccessKeyCapable"`
	NewPowerSavingModesNamesSupported                     bool   `xml:"NewPowerSavingModesNamesSupported"`
	IBMiNetworkInstalliSCSICapable                        bool   `xml:"IBMiNetworkInstalliSCSICapable"`
	PartitionDynamicKeySecureBootCapable                  bool   `xml:"PartitionDynamicKeySecureBootCapable"`
	SRIOVAdapterConfigOptionsCapable                      bool   `xml:"SRIOVAdapterConfigOptionsCapable"`
	KvmOnPowerVMCapable                                   bool   `xml:"KvmOnPowerVMCapable"`
	MultipathNVMeCapable                                  bool   `xml:"MultipathNVMeCapable"`
}

// SystemIOConfiguration represents configuration settings
type SystemIOConfiguration struct {
	AvailableWWPNs string                     `xml:"AvailableWWPNs"`
	MaximumIOPools string                     `xml:"MaximumIOPools"`
	WWPNPrefix     string                     `xml:"WWPNPrefix"`
	IOAdapters     []IOAdapterXML             `xml:"IOAdapters>IOAdapterChoice>IOAdapter"`
	IOBuses        []IOBusXML                 `xml:"IOBuses>IOBus"`
	VirtualNetwork SystemVirtualNetworkConfig `xml:"AssociatedSystemVirtualNetwork"`
	SRIOVAdapters  []SRIOVAdapter             `xml:"SRIOVAdapters>IOAdapterChoice>SRIOVAdapter"`
}

// SystemVirtualNetworkConfig represents configuration settings
type SystemVirtualNetworkConfig struct {
	VirtualEthernetAdapterMACAddressPrefix string    `xml:"VirtualEthernetAdapterMACAddressPrefix"`
	NetworkBridges                         []LinkXML `xml:"NetworkBridges>link"`
	VirtualNetworks                        []LinkXML `xml:"VirtualNetworks>link"`
	VirtualSwitches                        []LinkXML `xml:"VirtualSwitches>link"`
}

// IOAdapterXML represents an adapter configuration
type IOAdapterXML struct {
	// NEW: We need this to extract the AtomID for mapping!
	UUID                                string `xml:"Metadata>Atom>AtomID"`
	AdapterID                           string `xml:"AdapterID"`
	Description                         string `xml:"Description"`
	DeviceName                          string `xml:"DeviceName"`
	DynamicReconfigurationConnectorName string `xml:"DynamicReconfigurationConnectorName"`
	PhysicalLocation                    string `xml:"PhysicalLocation"`
	UniqueDeviceID                      string `xml:"UniqueDeviceID"`
	LogicalPartitionAssignmentCapable   bool   `xml:"LogicalPartitionAssignmentCapable"`
	DynamicPartitionAssignmentCapable   bool   `xml:"DynamicPartitionAssignmentCapable"`
}

// IOBusXML represents data structure
type IOBusXML struct {
	IOBusID                   string      `xml:"IOBusID"`
	BackplanePhysicalLocation string      `xml:"BackplanePhysicalLocation"`
	ConnectorIndex            string      `xml:"BusDynamicReconfigurationConnectorIndex"`
	ConnectorName             string      `xml:"BusDynamicReconfigurationConnectorName"`
	IOSlots                   []IOSlotXML `xml:"IOSlots>IOSlot"`
}

// IOSlotXML represents data structure
type IOSlotXML struct {
	BusGroupingRequired    bool         `xml:"BusGroupingRequired"`
	Description            string       `xml:"Description"`
	FeatureCodes           []string     `xml:"FeatureCodes"`
	IOUnitPhysicalLocation string       `xml:"IOUnitPhysicalLocation"`
	PartitionID            int          `xml:"PartitionID"`
	PartitionName          string       `xml:"PartitionName"`
	PartitionUUID          string       `xml:"PartitionUUID"`
	PartitionType          string       `xml:"PartitionType"`
	PCAdapterID            string       `xml:"PCAdapterID"`
	PCIClass               string       `xml:"PCIClass"`
	PCIDeviceID            string       `xml:"PCIDeviceID"`
	PCISubsystemDeviceID   string       `xml:"PCISubsystemDeviceID"`
	PCIManufacturerID      string       `xml:"PCIManufacturerID"`
	PCIRevisionID          string       `xml:"PCIRevisionID"`
	PCIVendorID            string       `xml:"PCIVendorID"`
	PCISubsystemVendorID   string       `xml:"PCISubsystemVendorID"`
	RelatedIBMiIOSlot      IBMiIOSlot   `xml:"RelatedIBMiIOSlot"`
	RelatedIOAdapter       IOAdapterXML `xml:"RelatedIOAdapter>IOAdapter"`
	ConnectorIndex         string       `xml:"SlotDynamicReconfigurationConnectorIndex"`
	ConnectorName          string       `xml:"SlotDynamicReconfigurationConnectorName"`
	PhysicalLocationCode   string       `xml:"SlotPhysicalLocationCode"`
	SRIOVCapableDevice     bool         `xml:"SRIOVCapableDevice"`
	SRIOVCapableSlot       bool         `xml:"SRIOVCapableSlot"`
	SRIOVLogicalPortsLimit int          `xml:"SRIOVLogicalPortsLimit"`
}

// IBMiIOSlot represents data structure
type IBMiIOSlot struct {
	AlternateLoadSourceAttached    bool   `xml:"AlternateLoadSourceAttached"`
	ConsoleCapable                 bool   `xml:"ConsoleCapable"`
	DirectOperationsConsoleCapable bool   `xml:"DirectOperationsConsoleCapable"`
	IOP                            bool   `xml:"IOP"`
	IOPInfoStale                   bool   `xml:"IOPInfoStale"`
	IOPoolID                       string `xml:"IOPoolID"`
	LANConsoleCapable              bool   `xml:"LANConsoleCapable"`
	LoadSourceAttached             bool   `xml:"LoadSourceAttached"`
	LoadSourceCapable              bool   `xml:"LoadSourceCapable"`
	OperationsConsoleAttached      bool   `xml:"OperationsConsoleAttached"`
	OperationsConsoleCapable       bool   `xml:"OperationsConsoleCapable"`
}

// SystemMemoryConfiguration represents configuration settings
type SystemMemoryConfiguration struct {
	AllowedHardwarePageTableRations                            []string `xml:"AllowedHardwarePageTableRations"`
	AllowedMemoryDeduplicationTableRatios                      string   `xml:"AllowedMemoryDeduplicationTableRatios"`
	AllowedMemoryRegionSize                                    string   `xml:"AllowedMemoryRegionSize"`
	ConfigurableHugePages                                      string   `xml:"ConfigurableHugePages"`
	ConfigurableSystemMemory                                   float64  `xml:"ConfigurableSystemMemory"`
	ConfiguredMirroredMemory                                   float64  `xml:"ConfiguredMirroredMemory"`
	CurrentAvailableHugePages                                  string   `xml:"CurrentAvailableHugePages"`
	CurrentAvailableMirroredMemory                             float64  `xml:"CurrentAvailableMirroredMemory"`
	CurrentAvailableSystemMemory                               float64  `xml:"CurrentAvailableSystemMemory"`
	CurrentLogicalMemoryBlockSize                              string   `xml:"CurrentLogicalMemoryBlockSize"`
	CurrentMemoryMirroringMode                                 string   `xml:"CurrentMemoryMirroringMode"`
	CurrentMirroredMemory                                      float64  `xml:"CurrentMirroredMemory"`
	DeconfiguredSystemMemory                                   float64  `xml:"DeconfiguredSystemMemory"`
	DefaultHardwarePageTableRatio                              string   `xml:"DefaultHardwarePageTableRatio"`
	DefaultHardwarePagingTableRatioForDedicatedMemoryPartition string   `xml:"DefaultHardwarePagingTableRatioForDedicatedMemoryPartition"`
	DefaultMemoryDeduplicationTableRatio                       string   `xml:"DefaultMemoryDeduplicationTableRatio"`
	HugePageCount                                              string   `xml:"HugePageCount"`
	HugePageSize                                               string   `xml:"HugePageSize"`
	InstalledSystemMemory                                      float64  `xml:"InstalledSystemMemory"`
	MaximumHugePages                                           string   `xml:"MaximumHugePages"`
	MaximumMemoryPoolCount                                     string   `xml:"MaximumMemoryPoolCount"`
	MaximumMirroredMemoryDefragmented                          float64  `xml:"MaximumMirroredMemoryDefragmented"`
	MaximumPagingVirtualIOServersPerSharedMemoryPool           string   `xml:"MaximumPagingVirtualIOServersPerSharedMemoryPool"`
	MemoryDefragmentationState                                 string   `xml:"MemoryDefragmentationState"`
	MemoryMirroringState                                       string   `xml:"MemoryMirroringState"`
	MemoryRegionSize                                           string   `xml:"MemoryRegionSize"`
	MemoryUsedByHypervisor                                     float64  `xml:"MemoryUsedByHypervisor"`
	MirrorableMemoryWithDefragmentation                        float64  `xml:"MirrorableMemoryWithDefragmentation"`
	MirrorableMemoryWithoutDefragmentation                     float64  `xml:"MirrorableMemoryWithoutDefragmentation"`
	MirroredMemoryUsedByHypervisor                             float64  `xml:"MirroredMemoryUsedByHypervisor"`
	PendingAvailableHugePages                                  string   `xml:"PendingAvailableHugePages"`
	PendingAvailableSystemMemory                               float64  `xml:"PendingAvailableSystemMemory"`
	PendingLogicalMemoryBlockSize                              string   `xml:"PendingLogicalMemoryBlockSize"`
	PendingMemoryMirroringMode                                 string   `xml:"PendingMemoryMirroringMode"`
	PendingMemoryRegionSize                                    string   `xml:"PendingMemoryRegionSize"`
	RequestedHugePages                                         string   `xml:"RequestedHugePages"`
	TemporaryMemoryForLogicalPartitionMobilityInUse            bool     `xml:"TemporaryMemoryForLogicalPartitionMobilityInUse"`
	DefaultPhysicalPageTableRatio                              string   `xml:"DefaultPhysicalPageTableRatio"`
	AllowedPhysicalPageTableRatios                             []string `xml:"AllowedPhysicalPageTableRatios"`
	PermanentSystemMemory                                      float64  `xml:"PermanentSystemMemory"`
	CurrentAssignedMemoryToPartitions                          float64  `xml:"CurrentAssignedMemoryToPartitions"`
	PendingLogicalMemoryRegionSize                             string   `xml:"PendingLogicalMemoryRegionSize"`
}

// SystemProcessorConfiguration represents configuration settings
type SystemProcessorConfiguration struct {
	ConfigurableSystemProcessorUnits                           float64   `xml:"ConfigurableSystemProcessorUnits"`
	CurrentAvailableSystemProcessorUnits                       float64   `xml:"CurrentAvailableSystemProcessorUnits"`
	CurrentMaximumProcessorsPerAIXOrLinuxPartition             string    `xml:"CurrentMaximumProcessorsPerAIXOrLinuxPartition"`
	CurrentMaximumProcessorsPerIBMiPartition                   string    `xml:"CurrentMaximumProcessorsPerIBMiPartition"`
	CurrentMaximumAllowedProcessorsPerPartition                string    `xml:"CurrentMaximumAllowedProcessorsPerPartition"`
	CurrentMaximumProcessorsPerVirtualIOServerPartition        string    `xml:"CurrentMaximumProcessorsPerVirtualIOServerPartition"`
	CurrentMaximumVirtualProcessorsPerAIXOrLinuxPartition      string    `xml:"CurrentMaximumVirtualProcessorsPerAIXOrLinuxPartition"`
	CurrentMaximumVirtualProcessorsPerIBMiPartition            string    `xml:"CurrentMaximumVirtualProcessorsPerIBMiPartition"`
	CurrentMaximumVirtualProcessorsPerVirtualIOServerPartition string    `xml:"CurrentMaximumVirtualProcessorsPerVirtualIOServerPartition"`
	DeconfiguredSystemProcessorUnits                           float64   `xml:"DeconfiguredSystemProcessorUnits"`
	InstalledSystemProcessorUnits                              float64   `xml:"InstalledSystemProcessorUnits"`
	MaximumProcessorUnitsPerIBMiPartition                      float64   `xml:"MaximumProcessorUnitsPerIBMiPartition"`
	MaximumAllowedVirtualProcessorsPerPartition                string    `xml:"MaximumAllowedVirtualProcessorsPerPartition"`
	MinimumProcessorUnitsPerVirtualProcessor                   float64   `xml:"MinimumProcessorUnitsPerVirtualProcessor"`
	NumberOfAllOSProcessorUnits                                float64   `xml:"NumberOfAllOSProcessorUnits"`
	NumberOfLinuxOnlyProcessorUnits                            float64   `xml:"NumberOfLinuxOnlyProcessorUnits"`
	NumberOfLinuxOrVIOSOnlyProcessorUnits                      float64   `xml:"NumberOfLinuxOrVIOSOnlyProcessorUnits"`
	NumberOfVirtualIOServerProcessorUnits                      float64   `xml:"NumberOfVirtualIOServerProcessorUnits"`
	PendingAvailableSystemProcessorUnits                       float64   `xml:"PendingAvailableSystemProcessorUnits"`
	SharedProcessorPoolCount                                   string    `xml:"SharedProcessorPoolCount"`
	SupportedPartitionProcessorCompatibilityModes              []string  `xml:"SupportedPartitionProcessorCompatibilityModes"`
	TemporaryProcessorUnitsForLogicalPartitionMobilityInUse    bool      `xml:"TemporaryProcessorUnitsForLogicalPartitionMobilityInUse"`
	SharedProcessorPools                                       []LinkXML `xml:"SharedProcessorPool>link"`
	PermanentSystemProcessors                                  float64   `xml:"PermanentSystemProcessors"`
}

// SystemSecurityConfiguration represents configuration settings
type SystemSecurityConfiguration struct {
	VirtualTrustedPlatformModuleKeyLength                  string `xml:"VirtualTrustedPlatformModuleKeyLength"`
	VirtualTrustedPlatformModuleKeyStatus                  string `xml:"VirtualTrustedPlatformModuleKeyStatus"`
	VirtualTrustedPlatformModuleVersion                    string `xml:"VirtualTrustedPlatformModuleVersion"`
	MaximumSupportedVirtualTrustedPlatformModulePartitions string `xml:"MaximumSupportedVirtualTrustedPlatformModulePartitions"`
	AvailableVirtualTrustedPlatformModulePartitions        string `xml:"AvailableVirtualTrustedPlatformModulePartitions"`
}

// SystemMigrationInformation represents information details
type SystemMigrationInformation struct {
	AllowInactiveSourceStorageVios                   string `xml:"AllowInactiveSourceStorageVios"`
	MaximumInactiveMigrations                        string `xml:"MaximumInactiveMigrations"`
	MaximumActiveMigrations                          string `xml:"MaximumActiveMigrations"`
	NumberOfInactiveMigrationsInProgress             string `xml:"NumberOfInactiveMigrationsInProgress"`
	NumberOfActiveMigrationsInProgress               string `xml:"NumberOfActiveMigrationsInProgress"`
	MaximumFirmwareActiveMigrations                  string `xml:"MaximumFirmwareActiveMigrations"`
	LogicalPartitionAffinityCheckCapable             bool   `xml:"LogicalPartitionAffinityCheckCapable"`
	MaximumFirmwareInactiveMigrations                string `xml:"MaximumFirmwareInactiveMigrations"`
	InactiveLogicalPartitionMigrationCapable         bool   `xml:"InactiveLogicalPartitionMigrationCapable"`
	ActiveLogicalPartitionMigrationCapable           bool   `xml:"ActiveLogicalPartitionMigrationCapable"`
	IBMiLogicalPartitionMigrationCapable             bool   `xml:"IBMiLogicalPartitionMigrationCapable"`
	LogicalPartitionPersistentMemoryMigrationCapable bool   `xml:"LogicalPartitionPersistentMemoryMigrationCapable"`
	LogicalPartitionRedundantMspsMigrationCapable    bool   `xml:"LogicalPartitionRedundantMspsMigrationCapable"`
	LogicalPartitionVSwitchChangeMigrationCapable    bool   `xml:"LogicalPartitionVSwitchChangeMigrationCapable"`
	LogicalPartitionSRIOVMigrationCapable            bool   `xml:"LogicalPartitionSRIOVMigrationCapable"`
	NPIVValidationPolicy                             string `xml:"NPIVValidationPolicy"`
	InactiveProfileMigrationPolicy                   string `xml:"InactiveProfileMigrationPolicy"`
}

// SystemEnergyManagementConfiguration represents configuration settings
type SystemEnergyManagementConfiguration struct {
	CurrentPowerSavingMode        string                     `xml:"CurrentPowerSavingMode"`
	RequiredPowerSavingMode       string                     `xml:"RequiredPowerSavingMode"`
	SupportedPowerSavingModeTypes []string                   `xml:"SupportedPowerSavingModeTypes"`
	IdlePowerSaverMode            bool                       `xml:"IdlePowerSaverMode"`
	DynamicPowerSavingTunables    PowerSavingTunablesDynamic `xml:"DynamicPowerSavingTunables"`
	IdlePowerSavingTunables       PowerSavingTunablesIdle    `xml:"IdlePowerSavingTunables"`
}

// PowerSavingTunablesDynamic represents data structure
type PowerSavingTunablesDynamic struct {
	UtilizationThresholdForIncreasingFrequency                 string `xml:"UtilizationThresholdForIncreasingFrequency"`
	UtilizationThresholdForDecreasingFrequency                 string `xml:"UtilizationThresholdForDecreasingFrequency"`
	SamplesForComputingUtilzationStatistics                    string `xml:"SamplesForComputingUtilzationStatistics"`
	StepSizeForGoingUpInFrequency                              string `xml:"StepSizeForGoingUpInFrequency"`
	StepSizeForGoingDownInFrequency                            string `xml:"StepSizeForGoingDownInFrequency"`
	DeltaPercentageForDeterminingActiveCores                   string `xml:"DeltaPercentageForDeterminingActiveCores"`
	UtilizationThresholdToDetermineActiveCoresWithSlack        string `xml:"UtilizationThresholdToDetermineActiveCoresWithSlack"`
	CoreFrequencyDeltaState                                    bool   `xml:"CoreFrequencyDeltaState"`
	CoreMaximumDeltaFrequency                                  string `xml:"CoreMaximumDeltaFrequency"`
	MinimumUtilizationThresholdForIncreasingFrequency          string `xml:"MinimumUtilizationThresholdForIncreasingFrequency"`
	MinimumUtilizationThresholdForDecreasingFrequency          string `xml:"MinimumUtilizationThresholdForDecreasingFrequency"`
	MinimumSamplesForComputingUtilzationStatistics             string `xml:"MinimumSamplesForComputingUtilzationStatistics"`
	MinimumStepSizeForGoingUpInFrequency                       string `xml:"MinimumStepSizeForGoingUpInFrequency"`
	MinimumStepSizeForGoingDownInFrequency                     string `xml:"MinimumStepSizeForGoingDownInFrequency"`
	MinimumDeltaPercentageForDeterminingActiveCores            string `xml:"MinimumDeltaPercentageForDeterminingActiveCores"`
	MinimumUtilizationThresholdToDetermineActiveCoresWithSlack string `xml:"MinimumUtilizationThresholdToDetermineActiveCoresWithSlack"`
	MinimumCoreMaximumDeltaFrequency                           string `xml:"MinimumCoreMaximumDeltaFrequency"`
	MaximumUtilizationThresholdForIncreasingFrequency          string `xml:"MaximumUtilizationThresholdForIncreasingFrequency"`
	MaximumUtilizationThresholdForDecreasingFrequency          string `xml:"MaximumUtilizationThresholdForDecreasingFrequency"`
	MaximumSamplesForComputingUtilzationStatistics             string `xml:"MaximumSamplesForComputingUtilzationStatistics"`
	MaximumStepSizeForGoingUpInFrequency                       string `xml:"MaximumStepSizeForGoingUpInFrequency"`
	MaximumStepSizeForGoingDownInFrequency                     string `xml:"MaximumStepSizeForGoingDownInFrequency"`
	MaximumDeltaPercentageForDeterminingActiveCores            string `xml:"MaximumDeltaPercentageForDeterminingActiveCores"`
	MaximumUtilizationThresholdToDetermineActiveCoresWithSlack string `xml:"MaximumUtilizationThresholdToDetermineActiveCoresWithSlack"`
	MaximumCoreMaximumDeltaFrequency                           string `xml:"MaximumCoreMaximumDeltaFrequency"`
}

// PowerSavingTunablesIdle represents data structure
type PowerSavingTunablesIdle struct {
	DelayTimeToEnterIdlePower                   string `xml:"DelayTimeToEnterIdlePower"`
	DelayTimeToExitIdlePower                    string `xml:"DelayTimeToExitIdlePower"`
	UtilizationThresholdToEnterIdlePower        string `xml:"UtilizationThresholdToEnterIdlePower"`
	UtilizationThresholdToExitIdlePower         string `xml:"UtilizationThresholdToExitIdlePower"`
	MinimumDelayTimeToEnterIdlePower            string `xml:"MinimumDelayTimeToEnterIdlePower"`
	MinimumDelayTimeToExitIdlePower             string `xml:"MinimumDelayTimeToExitIdlePower"`
	MinimumUtilizationThresholdToEnterIdlePower string `xml:"MinimumUtilizationThresholdToEnterIdlePower"`
	MinimumUtilizationThresholdToExitIdlePower  string `xml:"MinimumUtilizationThresholdToExitIdlePower"`
	MaximumDelayTimeToEnterIdlePower            string `xml:"MaximumDelayTimeToEnterIdlePower"`
	MaximumDelayTimeToExitIdlePower             string `xml:"MaximumDelayTimeToExitIdlePower"`
	MaximumUtilizationThresholdToEnterIdlePower string `xml:"MaximumUtilizationThresholdToEnterIdlePower"`
	MaximumUtilizationThresholdToExitIdlePower  string `xml:"MaximumUtilizationThresholdToExitIdlePower"`
}

// HardwareAcceleratorType represents data structure
type HardwareAcceleratorType struct {
	Type                string `xml:"Type"`
	TotalQoS            string `xml:"TotalQoS"`
	CurrentAvailableQoS string `xml:"CurrentAvailableQoS"`
	PendingAvailableQoS string `xml:"PendingAvailableQoS"`
}

// SystemPersistentMemoryConfiguration represents configuration settings
type SystemPersistentMemoryConfiguration struct {
	MaximumPersistentMemoryVolumes         string `xml:"MaximumPersistentMemoryVolumes"`
	CurrentPersistentMemoryVolumes         string `xml:"CurrentPersistentMemoryVolumes"`
	MaximumAixLinuxPersistentMemoryVolumes string `xml:"MaximumAixLinuxPersistentMemoryVolumes"`
	MaximumOS400PersistentMemoryVolumes    string `xml:"MaximumOS400PersistentMemoryVolumes"`
	MaximumVIOSPersistentMemoryVolumes     string `xml:"MaximumVIOSPersistentMemoryVolumes"`
	DramPersistentMemoryVolumeBlockSize    string `xml:"DramPersistentMemoryVolumeBlockSize"`
	DramPersistentMemoryVolumesSize        string `xml:"DramPersistentMemoryVolumesSize"`
	DramPersistentMemoryVolumesCurrentSize string `xml:"DramPersistentMemoryVolumesCurrentSize"`
	SupportedPersistentMemoryDeviceTypes   string `xml:"SupportedPersistentMemoryDeviceTypes"`
}

// =====================================================================
// SR-IOV AND VIRTUAL NIC STRUCTURES
// =====================================================================

// SRIOVPhysicalPort represents an individual ethernet port on an SR-IOV adapter.
// These only appear in the XML if the SR-IOV adapter is configured in "Shared" mode.
type SRIOVPhysicalPort struct {
	LocationCode        string `xml:"LocationCode"`
	ConfiguredLinkSpeed string `xml:"ConfiguredLinkSpeed"`
	HardwareLinkSpeed   string `xml:"HardwareLinkSpeed"`
	PortCapacity        string `xml:"PortCapacity"`
	PortID              string `xml:"PortID"`
}

// SRIOVAdapter represents a Single Root I/O Virtualization adapter on the Managed System.
type SRIOVAdapter struct {
	AdapterID    string `xml:"AdapterID"`
	LocationCode string `xml:"PhysicalLocation"`
	AdapterMode  string `xml:"AdapterMode"`
	AdapterState string `xml:"AdapterState"`
	IsFunctional string `xml:"IsFunctional"`
	Description  string `xml:"Description"`

	PhysicalPorts []SRIOVPhysicalPort `xml:"SRIOVEthernetPhysicalPorts>SRIOVEthernetPhysicalPort"`
}

// SRIOVLogicalPortRequest represents the payload required to provision an SR-IOV port.
// ⚠️ WARNING: IBM HMC schema strictly enforces the exact sequence defined in their XSD.
// The order below matches the precise schema sequence expected by the HMC.
type SRIOVLogicalPortRequest struct {
	XMLName       xml.Name `xml:"SRIOVEthernetLogicalPort"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	XMLNS         string   `xml:"xmlns,attr"`

	// --- STRICT SCHEMA SEQUENCE ORDER ---
	AdapterID                    string  `xml:"AdapterID"`
	IsPromiscous                 *string `xml:"IsPromiscous,omitempty"` // Note IBM's spelling
	ConfiguredCapacity           *string `xml:"ConfiguredCapacity,omitempty"`
	PhysicalPortID               string  `xml:"PhysicalPortID"`
	PortVLANID                   *string `xml:"PortVLANID,omitempty"`
	AllowedMACAddresses          *string `xml:"AllowedMACAddresses,omitempty"`
	IEEE8021QAllowablePriorities *string `xml:"IEEE8021QAllowablePriorities,omitempty"`
	AllowedVLANs                 *string `xml:"AllowedVLANs,omitempty"`
}

// SRIOVPortCreateOptions holds the advanced parameters for provisioning an SR-IOV Logical Port.
// These map directly to the advanced configuration GUI in the HMC.
type SRIOVPortCreateOptions struct {
	Capacity               string // e.g., "2.0%"
	PortVLANID             string // Default is usually "0"
	IsPromiscuous          bool   // true = On, false = Off
	AllowedMACAddresses    string // "ALL", "NONE", or specify (e.g., "1607984ec203")
	AllowedVLANs           string // "ALL", "NONE", or specify (e.g., "1,2,100")
	Allowed8021QPriorities string // "ALL", "NONE", or specify (e.g., "0,1,2")
}

// =====================================================================
// EXHAUSTIVE LOGICAL PARTITION XML STRUCTURES
// =====================================================================

// LogicalPartitionDetailed represents the complete XML payload of a Logical Partition
// LogicalPartitionDetailed represents the complete XML payload of a Logical Partition
type LogicalPartitionDetailed struct {
	XMLName       xml.Name `xml:"LogicalPartition"`
	SchemaVersion string   `xml:"schemaVersion,attr"` // Captures the version attribute

	// --- Fixed Metadata Mapping ---
	MetadataID      string `xml:"Metadata>Atom>AtomID"`
	MetadataCreated string `xml:"Metadata>Atom>AtomCreated"`

	// --- Fixed Uptime Mapping (To capture the 'group' attribute) ---
	Uptime struct {
		Value float64 `xml:",chardata"`
		Group string  `xml:"group,attr"`
	} `xml:"Uptime"`

	AllowPerformanceDataCollection        bool                       `xml:"AllowPerformanceDataCollection"`
	AssociatedPartitionProfile            LinkXML                    `xml:"AssociatedPartitionProfile"`
	DefaultProfileName                    string                     `xml:"DefaultProfileName"`
	AvailabilityPriority                  int                        `xml:"AvailabilityPriority"`
	CurrentProcessorCompatibilityMode     string                     `xml:"CurrentProcessorCompatibilityMode"`
	CurrentProfileSync                    string                     `xml:"CurrentProfileSync"`
	IsBootable                            bool                       `xml:"IsBootable"`
	IsConnectionMonitoringEnabled         bool                       `xml:"IsConnectionMonitoringEnabled"`
	IsOperationInProgress                 bool                       `xml:"IsOperationInProgress"`
	IsRedundantErrorPathReportingEnabled  bool                       `xml:"IsRedundantErrorPathReportingEnabled"`
	IsTimeReferencePartition              bool                       `xml:"IsTimeReferencePartition"`
	IsVirtualServiceAttentionLEDOn        bool                       `xml:"IsVirtualServiceAttentionLEDOn"`
	IsVirtualTrustedPlatformModuleEnabled bool                       `xml:"IsVirtualTrustedPlatformModuleEnabled"`
	KeylockPosition                       string                     `xml:"KeylockPosition"`
	LogicalSerialNumber                   string                     `xml:"LogicalSerialNumber"`
	OperatingSystemVersion                string                     `xml:"OperatingSystemVersion"`
	PartitionCapabilities                 LparCapabilities           `xml:"PartitionCapabilities"`
	PartitionID                           int                        `xml:"PartitionID"`
	PartitionIOConfiguration              LparIOConfiguration        `xml:"PartitionIOConfiguration"`
	PartitionMemoryConfiguration          LparMemoryConfiguration    `xml:"PartitionMemoryConfiguration"`
	PartitionName                         string                     `xml:"PartitionName"`
	PartitionProcessorConfiguration       LparProcessorConfiguration `xml:"PartitionProcessorConfiguration"`
	PartitionProfiles                     []LinkXML                  `xml:"PartitionProfiles>link"`
	PartitionState                        string                     `xml:"PartitionState"`
	PartitionType                         string                     `xml:"PartitionType"`
	PartitionUUID                         string                     `xml:"PartitionUUID"`
	PendingProcessorCompatibilityMode     string                     `xml:"PendingProcessorCompatibilityMode"`
	ProcessorPool                         LinkXML                    `xml:"ProcessorPool"`

	// --- Fixed: Changed to float64 to prevent unmarshal errors with scientific notation ---
	ProgressPartitionDataRemaining float64 `xml:"ProgressPartitionDataRemaining"`
	ProgressPartitionDataTotal     float64 `xml:"ProgressPartitionDataTotal"`

	ResourceMonitoringControlState string  `xml:"ResourceMonitoringControlState"`
	ResourceMonitoringIPAddress    string  `xml:"ResourceMonitoringIPAddress"`
	AssociatedManagedSystem        LinkXML `xml:"AssociatedManagedSystem"`

	// --- VIRTUAL ADAPTER ARRAYS ---
	ClientNetworkAdapters             []LinkXML `xml:"ClientNetworkAdapters>link"`
	HostEthernetAdapterLogicalPorts   []LinkXML `xml:"HostEthernetAdapterLogicalPorts>link"`
	VirtualFibreChannelClientAdapters []LinkXML `xml:"VirtualFibreChannelClientAdapters>link"`
	VirtualSCSIClientAdapters         []LinkXML `xml:"VirtualSCSIClientAdapters>link"`
	AssociatedTrunkAdapters           []LinkXML `xml:"AssociatedTrunkAdapters>link"`
	DedicatedVirtualNICs              []LinkXML `xml:"DedicatedVirtualNICs>link"`
	SRIOVEthernetLogicalPorts         []LinkXML `xml:"SRIOVEthernetLogicalPorts>link"`
	SRIOVRoCELogicalPorts             []LinkXML `xml:"SRIOVRoCELogicalPorts>link"`
	SharedVirtualNICs                 []LinkXML `xml:"SharedVirtualNICs>link"`
	// ------------------------------

	MACAddressPrefix                   string                            `xml:"MACAddressPrefix"`
	IsServicePartition                 bool                              `xml:"IsServicePartition"`
	PowerVMManagementCapable           bool                              `xml:"PowerVMManagementCapable"`
	ReferenceCode                      string                            `xml:"ReferenceCode"`
	AssignAllResources                 bool                              `xml:"AssignAllResources"`
	HardwareAcceleratorQoS             HardwareAcceleratorQoSXML         `xml:"HardwareAcceleratorQoS"`
	LastActivatedProfile               string                            `xml:"LastActivatedProfile"`
	HasPhysicalIO                      bool                              `xml:"HasPhysicalIO"`
	OperatingSystemType                string                            `xml:"OperatingSystemType"`
	PendingSecureBoot                  int                               `xml:"PendingSecureBoot"`
	CurrentSecureBoot                  int                               `xml:"CurrentSecureBoot"`
	KeyStoreSize                       int                               `xml:"KeyStoreSize"`
	BootMode                           string                            `xml:"BootMode"`
	SystemName                         string                            `xml:"SystemName"`
	PowerOnWithHypervisor              bool                              `xml:"PowerOnWithHypervisor"`
	PersistentMemoryConfiguration      LparPersistentMemoryConfiguration `xml:"AssociatedPersistentMemoryConfiguration"`
	MigrationStorageViosDataStatus     string                            `xml:"MigrationStorageViosDataStatus"`
	MigrationStorageViosDataTimestamp  string                            `xml:"MigrationStorageViosDataTimestamp"`
	RemoteRestartCapable               bool                              `xml:"RemoteRestartCapable"`
	SimplifiedRemoteRestartCapable     bool                              `xml:"SimplifiedRemoteRestartCapable"`
	HasDedicatedProcessorsForMigration bool                              `xml:"HasDedicatedProcessorsForMigration"`
	SuspendCapable                     bool                              `xml:"SuspendCapable"`
	MigrationDisable                   bool                              `xml:"MigrationDisable"`
	MigrationState                     string                            `xml:"MigrationState"`
	RemoteRestartState                 string                            `xml:"RemoteRestartState"`
	BootListInformation                LparBootListInformation           `xml:"BootListInformation"`
	VirtualSerialNumber                string                            `xml:"VirtualSerialNumber"`
	KvmCapable                         bool                              `xml:"KvmCapable"`
}

// HardwareAcceleratorQoSXML represents data structure
type HardwareAcceleratorQoSXML struct {
	// Captures the parent element
}

// LparCapabilities represents data structure
type LparCapabilities struct {
	DynamicLogicalPartitionIOCapable                        bool `xml:"DynamicLogicalPartitionIOCapable"`
	DynamicLogicalPartitionMemoryCapable                    bool `xml:"DynamicLogicalPartitionMemoryCapable"`
	DynamicLogicalPartitionVIOSCapable                      bool `xml:"DynamicLogicalPartitionVIOSCapable"`
	DynamicLogicalPartitionProcessorCapable                 bool `xml:"DynamicLogicalPartitionProcessorCapable"`
	InternalAndExternalIntrusionDetectionCapable            bool `xml:"InternalAndExternalIntrusionDetectionCapable"`
	ResourceMonitoringControlOperatingSystemShutdownCapable bool `xml:"ResourceMonitoringControlOperatingSystemShutdownCapable"`
}

// LparIOConfiguration represents configuration settings
type LparIOConfiguration struct {
	MaximumVirtualIOSlots        int       `xml:"MaximumVirtualIOSlots"`
	CurrentMaximumVirtualIOSlots int       `xml:"CurrentMaximumVirtualIOSlots"`
	ProfileIOSlots               []LinkXML `xml:"ProfileIOSlots>link"`
}

// LparMemoryConfiguration represents configuration settings
type LparMemoryConfiguration struct {
	ActiveMemoryExpansionEnabled          bool    `xml:"ActiveMemoryExpansionEnabled"`
	ActiveMemorySharingEnabled            bool    `xml:"ActiveMemorySharingEnabled"`
	DesiredHugePageCount                  int     `xml:"DesiredHugePageCount"`
	DesiredMemory                         float64 `xml:"DesiredMemory"`
	ExpansionFactor                       float64 `xml:"ExpansionFactor"`
	HardwarePageTableRatio                string  `xml:"HardwarePageTableRatio"`
	MaximumHugePageCount                  int     `xml:"MaximumHugePageCount"`
	MaximumMemory                         float64 `xml:"MaximumMemory"`
	MinimumHugePageCount                  int     `xml:"MinimumHugePageCount"`
	MinimumMemory                         float64 `xml:"MinimumMemory"`
	CurrentExpansionFactor                float64 `xml:"CurrentExpansionFactor"`
	CurrentHardwarePageTableRatio         string  `xml:"CurrentHardwarePageTableRatio"`
	CurrentHugePageCount                  int     `xml:"CurrentHugePageCount"`
	CurrentMaximumHugePageCount           int     `xml:"CurrentMaximumHugePageCount"`
	CurrentMaximumMemory                  float64 `xml:"CurrentMaximumMemory"`
	CurrentMemory                         float64 `xml:"CurrentMemory"`
	CurrentMinimumHugePageCount           int     `xml:"CurrentMinimumHugePageCount"`
	CurrentMinimumMemory                  float64 `xml:"CurrentMinimumMemory"`
	MemoryExpansionHardwareAccessEnabled  bool    `xml:"MemoryExpansionHardwareAccessEnabled"`
	MemoryEncryptionHardwareAccessEnabled bool    `xml:"MemoryEncryptionHardwareAccessEnabled"`
	MemoryExpansionEnabled                bool    `xml:"MemoryExpansionEnabled"`
	RedundantErrorPathReportingEnabled    bool    `xml:"RedundantErrorPathReportingEnabled"`
	RuntimeHugePageCount                  int     `xml:"RuntimeHugePageCount"`
	RuntimeMemory                         float64 `xml:"RuntimeMemory"`
	RuntimeMinimumMemory                  float64 `xml:"RuntimeMinimumMemory"`
	SharedMemoryEnabled                   bool    `xml:"SharedMemoryEnabled"`
	PhysicalPageTableRatio                string  `xml:"PhysicalPageTableRatio"`
}

// LparProcessorConfiguration represents configuration settings
type LparProcessorConfiguration struct {
	HasDedicatedProcessors                 bool                                    `xml:"HasDedicatedProcessors"`
	SharedProcessorConfiguration           LparSharedProcessorConfiguration        `xml:"SharedProcessorConfiguration"`
	SharingMode                            string                                  `xml:"SharingMode"`
	CurrentHasDedicatedProcessors          bool                                    `xml:"CurrentHasDedicatedProcessors"`
	CurrentSharingMode                     string                                  `xml:"CurrentSharingMode"`
	CurrentDedicatedProcessorConfiguration LparDedicatedProcessorConfiguration     `xml:"CurrentDedicatedProcessorConfiguration"`
	RuntimeHasDedicatedProcessors          bool                                    `xml:"RuntimeHasDedicatedProcessors"`
	RuntimeSharingMode                     string                                  `xml:"RuntimeSharingMode"`
	CurrentSharedProcessorConfiguration    LparCurrentSharedProcessorConfiguration `xml:"CurrentSharedProcessorConfiguration"`
}

// LparSharedProcessorConfiguration represents configuration settings
type LparSharedProcessorConfiguration struct {
	DesiredProcessingUnits   float64 `xml:"DesiredProcessingUnits"`
	DesiredVirtualProcessors int     `xml:"DesiredVirtualProcessors"`
	MaximumProcessingUnits   float64 `xml:"MaximumProcessingUnits"`
	MaximumVirtualProcessors int     `xml:"MaximumVirtualProcessors"`
	MinimumProcessingUnits   float64 `xml:"MinimumProcessingUnits"`
	MinimumVirtualProcessors int     `xml:"MinimumVirtualProcessors"`
	SharedProcessorPoolID    int     `xml:"SharedProcessorPoolID"`
	UncappedWeight           int     `xml:"UncappedWeight"`
}

// LparCurrentSharedProcessorConfiguration represents configuration settings
type LparCurrentSharedProcessorConfiguration struct {
	AllocatedVirtualProcessors      float64 `xml:"AllocatedVirtualProcessors"` // Changed to float64
	CurrentMaximumProcessingUnits   float64 `xml:"CurrentMaximumProcessingUnits"`
	CurrentMinimumProcessingUnits   float64 `xml:"CurrentMinimumProcessingUnits"`
	CurrentProcessingUnits          float64 `xml:"CurrentProcessingUnits"`
	CurrentSharedProcessorPoolID    int     `xml:"CurrentSharedProcessorPoolID"`
	CurrentUncappedWeight           float64 `xml:"CurrentUncappedWeight"` // Changed to float64
	CurrentMinimumVirtualProcessors int     `xml:"CurrentMinimumVirtualProcessors"`
	CurrentMaximumVirtualProcessors int     `xml:"CurrentMaximumVirtualProcessors"`
	RuntimeProcessingUnits          float64 `xml:"RuntimeProcessingUnits"`
	RuntimeUncappedWeight           float64 `xml:"RuntimeUncappedWeight"` // Changed to float64
}

// LparDedicatedProcessorConfiguration represents configuration settings
type LparDedicatedProcessorConfiguration struct {
	CurrentProcessors        float64 `xml:"CurrentProcessors"`
	DesiredProcessors        float64 `xml:"DesiredProcessors"`
	MaximumProcessors        float64 `xml:"MaximumProcessors"`
	MinimumProcessors        float64 `xml:"MinimumProcessors"`
	CurrentMaximumProcessors int     `xml:"CurrentMaximumProcessors"`
	CurrentMinimumProcessors int     `xml:"CurrentMinimumProcessors"`
	RunProcessors            int     `xml:"RunProcessors"`
}

// LparPersistentMemoryConfiguration represents configuration settings
type LparPersistentMemoryConfiguration struct {
	MaximumPersistentMemoryVolumes     int `xml:"MaximumPersistentMemoryVolumes"`
	CurrentPersistentMemoryVolumes     int `xml:"CurrentPersistentMemoryVolumes"`
	MaximumDramPersistentMemoryVolumes int `xml:"MaximumDramPersistentMemoryVolumes"`
	CurrentDramPersistentMemoryVolumes int `xml:"CurrentDramPersistentMemoryVolumes"`
}

// LparBootListInformation represents information details
type LparBootListInformation struct {
	PendingBootString      string `xml:"PendingBootString"`
	BootDeviceList         string `xml:"BootDeviceList"`
	ShadowBootDeviceList   string `xml:"ShadowBootDeviceList"`
	LastBootedDeviceString string `xml:"LastBootedDeviceString"`
}

// VirtualFibreChannelClientAdapter represents a vFC adapter on an LPAR
type VirtualFibreChannelClientAdapter struct {
	XMLName                             xml.Name `xml:"VirtualFibreChannelClientAdapter"`
	UUID                                string   `xml:"Metadata>Atom>AtomID"`
	DynamicReconfigurationConnectorName string   `xml:"DynamicReconfigurationConnectorName"`
	LocationCode                        string   `xml:"LocationCode"`
	LocalPartitionID                    string   `xml:"LocalPartitionID"`
	RequiredAdapter                     string   `xml:"RequiredAdapter"`
	VariedOn                            string   `xml:"VariedOn"`
	VirtualSlotNumber                   string   `xml:"VirtualSlotNumber"`
	AdapterType                         string   `xml:"AdapterType"`
	ConnectingPartitionID               string   `xml:"ConnectingPartitionID"`
	ConnectingVirtualSlotNumber         string   `xml:"ConnectingVirtualSlotNumber"`
	WWPNs                               string   `xml:"WWPNs"`
}

// VirtualNICDedicated represents a Virtual NIC explicitly backed by an SR-IOV Logical Port.
type VirtualNICDedicated struct {
	XMLName           xml.Name `xml:"VirtualNICDedicated"`
	UUID              string   `xml:"Metadata>Atom>AtomID"`
	LocationCode      string   `xml:"LocationCode"`
	MACAddress        string   `xml:"MACAddress"`
	VirtualSlotNumber string   `xml:"VirtualSlotNumber"`
	Capacity          string   `xml:"Capacity"` // Percentage of backing port bandwidth
	VariedOn          string   `xml:"VariedOn"`

	// This links the vNIC to the actual SR-IOV Logical Port on the Managed System
	BackingLogicalPort LinkXML `xml:"BackingLogicalPort>link"`
}

// SRIOVLogicalPort represents an SR-IOV Ethernet Logical Port assigned to an LPAR.
type SRIOVLogicalPort struct {
	UUID               string `xml:"Metadata>Atom>AtomID"`
	ConfigurationID    string `xml:"ConfigurationID"`
	AdapterID          string `xml:"AdapterID"`
	LogicalPortID      string `xml:"LogicalPortID"`
	PhysicalPortID     string `xml:"PhysicalPortID"`
	ConfiguredCapacity string `xml:"ConfiguredCapacity"` // Usually a percentage, e.g., "2.0"
	PortVLANID         string `xml:"PortVLANID"`
	LocationCode       string `xml:"LocationCode"`
	IsPromiscuous      bool   `xml:"IsPromiscous"` // Note IBM's spelling "IsPromiscous" without the 'u'
	IsFunctional       bool   `xml:"IsFunctional"`
}

// =====================================================================
// JOB RESPONSE STRUCTURES
// =====================================================================

// =====================================================================
// PARTITION TEMPLATE OPERATION RESULTS
// =====================================================================

// TransformResult represents the result of transforming a partition template
type TransformResult struct {
	JobID           string
	Status          string
	TransformedUUID string
	ErrorMessage    string
	Success         bool
}

// TemplateValidationResult represents the result of checking/validating a partition template
type TemplateValidationResult struct {
	IsValid      bool
	Errors       []string
	Warnings     []string
	JobID        string
	Status       string
	ErrorMessage string
}

// PowerOnOptions contains parameters for powering on a logical partition
type PowerOnOptions struct {
	// Required
	ProfileUUID string // Partition profile UUID

	// Boot Configuration
	Keylock  string // "normal" or "manual" (default: "normal")
	BootMode string // "norm", "dd", "ds", "of", "sms", "netboot" (default: "norm")

	// IBM i Specific
	IIPLSource string // IPL source: "a", "b", "c", "d" (for OS400)
	OSType     string // "AIX/Linux", "OS400", "Virtual IO Server"

	// Network Boot Parameters
	LocationCode string // Physical location code (e.g., "U78DA.ND0.WZS00AR-P1-C7-T1")
	ClientIP     string // Client IP address for netboot
	ServerIP     string // Server/NIM IP address for netboot
	Gateway      string // Gateway address for netboot
	Netmask      string // Subnet mask for netboot

	// Advanced Network Boot (extensible for future IBM parameters)
	ConnectionSpeed string // "auto", "1", "10", "100", "1000"
	DuplexMode      string // "auto", "half", "full"
}

// AggregatedMetricsOptions holds the query parameters for retrieving PCM metrics.
type AggregatedMetricsOptions struct {
	StartTS     time.Time // Mandatory: The API returns metrics after this time
	EndTS       time.Time // Optional: The API returns metrics on or before this time
	NoOfSamples int       // Optional: Number of aggregated metrics to return (> 0)
}

// PcmMetricsSnapshot represents a single metrics snapshot link from the PCM Atom feed.
type PcmMetricsSnapshot struct {
	Updated   string // End timestamp of the Aggregated JSON
	Published string // Start timestamp of the Aggregated JSON
	Category  string // Metrics category (e.g., "LogicalPartition")
	Frequency string // Metrics aggregation frequency in seconds
	JSONLink  string // Direct URL to download the application/json metrics payload
}

// =====================================================================
// PCM (PERFORMANCE & CAPACITY MONITORING) JSON STRUCTURES
// =====================================================================

// PcmMetricsPayload represents the payload structure
type PcmMetricsPayload struct {
	SystemUtil SystemUtil `json:"systemUtil"`
}

// SystemUtil represents utility information
type SystemUtil struct {
	UtilInfo    UtilInfo     `json:"utilInfo"`
	UtilSamples []UtilSample `json:"utilSamples"`
}

// UtilInfo represents information details
type UtilInfo struct {
	Version          string   `json:"version"`
	MetricType       string   `json:"metricType"`
	Frequency        int      `json:"frequency"`
	StartTimeStamp   string   `json:"startTimeStamp"`
	EndTimeStamp     string   `json:"endTimeStamp"`
	MTMS             string   `json:"mtms"`
	Name             string   `json:"name"`
	UUID             string   `json:"uuid"`
	MetricArrayOrder []string `json:"metricArrayOrder"` // e.g., ["AVG", "MIN", "MAX"]
}

// UtilSample represents utility information
type UtilSample struct {
	SampleType string     `json:"sampleType"`
	SampleInfo SampleInfo `json:"sampleInfo"`
	LparsUtil  []LparUtil `json:"lparsUtil"`
}

// SampleInfo represents information details
type SampleInfo struct {
	TimeStamp              string      `json:"timeStamp"`
	NumOfSamplesAggregated int         `json:"numOfSamplesAggregated"`
	Status                 int         `json:"status"`
	ErrorInfo              []ErrorInfo `json:"errorInfo"`
}

// ErrorInfo represents information details
type ErrorInfo struct {
	ErrID          string `json:"errId"`
	ErrMsg         string `json:"errMsg"`
	UUID           string `json:"uuid"`
	ReportedBy     string `json:"reportedBy"`
	OccurrenceCount int    `json:"occurenceCount"`
}

// LparUtil represents utility information
type LparUtil struct {
	ID            int              `json:"id"`
	UUID          string           `json:"uuid"`
	Name          string           `json:"name"`
	State         string           `json:"state"`
	Type          string           `json:"type"`
	OSType        string           `json:"osType"`
	AffinityScore float64          `json:"affinityScore"`
	Memory        MemoryMetrics    `json:"memory"`
	Processor     ProcessorMetrics `json:"processor"`
	Network       NetworkMetrics   `json:"network"`
	Storage       StorageMetrics   `json:"storage"`
}

// MemoryMetrics represents metrics data
type MemoryMetrics struct {
	PoolID               int       `json:"poolId"`
	Weight               int       `json:"weight"`
	LogicalMem           []float64 `json:"logicalMem"`
	BackedPhysicalMem    []float64 `json:"backedPhysicalMem"`
	TotalIOMem           []float64 `json:"totalIOMem"`
	MappedIOMem          []float64 `json:"mappedIOMem"`
	VirtualPersistentMem []float64 `json:"virtualPersistentMem"`
}

// ProcessorMetrics represents metrics data
type ProcessorMetrics struct {
	PoolID                      int       `json:"poolId"`
	Weight                      int       `json:"weight"`
	Mode                        string    `json:"mode"`
	MaxVirtualProcessors        []float64 `json:"maxVirtualProcessors"`
	CurrentVirtualProcessors    []float64 `json:"currentVirtualProcessors"`
	MaxProcUnits                []float64 `json:"maxProcUnits"`
	EntitledProcUnits           []float64 `json:"entitledProcUnits"`
	UtilizedProcUnitsDeductIdle []float64 `json:"utilizedProcUnitsDeductIdle"`
	UtilizedProcUnits           []float64 `json:"utilizedProcUnits"`
	UtilizedCappedProcUnits     []float64 `json:"utilizedCappedProcUnits"`
	UtilizedUncappedProcUnits   []float64 `json:"utilizedUncappedProcUnits"`
	IdleProcUnits               []float64 `json:"idleProcUnits"`
	DonatedProcUnits            []float64 `json:"donatedProcUnits"`
	TimeSpentWaitingForDispatch []float64 `json:"timeSpentWaitingForDispatch"`
	TimePerInstructionExecution []float64 `json:"timePerInstructionExecution"`
}

// NetworkMetrics represents metrics data
type NetworkMetrics struct {
	VirtualEthernetAdapters []VirtualEthernetAdapterMetrics `json:"virtualEthernetAdapters"`
	SriovLogicalPorts       []SriovLogicalPortMetrics       `json:"sriovLogicalPorts"`
}

// VirtualEthernetAdapterMetrics represents metrics data
type VirtualEthernetAdapterMetrics struct {
	PhysicalLocation         string    `json:"physicalLocation"`
	VlanID                   int       `json:"vlanId"`
	VswitchID                int       `json:"vswitchId"`
	IsPortVLANID             bool      `json:"isPortVLANID"`
	ViosID                   int       `json:"viosId"`
	SharedEthernetAdapterID  string    `json:"sharedEthernetAdapterId"`
	ReceivedPackets          []float64 `json:"receivedPackets"`
	SentPackets              []float64 `json:"sentPackets"`
	DroppedPackets           []float64 `json:"droppedPackets"`
	SentBytes                []float64 `json:"sentBytes"`
	ReceivedBytes            []float64 `json:"receivedBytes"`
	ReceivedPhysicalPackets  []float64 `json:"receivedPhysicalPackets"`
	SentPhysicalPackets      []float64 `json:"sentPhysicalPackets"`
	DroppedPhysicalPackets   []float64 `json:"droppedPhysicalPackets"`
	SentPhysicalBytes        []float64 `json:"sentPhysicalBytes"`
	ReceivedPhysicalBytes    []float64 `json:"receivedPhysicalBytes"`
	TransferredBytes         []float64 `json:"transferredBytes"`
	TransferredPhysicalBytes []float64 `json:"transferredPhysicalBytes"`
}

// SriovLogicalPortMetrics represents metrics data
type SriovLogicalPortMetrics struct {
	DrcIndex          string    `json:"drcIndex"`
	PhysicalLocation  string    `json:"physicalLocation"`
	PhysicalDrcIndex  string    `json:"physicalDrcIndex"`
	PhysicalPortID    int       `json:"physicalPortId"`
	VnicDeviceMode    string    `json:"vnicDeviceMode"`
	ConfigurationType string    `json:"configurationType"`
	ReceivedPackets   []float64 `json:"receivedPackets"`
	SentPackets       []float64 `json:"sentPackets"`
	DroppedPackets    []float64 `json:"droppedPackets"`
	SentBytes         []float64 `json:"sentBytes"`
	ReceivedBytes     []float64 `json:"receivedBytes"`
	ErrorIn           []float64 `json:"errorIn"`
	ErrorOut          []float64 `json:"errorOut"`
	TransferredBytes  []float64 `json:"transferredBytes"`
}

// StorageMetrics represents metrics data
type StorageMetrics struct {
	GenericVirtualAdapters      []GenericVirtualAdapterMetrics      `json:"genericVirtualAdapters"`
	VirtualFiberChannelAdapters []VirtualFiberChannelAdapterMetrics `json:"virtualFiberChannelAdapters"`
}

// GenericVirtualAdapterMetrics represents metrics data
type GenericVirtualAdapterMetrics struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	ViosID           int       `json:"viosId"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// VirtualFiberChannelAdapterMetrics represents metrics data
type VirtualFiberChannelAdapterMetrics struct {
	ID               string    `json:"id"`
	Wwpn             string    `json:"wwpn"`
	Wwpn2            string    `json:"wwpn2"`
	PhysicalLocation string    `json:"physicalLocation"`
	PhysicalPortWwpn string    `json:"physicalPortWWPN"`
	ViosID           int       `json:"viosId"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	RunningSpeed     []float64 `json:"runningSpeed"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// ManagedSystemMetricsOptions holds the query options for a system-wide metric call.
type ManagedSystemMetricsOptions struct {
	StartTS     time.Time // Mandatory: Metrics after this time
	EndTS       time.Time // Optional: Metrics on or before this time
	NoOfSamples int       // Optional: Number of aggregated metrics to return (> 0)
	Feed        string    // Optional: "bySource" (default) or "byTier"
}

// =====================================================================
// IBM MANAGED SYSTEM PCM METRICS EXHAUSTIVE SPECIFICATION
// =====================================================================

// SysPcmMetricsPayload represents the payload structure
type SysPcmMetricsPayload struct {
	SystemUtil SysSystemUtil `json:"systemUtil"`
}

// SysSystemUtil represents utility information
type SysSystemUtil struct {
	UtilInfo    SysUtilInfo     `json:"utilInfo"`
	UtilSamples []SysUtilSample `json:"utilSamples"`
}

// SysUtilInfo represents information details
type SysUtilInfo struct {
	Version          string   `json:"version"`
	MetricType       string   `json:"metricType"`
	Frequency        int      `json:"frequency"`
	StartTimeStamp   string   `json:"startTimeStamp"`
	EndTimeStamp     string   `json:"endTimeStamp"`
	MTMS             string   `json:"mtms"`
	Name             string   `json:"name"`
	UUID             string   `json:"uuid"`
	MetricArrayOrder []string `json:"metricArrayOrder"` // e.g., ["Avg", "Min", "Max"]
}

// SysUtilSample represents utility information
type SysUtilSample struct {
	SampleType         string             `json:"sampleType"`
	SampleInfo         SysSampleInfo      `json:"sampleInfo"`
	SystemFirmwareUtil SystemFirmwareUtil `json:"systemFirmwareUtil"`
	ServerUtil         ServerUtil         `json:"serverUtil"`
	ViosUtil           []SysViosUtil      `json:"viosUtil"`
}

// SysSampleInfo represents information details
type SysSampleInfo struct {
	TimeStamp              string         `json:"timeStamp"`
	NumOfSamplesAggregated int            `json:"numOfSamplesAggregated"`
	Status                 int            `json:"status"`
	ErrorInfo              []SysErrorInfo `json:"errorInfo"`
}

// SysErrorInfo represents information details
type SysErrorInfo struct {
	ErrID          string `json:"errId"`
	ErrMsg         string `json:"errMsg"`
	UUID           string `json:"uuid"`
	ReportedBy     string `json:"reportedBy"`
	OccurenceCount int    `json:"occurenceCount"`
}

// SystemFirmwareUtil represents utility information
type SystemFirmwareUtil struct {
	UtilizedProcUnits []float64 `json:"utilizedProcUnits"`
	AssignedMem       []float64 `json:"assignedMem"`
}

// ServerUtil represents utility information
type ServerUtil struct {
	Processor             SysServerProcessor    `json:"processor"`
	Memory                SysServerMemory       `json:"memory"`
	PhysicalProcessorPool PhysicalProcessorPool `json:"physicalProcessorPool"`
	SharedMemoryPool      []SharedMemoryPool    `json:"sharedMemoryPool"`
	SharedProcessorPool   []SharedProcessorPool `json:"sharedProcessorPool"`
	Network               SysServerNetwork      `json:"network"`
}

// SysServerProcessor represents processor information
type SysServerProcessor struct {
	TotalProcUnits              []float64 `json:"totalProcUnits"`
	UtilizedProcUnits           []float64 `json:"utilizedProcUnits"`
	UtilizedProcUnitsDeductIdle []float64 `json:"utilizedProcUnitsDeductIdle"`
	AvailableProcUnits          []float64 `json:"availableProcUnits"`
	ConfigurableProcUnits       []float64 `json:"configurableProcUnits"`
}

// SysServerMemory represents memory information
type SysServerMemory struct {
	TotalMem             []float64 `json:"totalMem"`
	AvailableMem         []float64 `json:"availableMem"`
	ConfigurableMem      []float64 `json:"configurableMem"`
	AssignedMemToLpars   []float64 `json:"assignedMemToLpars"`
	VirtualPersistentMem []float64 `json:"virtualPersistentMem"`
}

// PhysicalProcessorPool represents a resource pool
type PhysicalProcessorPool struct {
	AssignedProcUnits   []float64 `json:"assignedProcUnits"`
	UtilizedProcUnits   []float64 `json:"utilizedProcUnits"`
	AvailableProcUnits  []float64 `json:"availableProcUnits"`
	ConfiguredProcUnits []float64 `json:"configuredProcUnits"`
	BorrowedProcUnits   []float64 `json:"borrowedProcUnits"`
}

// SharedMemoryPool represents a resource pool
type SharedMemoryPool struct {
	ID                       int       `json:"id"`
	TotalMem                 []float64 `json:"totalMem"`
	AssignedMemToLpars       []float64 `json:"assignedMemToLpars"`
	TotalIOMem               []float64 `json:"totalIOMem"`
	MappedIOMemToLpars       []float64 `json:"mappedIOMemToLpars"`
	AssignedMemToSysFirmware []float64 `json:"assignedMemToSysFirmware"`
}

// SharedProcessorPool represents a resource pool
type SharedProcessorPool struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	AssignedProcUnits   []float64 `json:"assignedProcUnits"`
	UtilizedProcUnits   []float64 `json:"utilizedProcUnits"`
	AvailableProcUnits  []float64 `json:"availableProcUnits"`
	ConfiguredProcUnits []float64 `json:"configuredProcUnits"`
	BorrowedProcUnits   []float64 `json:"borrowedProcUnits"`
}

// SysServerNetwork represents network information
type SysServerNetwork struct {
	SriovAdapters []SysSriovAdapter `json:"sriovAdapters"`
	HEAdapters    []SysHEAdapter    `json:"HEAdapters"`
}

// SysSriovAdapter represents an adapter configuration
type SysSriovAdapter struct {
	DrcIndex      string                 `json:"drcIndex"`
	PhysicalPorts []SysSriovPhysicalPort `json:"physicalPorts"`
}

// SysSriovPhysicalPort represents a port
type SysSriovPhysicalPort struct {
	ID               int       `json:"id"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	ErrorIn          []float64 `json:"errorIn"`
	ErrorOut         []float64 `json:"errorOut"`
	TransferredBytes []float64 `json:"transferredBytes"`
}

// SysHEAdapter represents an adapter configuration
type SysHEAdapter struct {
	DrcIndex      string              `json:"drcIndex"`
	PhysicalPorts []SysHEPhysicalPort `json:"physicalPorts"`
}

// SysHEPhysicalPort represents a port
type SysHEPhysicalPort struct {
	ID               int       `json:"id"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	TransferredBytes []float64 `json:"transferredBytes"`
}

// --- VIOS TELEMETRY ARRAYS ---

// SysViosUtil represents utility information
type SysViosUtil struct {
	ID            int               `json:"id"`
	UUID          string            `json:"uuid"`
	Name          string            `json:"name"`
	State         string            `json:"state"`
	AffinityScore float64           `json:"affinityScore"`
	Memory        ViosMemoryInfo    `json:"memory"`
	Processor     ViosProcessorInfo `json:"processor"`
	Network       ViosNetworkInfo   `json:"network"`
	Storage       ViosStorageInfo   `json:"storage"`
}

// ViosMemoryInfo represents information details
type ViosMemoryInfo struct {
	AssignedMem          []float64 `json:"assignedMem"`
	UtilizedMem          []float64 `json:"utilizedMem"`
	VirtualPersistentMem []float64 `json:"virtualPersistentMem"`
}

// ViosProcessorInfo represents information details
type ViosProcessorInfo struct {
	PoolID                      int       `json:"poolId"`
	Weight                      int       `json:"weight"`
	Mode                        string    `json:"mode"`
	MaxVirtualProcessors        []float64 `json:"maxVirtualProcessors"`
	CurrentVirtualProcessors    []float64 `json:"currentVirtualProcessors"`
	MaxProcUnits                []float64 `json:"maxProcUnits"`
	EntitledProcUnits           []float64 `json:"entitledProcUnits"`
	UtilizedProcUnits           []float64 `json:"utilizedProcUnits"`
	UtilizedProcUnitsDeductIdle []float64 `json:"utilizedProcUnitsDeductIdle"`
	UtilizedCappedProcUnits     []float64 `json:"utilizedCappedProcUnits"`
	UtilizedUncappedProcUnits   []float64 `json:"utilizedUncappedProcUnits"`
	IdleProcUnits               []float64 `json:"idleProcUnits"`
	DonatedProcUnits            []float64 `json:"donatedProcUnits"`
	TimeSpentWaitingForDispatch []float64 `json:"timeSpentWaitingForDispatch"`
	TimePerInstructionExecution []float64 `json:"timePerInstructionExecution"`
}

// ViosNetworkInfo represents information details
type ViosNetworkInfo struct {
	ClientLpars             []string                     `json:"clientLpars"`
	GenericAdapters         []ViosGenericAdapter         `json:"genericAdapters"`
	SharedAdapters          []ViosSharedAdapter          `json:"sharedAdapters"`
	VirtualEthernetAdapters []ViosVirtualEthernetAdapter `json:"virtualEthernetAdapters"`
	SriovLogicalPorts       []ViosSriovLogicalPort       `json:"sriovLogicalPorts"`
}

// ViosGenericAdapter represents an adapter configuration
type ViosGenericAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	TransferredBytes []float64 `json:"transferredBytes"`
}

// ViosSharedAdapter represents an adapter configuration
type ViosSharedAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	TransferredBytes []float64 `json:"transferredBytes"`
	BridgedAdapters  []string  `json:"bridgedAdapters"`
}

// ViosVirtualEthernetAdapter represents an adapter configuration
type ViosVirtualEthernetAdapter struct {
	PhysicalLocation         string    `json:"physicalLocation"`
	VlanID                   int       `json:"vlanId"`
	VswitchID                int       `json:"vswitchId"`
	IsPortVLANID             bool      `json:"isPortVLANID"`
	ReceivedPackets          []float64 `json:"receivedPackets"`
	SentPackets              []float64 `json:"sentPackets"`
	DroppedPackets           []float64 `json:"droppedPackets"`
	SentBytes                []float64 `json:"sentBytes"`
	ReceivedBytes            []float64 `json:"receivedBytes"`
	ReceivedPhysicalPackets  []float64 `json:"receivedPhysicalPackets"`
	SentPhysicalPackets      []float64 `json:"sentPhysicalPackets"`
	DroppedPhysicalPackets   []float64 `json:"droppedPhysicalPackets"`
	SentPhysicalBytes        []float64 `json:"sentPhysicalBytes"`
	ReceivedPhysicalBytes    []float64 `json:"receivedPhysicalBytes"`
	TransferredBytes         []float64 `json:"transferredBytes"`
	TransferredPhysicalBytes []float64 `json:"transferredPhysicalBytes"`
}

// ViosSriovLogicalPort represents a port
type ViosSriovLogicalPort struct {
	DrcIndex            string    `json:"drcIndex"`
	PhysicalLocation    string    `json:"physicalLocation"`
	PhysicalDrcIndex    string    `json:"physicalDrcIndex"`
	PhysicalPortID      int       `json:"physicalPortId"`
	ClientPartitionUUID string    `json:"clientPartitionUUID"`
	VnicDeviceMode      string    `json:"vnicDeviceMode"`
	ConfigurationType   string    `json:"configurationType"`
	ReceivedPackets     []float64 `json:"receivedPackets"`
	SentPackets         []float64 `json:"sentPackets"`
	DroppedPackets      []float64 `json:"droppedPackets"`
	SentBytes           []float64 `json:"sentBytes"`
	ReceivedBytes       []float64 `json:"receivedBytes"`
	ErrorIn             []float64 `json:"errorIn"`
	ErrorOut            []float64 `json:"errorOut"`
	TransferredBytes    []float64 `json:"transferredBytes"`
}

// ViosStorageInfo represents information details
type ViosStorageInfo struct {
	ClientLpars             []string                     `json:"clientLpars"`
	GenericVirtualAdapters  []ViosGenericVirtualAdapter  `json:"genericVirtualAdapters"`
	GenericPhysicalAdapters []ViosGenericPhysicalAdapter `json:"genericPhysicalAdapters"`
	FiberChannelAdapters    []ViosFiberChannelAdapter    `json:"fiberChannelAdapters"`
	SharedStoragePools      []ViosSharedStoragePool      `json:"sharedStoragePools"`
}

// ViosGenericVirtualAdapter represents an adapter configuration
type ViosGenericVirtualAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// ViosGenericPhysicalAdapter represents an adapter configuration
type ViosGenericPhysicalAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// ViosFiberChannelAdapter represents an adapter configuration
type ViosFiberChannelAdapter struct {
	ID               string    `json:"id"`
	Wwpn             string    `json:"wwpn"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfPorts       int       `json:"numOfPorts"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	RunningSpeed     []float64 `json:"runningSpeed"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// ViosSharedStoragePool represents a resource pool
type ViosSharedStoragePool struct {
	ID               string    `json:"id"`
	TotalSpace       []float64 `json:"totalSpace"`
	UsedSpace        []float64 `json:"usedSpace"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// LtmMetricsOptions holds query intervals for raw Long Term Monitor loops.
type LtmMetricsOptions struct {
	StartTS time.Time // Optional: Fetch snapshots after this window
	EndTS   time.Time // Optional: Fetch snapshots on or before this window
}

// =====================================================================
// LTM POWER HYPERVISOR EXHAUSTIVE STRUCTURE DEFINITION
// =====================================================================

// LtmPhypPayload represents the payload structure
type LtmPhypPayload struct {
	SystemUtil LtmPhypSystemUtil `json:"systemUtil"`
}

// LtmPhypSystemUtil represents utility information
type LtmPhypSystemUtil struct {
	UtilInfo   LtmPhypUtilInfo   `json:"utilInfo"`
	UtilSample LtmPhypUtilSample `json:"utilSample"` // Strict singular wrapper for Raw LTM metrics
}

// LtmPhypUtilInfo represents information details
type LtmPhypUtilInfo struct {
	Version        string `json:"version"`
	MetricType     string `json:"metricType"`
	MonitoringType string `json:"monitoringType"`
	MTMS           string `json:"mtms"`
	Name           string `json:"name"`
}

// LtmPhypUtilSample represents utility information
type LtmPhypUtilSample struct {
	TimeStamp             string                       `json:"timeStamp"`
	Status                int                          `json:"status"`
	ErrorInfo             []LtmPhypErrorInfo           `json:"errorInfo"`
	TimeBasedCycles       float64                      `json:"timeBasedCycles"`
	SystemFirmware        LtmPhypSystemFirmware        `json:"systemFirmware"`
	Processor             LtmPhypSystemProcessor       `json:"processor"`
	Memory                LtmPhypSystemMemory          `json:"memory"`
	SharedMemoryPool      []LtmPhypSharedMemoryPool    `json:"sharedMemoryPool"`
	PhysicalProcessorPool LtmPhypPhysicalProcessorPool `json:"physicalProcessorPool"`
	SharedProcessorPool   []LtmPhypSharedProcessorPool `json:"sharedProcessorPool"`
	Network               LtmPhypSystemNetwork         `json:"network"`
	LparsUtil             []LtmPhypLparUtil            `json:"lparsUtil"`
	ViosUtil              []LtmPhypViosUtil            `json:"viosUtil"`
}

// LtmPhypErrorInfo represents information details
type LtmPhypErrorInfo struct {
	ErrID  string `json:"errId"`
	ErrMsg string `json:"errMsg"`
}

// LtmPhypSystemFirmware represents data structure
type LtmPhypSystemFirmware struct {
	UtilizedProcCycles float64 `json:"utilizedProcCycles"`
	AssignedMem        float64 `json:"assignedMem"`
}

// LtmPhypSystemProcessor represents processor information
type LtmPhypSystemProcessor struct {
	TotalProcUnits        float64 `json:"totalProcUnits"`
	ConfigurableProcUnits float64 `json:"configurableProcUnits"`
	AvailableProcUnits    float64 `json:"availableProcUnits"`
	ProcCyclesPerSecond   float64 `json:"procCyclesPerSecond"`
}

// LtmPhypSystemMemory represents memory information
type LtmPhypSystemMemory struct {
	TotalMem             float64 `json:"totalMem"`
	AvailableMem         float64 `json:"availableMem"`
	ConfigurableMem      float64 `json:"configurableMem"`
	VirtualPersistentMem float64 `json:"virtualPersistentMem"`
}

// LtmPhypSharedMemoryPool represents a resource pool
type LtmPhypSharedMemoryPool struct {
	ID                       int     `json:"id"`
	Name                     string  `json:"name"`
	TotalMem                 float64 `json:"totalMem"`
	AssignedMemToLpars       float64 `json:"assignedMemToLpars"`
	AssignedMemToSysFirmware float64 `json:"assignedMemToSysFirmware"`
	TotalIOMem               float64 `json:"totalIOMem"`
	MappedIOMemToLpars       float64 `json:"mappedIOMemToLpars"`
}

// LtmPhypPhysicalProcessorPool represents a resource pool
type LtmPhypPhysicalProcessorPool struct {
	TotalPoolCycles            float64 `json:"totalPoolCycles"`
	UtilizedPoolCycles         float64 `json:"utilizedPoolCycles"`
	ConfigurablePoolProcUnits  float64 `json:"configurablePoolProcUnits"`
	CurrAvailablePoolProcUnits float64 `json:"currAvailablePoolProcUnits"`
	BorrowedPoolProcUnits      float64 `json:"borrowedPoolProcUnits"`
}

// LtmPhypSharedProcessorPool represents a resource pool
type LtmPhypSharedProcessorPool struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	AssignedProcCycles float64 `json:"assignedProcCycles"`
	UtilizedProcCycles float64 `json:"utilizedProcCycles"`
	MaxProcUnits       float64 `json:"maxProcUnits"`
	BorrowedProcUnits  float64 `json:"borrowedProcUnits"`
}

// LtmPhypSystemNetwork represents network information
type LtmPhypSystemNetwork struct {
	SriovAdapters []LtmPhypSriovAdapter `json:"sriovAdapters"`
	HEAdapters    []LtmPhypHEAdapter    `json:"HEAdapters"`
}

// LtmPhypSriovAdapter represents an adapter configuration
type LtmPhypSriovAdapter struct {
	DrcIndex      string                     `json:"drcIndex"`
	PhysicalPorts []LtmPhypSriovPhysicalPort `json:"physicalPorts"`
}

// LtmPhypSriovPhysicalPort represents a port
type LtmPhypSriovPhysicalPort struct {
	ID                     int     `json:"id"`
	PhysicalLocation       string  `json:"physicalLocation"`
	ReceivedPackets        float64 `json:"receivedPackets"`
	SentPackets            float64 `json:"sentPackets"`
	DroppedSentPackets     float64 `json:"droppedSentPackets"`
	DroppedReceivedPackets float64 `json:"droppedReceivedPackets"`
	SentBytes              float64 `json:"sentBytes"`
	ReceivedBytes          float64 `json:"receivedBytes"`
	ErrorIn                float64 `json:"errorIn"`
	ErrorOut               float64 `json:"errorOut"`
}

// LtmPhypHEAdapter represents an adapter configuration
type LtmPhypHEAdapter struct {
	DrcIndex         string                  `json:"drcIndex"`
	PhysicalLocation string                  `json:"physicalLocation"`
	PhysicalPorts    []LtmPhypHEPhysicalPort `json:"physicalPorts"`
}

// LtmPhypHEPhysicalPort represents a port
type LtmPhypHEPhysicalPort struct {
	ID               int     `json:"id"`
	PhysicalLocation string  `json:"physicalLocation"`
	ReceivedPackets  float64 `json:"receivedPackets"`
	SentPackets      float64 `json:"sentPackets"`
	DroppedPackets   float64 `json:"droppedPackets"`
	SentBytes        float64 `json:"sentBytes"`
	ReceivedBytes    float64 `json:"receivedBytes"`
}

// =====================================================================
// LPAR RESOURCE TRACKING STRUCTURES
// =====================================================================

// LtmPhypLparUtil represents utility information
type LtmPhypLparUtil struct {
	ID             int                  `json:"id"`
	UUID           string               `json:"uuid"`
	Type           string               `json:"type"`
	OSType         string               `json:"osType"`
	Name           string               `json:"name"`
	State          string               `json:"state"`
	MigrationState string               `json:"migrationState"`
	AffinityScore  float64              `json:"affinityScore"`
	Memory         LtmPhypLparMemory    `json:"memory"`
	Processor      LtmPhypLparProcessor `json:"processor"`
	Network        LtmPhypLparNetwork   `json:"network"`
	Storage        LtmPhypLparStorage   `json:"Storage"` // Capitalized in schema definition mapping
}

// LtmPhypLparMemory represents memory information
type LtmPhypLparMemory struct {
	PoolID               int     `json:"poolId"`
	Weight               int     `json:"weight"`
	LogicalMem           float64 `json:"logicalMem"`
	BackedPhysicalMem    float64 `json:"backedPhysicalMem"`
	TotalIOMem           float64 `json:"totalIOMem"`
	MappedIOMem          float64 `json:"mappedIOMem"`
	VirtualPersistentMem float64 `json:"virtualPersistentMem"`
}

// LtmPhypLparProcessor represents processor information
type LtmPhypLparProcessor struct {
	PoolID                         int     `json:"poolId"`
	Mode                           string  `json:"mode"`
	MaxVirtualProcessors           float64 `json:"maxVirtualProcessors"`
	CurrentVirtualProcessors       float64 `json:"currentVirtualProcessors"`
	MaxProcUnits                   float64 `json:"maxProcUnits"`
	Weight                         int     `json:"weight"`
	EntitledProcCycles             float64 `json:"entitledProcCycles"`
	EntitledProcUnits              float64 `json:"entitledProcUnits"`
	UtilizedCappedProcCycles       float64 `json:"utilizedCappedProcCycles"`
	UtilizedUnCappedProcCycles     float64 `json:"utilizedUnCappedProcCycles"`
	IdleProcCycles                 float64 `json:"idleProcCycles"`
	DonatedProcCycles              float64 `json:"donatedProcCycles"`
	TimeSpentWaitingForDispatch    float64 `json:"timeSpentWaitingForDispatch"`
	TotalInstructions              float64 `json:"totalInstructions"`
	TotalInstructionsExecutionTime float64 `json:"totalInstructionsExecutionTime"`
}

// LtmPhypLparNetwork represents network information
type LtmPhypLparNetwork struct {
	VirtualEthernetAdapters []LtmPhypVirtualEthernetAdapter `json:"virtualEthernetAdapters"`
	SriovLogicalPorts       []LtmPhypSriovLogicalPort       `json:"sriovLogicalPorts"`
}

// LtmPhypVirtualEthernetAdapter represents an adapter configuration
type LtmPhypVirtualEthernetAdapter struct {
	VlanID                  int     `json:"vlanId"`
	VswitchID               int     `json:"vswitchId"`
	PhysicalLocation        string  `json:"physicalLocation"`
	IsPortVLANID            bool    `json:"isPortVLANID"` // Strictly cased as per JSON format data
	ReceivedPackets         float64 `json:"receivedPackets"`
	SentPackets             float64 `json:"sentPackets"`
	DroppedPackets          float64 `json:"droppedPackets"`
	SentBytes               float64 `json:"sentBytes"`
	ReceivedBytes           float64 `json:"receivedBytes"`
	ReceivedPhysicalPackets float64 `json:"receivedPhysicalPackets"`
	SentPhysicalPackets     float64 `json:"sentPhysicalPackets"`
	DroppedPhysicalPackets  float64 `json:"droppedPhysicalPackets"`
	SentPhysicalBytes       float64 `json:"sentPhysicalBytes"`
	ReceivedPhysicalBytes   float64 `json:"receivedPhysicalBytes"`
}

// LtmPhypSriovLogicalPort represents a port
type LtmPhypSriovLogicalPort struct {
	DrcIndex               string  `json:"drcIndex"`
	PhysicalDrcIndex       string  `json:"physicalDrcIndex"`
	PhysicalPortID         int     `json:"physicalPortId"`
	ClientPartitionUUID    string  `json:"clientPartitionUUID"`
	VnicDeviceMode         string  `json:"vnicDeviceMode"`
	ConfigurationType      string  `json:"configurationType"`
	PhysicalLocation       string  `json:"physicalLocation"`
	ReceivedPackets        float64 `json:"receivedPackets"`
	SentPackets            float64 `json:"sentPackets"`
	DroppedSentPackets     float64 `json:"droppedSentPackets"`
	DroppedReceivedPackets float64 `json:"droppedReceivedPackets"`
	SentBytes              float64 `json:"sentBytes"`
	ReceivedBytes          float64 `json:"receivedBytes"`
	ErrorIn                float64 `json:"errorIn"`
	ErrorOut               float64 `json:"errorOut"`
}

// LtmPhypLparStorage represents storage information
type LtmPhypLparStorage struct {
	VirtualFiberChannelAdapters []LtmPhypVirtualFiberChannelAdapter `json:"virtualFiberChannelAdapters"`
	GenericVirtualAdapters      []LtmPhypGenericVirtualAdapter      `json:"genericVirtualAdapters"`
}

// LtmPhypVirtualFiberChannelAdapter represents an adapter configuration
type LtmPhypVirtualFiberChannelAdapter struct {
	ViosID           int      `json:"viosId"`
	WwpnPair         []string `json:"wwpnPair"` // Maps target explicit array format ["wwpn1", "wwpn2"]
	PhysicalLocation string   `json:"physicalLocation"`
}

// LtmPhypGenericVirtualAdapter represents an adapter configuration
type LtmPhypGenericVirtualAdapter struct {
	PhysicalLocation  string `json:"physicalLocation"`
	ViosID            int    `json:"viosId"`
	ViosAdapterSlotID int    `json:"viosAdapterSlotId"`
}

// =====================================================================
// VIOS HYPERVISOR SUB-TIER PROFILE STRUCTURES
// =====================================================================

// LtmPhypViosUtil represents utility information
type LtmPhypViosUtil struct {
	ID            int                  `json:"id"`
	UUID          string               `json:"uuid"`
	Name          string               `json:"name"`
	State         string               `json:"state"`
	AffinityScore float64              `json:"affinityScore"`
	Memory        LtmPhypViosMemory    `json:"memory"`
	Processor     LtmPhypLparProcessor `json:"processor"` // Shared mapping properties with LPAR compute sets
	Network       LtmPhypLparNetwork   `json:"network"`   // Shared mapping properties with LPAR connectivity components
}

// LtmPhypViosMemory represents memory information
type LtmPhypViosMemory struct {
	AssignedMem          float64 `json:"assignedMem"`
	VirtualPersistentMem float64 `json:"virtualPersistentMem"`
}

// =====================================================================
// LTM VIRTUAL I/O SERVER (VIOS) EXHAUSTIVE STRUCTURE DEFINITION
// =====================================================================

// LtmViosPayload represents the payload structure
type LtmViosPayload struct {
	SystemUtil LtmViosSystemUtil `json:"systemUtil"`
}

// LtmViosSystemUtil represents utility information
type LtmViosSystemUtil struct {
	UtilInfo   LtmViosUtilInfo   `json:"utilInfo"`
	UtilSample LtmViosUtilSample `json:"utilSample"` // Singular wrapper for Raw LTM metrics
}

// LtmViosUtilInfo represents information details
type LtmViosUtilInfo struct {
	Version        string `json:"version"`
	MetricType     string `json:"metricType"`
	MonitoringType string `json:"monitoringType"`
	MTMS           string `json:"mtms"`
}

// LtmViosUtilSample represents utility information
type LtmViosUtilSample struct {
	TimeStamp string             `json:"timeStamp"`
	Status    int                `json:"status"`
	ErrorInfo []LtmViosErrorInfo `json:"errorInfo"`
	ViosUtil  []LtmViosUtilEntry `json:"viosUtil"`
}

// LtmViosErrorInfo represents information details
type LtmViosErrorInfo struct {
	ErrID  string `json:"errId"`
	ErrMsg string `json:"errMsg"`
}

// LtmViosUtilEntry represents utility information
type LtmViosUtilEntry struct {
	ID      string         `json:"id"` // Explicitly defined as a string in VIOS schema
	Name    string         `json:"name"`
	Memory  LtmViosMemory  `json:"memory"`
	Network LtmViosNetwork `json:"network"`
	Storage LtmViosStorage `json:"storage"`
}

// LtmViosMemory represents memory information
type LtmViosMemory struct {
	UtilizedMem float64 `json:"utilizedMem"`
}

// --- VIOS NETWORK TRACKING ---

// LtmViosNetwork represents network information
type LtmViosNetwork struct {
	GenericAdapters []LtmViosGenericAdapter `json:"genericAdapters"`
	SharedAdapters  []LtmViosSharedAdapter  `json:"sharedAdapters"`
}

// LtmViosGenericAdapter represents an adapter configuration
type LtmViosGenericAdapter struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	PhysicalLocation string  `json:"physicalLocation"`
	ReceivedPackets  float64 `json:"receivedPackets"`
	SentPackets      float64 `json:"sentPackets"`
	DroppedPackets   float64 `json:"droppedPackets"`
	SentBytes        float64 `json:"sentBytes"`
	ReceivedBytes    float64 `json:"receivedBytes"`
}

// LtmViosSharedAdapter represents an adapter configuration
type LtmViosSharedAdapter struct {
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	PhysicalLocation string   `json:"physicalLocation"`
	ReceivedPackets  float64  `json:"receivedPackets"`
	SentPackets      float64  `json:"sentPackets"`
	DroppedPackets   float64  `json:"droppedPackets"`
	SentBytes        float64  `json:"sentBytes"`
	ReceivedBytes    float64  `json:"receivedBytes"`
	BridgedAdapters  []string `json:"bridgedAdapters"`
}

// --- VIOS STORAGE TRACKING ---

// LtmViosStorage represents storage information
type LtmViosStorage struct {
	GenericPhysicalAdapters []LtmViosGenericPhysicalAdapter `json:"genericPhysicalAdapters"`
	GenericVirtualAdapters  []LtmViosGenericVirtualAdapter  `json:"genericVirtualAdapters"`
	FiberChannelAdapters    []LtmViosFiberChannelAdapter    `json:"fiberChannelAdapters"`
	SharedStoragePools      []LtmViosSharedStoragePool      `json:"sharedStoragePools"`
}

// LtmViosGenericPhysicalAdapter represents an adapter configuration
type LtmViosGenericPhysicalAdapter struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	PhysicalLocation string  `json:"physicalLocation"`
	NumOfReads       float64 `json:"numOfReads"`
	NumOfWrites      float64 `json:"numOfWrites"`
	ReadBytes        float64 `json:"readBytes"`
	WriteBytes       float64 `json:"writeBytes"`
}

// LtmViosGenericVirtualAdapter represents an adapter configuration
type LtmViosGenericVirtualAdapter struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	PhysicalLocation string  `json:"physicalLocation"`
	NumOfReads       float64 `json:"numOfReads"`
	NumOfWrites      float64 `json:"numOfWrites"`
	ReadBytes        float64 `json:"readBytes"`
	WriteBytes       float64 `json:"writeBytes"`
}

// LtmViosFiberChannelAdapter represents an adapter configuration
type LtmViosFiberChannelAdapter struct {
	ID               string                    `json:"id"`
	Wwpn             string                    `json:"wwpn"`
	PhysicalLocation string                    `json:"physicalLocation"`
	NumOfReads       float64                   `json:"numOfReads"`
	NumOfWrites      float64                   `json:"numOfWrites"`
	ReadBytes        float64                   `json:"readBytes"`
	WriteBytes       float64                   `json:"writeBytes"`
	RunningSpeed     float64                   `json:"runningSpeed"`
	Ports            []LtmViosFiberChannelPort `json:"ports"`
}

// LtmViosFiberChannelPort represents a port
type LtmViosFiberChannelPort struct {
	ID               string  `json:"id"`
	Wwpn             string  `json:"wwpn"`
	NumOfReads       float64 `json:"numOfReads"`
	NumOfWrites      float64 `json:"numOfWrites"`
	ReadBytes        float64 `json:"readBytes"`
	WriteBytes       float64 `json:"writeBytes"`
	RunningSpeed     float64 `json:"runningSpeed"`
	PhysicalLocation string  `json:"physicalLocation"`
}

// LtmViosSharedStoragePool represents a resource pool
type LtmViosSharedStoragePool struct {
	ID          string   `json:"id"`
	PoolDisks   []string `json:"poolDisks"`
	NumOfReads  float64  `json:"numOfReads"`
	NumOfWrites float64  `json:"numOfWrites"`
	TotalSpace  float64  `json:"totalSpace"`
	UsedSpace   float64  `json:"usedSpace"`
	ReadBytes   float64  `json:"readBytes"`
	WriteBytes  float64  `json:"writeBytes"`
}

// =====================================================================
// MANAGED SYSTEM PCM PREFERENCES SPECIFICATION
// =====================================================================

// ManagedSystemPcmPreference maps the HMC XML payload for metrics collection configurations.
type ManagedSystemPcmPreference struct {
	EnergyMonitoringCapable bool `xml:"EnergyMonitoringCapable"` // Read-Only
	LongTermMonitorEnabled  bool `xml:"LongTermMonitorEnabled"`
	AggregationEnabled      bool `xml:"AggregationEnabled"`
	ShortTermMonitorEnabled bool `xml:"ShortTermMonitorEnabled"`
	ComputeLTMEnabled       bool `xml:"ComputeLTMEnabled"`
	EnergyMonitorEnabled    bool `xml:"EnergyMonitorEnabled"`
}

// =====================================================================
// GLOBAL MANAGEMENT CONSOLE PCM PREFERENCES
// =====================================================================

// ManagementConsolePcmPreference represents the global PCM preferences for the entire HMC.
type ManagementConsolePcmPreference struct {
	AggregatedMetricsStorageDuration         int                          `xml:"AggregatedMetricsStorageDuration"`
	MaximumManagedSystemsForLongTermMonitor  int                          `xml:"MaximumManagedSystemsForLongTermMonitor"`
	MaximumManagedSystemsForComputeLTM       int                          `xml:"MaximumManagedSystemsForComputeLTM"`
	MaximumManagedSystemsForAggregation      int                          `xml:"MaximumManagedSystemsForAggregation"`
	MaximumManagedSystemsForShortTermMonitor int                          `xml:"MaximumManagedSystemsForShortTermMonitor"`
	MaximumManagedSystemsForEnergyMonitor    int                          `xml:"MaximumManagedSystemsForEnergyMonitor"`
	ManagedSystemPcmPreferences              []SystemPcmPreferenceElement `xml:"ManagedSystemPcmPreferences>ManagedSystemPcmPreference"`
}

// SystemPcmPreferenceElement represents a single system's PCM state within the global list.
type SystemPcmPreferenceElement struct {
	MetadataID              string `xml:"Metadata>Atom>AtomID"`
	SystemName              string `xml:"SystemName"`
	EnergyMonitoringCapable bool   `xml:"EnergyMonitoringCapable"`
	LongTermMonitorEnabled  bool   `xml:"LongTermMonitorEnabled"`
	AggregationEnabled      bool   `xml:"AggregationEnabled"`
	ShortTermMonitorEnabled bool   `xml:"ShortTermMonitorEnabled"`
	ComputeLTMEnabled       bool   `xml:"ComputeLTMEnabled"`
	EnergyMonitorEnabled    bool   `xml:"EnergyMonitorEnabled"`
}

// LparProcessedMetricsOptions holds the query options for LPAR processed metric calls.
type LparProcessedMetricsOptions struct {
	StartTS     time.Time // Mandatory: Metrics after this time
	EndTS       time.Time // Optional: Metrics on or before this time
	NoOfSamples int       // Optional: Number of processed metrics to return (> 0)
}

// =====================================================================
// LPAR PROCESSED METRICS JSON SPECIFICATION
// =====================================================================

// LparProcessedMetricsPayload represents the payload structure
type LparProcessedMetricsPayload struct {
	SystemUtil LparProcessedSystemUtil `json:"systemUtil"`
}

// LparProcessedSystemUtil represents utility information
type LparProcessedSystemUtil struct {
	UtilInfo    LparProcessedUtilInfo     `json:"utilInfo"`
	UtilSamples []LparProcessedUtilSample `json:"utilSamples"` // Array of 30-sec snapshot wrappers
}

// LparProcessedUtilInfo represents information details
type LparProcessedUtilInfo struct {
	Version          string   `json:"version"`
	MetricType       string   `json:"metricType"`
	Frequency        int      `json:"frequency"` // Expected to be 30
	StartTimeStamp   string   `json:"startTimeStamp"`
	EndTimeStamp     string   `json:"endTimeStamp"`
	MTMS             string   `json:"mtms"`
	Name             string   `json:"name"`
	UUID             string   `json:"uuid"`
	MetricArrayOrder []string `json:"metricArrayOrder"` // Expected to be ["AVG"]
}

// LparProcessedUtilSample represents utility information
type LparProcessedUtilSample struct {
	SampleType string                  `json:"sampleType"` // Expected: "LogicalPartition"
	SampleInfo LparProcessedSampleInfo `json:"sampleInfo"`
	LparsUtil  []LparProcessedLparUtil `json:"lparsUtil"`
}

// LparProcessedSampleInfo represents information details
type LparProcessedSampleInfo struct {
	TimeStamp              string                   `json:"timeStamp"`
	NumOfSamplesAggregated int                      `json:"numOfSamplesAggregated"`
	Status                 int                      `json:"status"`
	ErrorInfo              []LparProcessedErrorInfo `json:"errorInfo"`
}

// LparProcessedErrorInfo represents information details
type LparProcessedErrorInfo struct {
	ErrID          string `json:"errId"`
	ErrMsg         string `json:"errMsg"`
	UUID           string `json:"uuid"`
	ReportedBy     string `json:"reportedBy"`
	OccurenceCount int    `json:"occurenceCount"`
}

// --- CORE LPAR RESOURCE BLOCKS ---

// LparProcessedLparUtil represents utility information
type LparProcessedLparUtil struct {
	ID            int                    `json:"id"`
	UUID          string                 `json:"uuid"`
	Name          string                 `json:"name"`
	State         string                 `json:"state"`
	Type          string                 `json:"type"`
	OSType        string                 `json:"osType"`
	AffinityScore float64                `json:"affinityScore"`
	Memory        LparProcessedMemory    `json:"memory"`
	Processor     LparProcessedProcessor `json:"processor"`
	Network       LparProcessedNetwork   `json:"network"`
	Storage       LparProcessedStorage   `json:"storage"`
}

// LparProcessedMemory represents memory information
type LparProcessedMemory struct {
	PoolID               int       `json:"poolId"`
	Weight               int       `json:"weight"`
	LogicalMem           []float64 `json:"logicalMem"`
	BackedPhysicalMem    []float64 `json:"backedPhysicalMem"`
	TotalIOMem           []float64 `json:"totalIOMem"`
	MappedIOMem          []float64 `json:"mappedIOMem"`
	VirtualPersistentMem []float64 `json:"virtualPersistentMem"`
}

// LparProcessedProcessor represents processor information
type LparProcessedProcessor struct {
	PoolID                      int       `json:"poolId"`
	Weight                      int       `json:"weight"`
	Mode                        string    `json:"mode"`
	MaxVirtualProcessors        []float64 `json:"maxVirtualProcessors"`
	CurrentVirtualProcessors    []float64 `json:"currentVirtualProcessors"`
	MaxProcUnits                []float64 `json:"maxProcUnits"`
	EntitledProcUnits           []float64 `json:"entitledProcUnits"`
	UtilizedProcUnitsDeductIdle []float64 `json:"utilizedProcUnitsDeductIdle"`
	UtilizedProcUnits           []float64 `json:"utilizedProcUnits"`
	UtilizedCappedProcUnits     []float64 `json:"utilizedCappedProcUnits"`
	UtilizedUncappedProcUnits   []float64 `json:"utilizedUncappedProcUnits"`
	IdleProcUnits               []float64 `json:"idleProcUnits"`
	DonatedProcUnits            []float64 `json:"donatedProcUnits"`
	TimeSpentWaitingForDispatch []float64 `json:"timeSpentWaitingForDispatch"`
	TimePerInstructionExecution []float64 `json:"timePerInstructionExecution"`
}

// --- LPAR NETWORK BLOCKS ---

// LparProcessedNetwork represents network information
type LparProcessedNetwork struct {
	VirtualEthernetAdapters []LparProcessedVirtualEthAdapter `json:"virtualEthernetAdapters"`
	SriovLogicalPorts       []LparProcessedSriovLogicalPort  `json:"sriovLogicalPorts"`
}

// LparProcessedVirtualEthAdapter represents an adapter configuration
type LparProcessedVirtualEthAdapter struct {
	PhysicalLocation         string    `json:"physicalLocation"`
	VlanID                   int       `json:"vlanId"`
	VswitchID                int       `json:"vswitchId"`
	IsPortVLANID             bool      `json:"isPortVLANID"`
	ViosID                   int       `json:"viosId"`
	SharedEthernetAdapterID  string    `json:"sharedEthernetAdapterId"`
	ReceivedPackets          []float64 `json:"receivedPackets"`
	SentPackets              []float64 `json:"sentPackets"`
	DroppedPackets           []float64 `json:"droppedPackets"`
	SentBytes                []float64 `json:"sentBytes"`
	ReceivedBytes            []float64 `json:"receivedBytes"`
	ReceivedPhysicalPackets  []float64 `json:"receivedPhysicalPackets"`
	SentPhysicalPackets      []float64 `json:"sentPhysicalPackets"`
	DroppedPhysicalPackets   []float64 `json:"droppedPhysicalPackets"`
	SentPhysicalBytes        []float64 `json:"sentPhysicalBytes"`
	ReceivedPhysicalBytes    []float64 `json:"receivedPhysicalBytes"`
	TransferredBytes         []float64 `json:"transferredBytes"`
	TransferredPhysicalBytes []float64 `json:"transferredPhysicalBytes"`
}

// LparProcessedSriovLogicalPort represents a port
type LparProcessedSriovLogicalPort struct {
	DrcIndex          string    `json:"drcIndex"`
	PhysicalLocation  string    `json:"physicalLocation"`
	PhysicalDrcIndex  string    `json:"physicalDrcIndex"`
	PhysicalPortID    int       `json:"physicalPortId"`
	VnicDeviceMode    string    `json:"vnicDeviceMode"`
	ConfigurationType string    `json:"configurationType"`
	ReceivedPackets   []float64 `json:"receivedPackets"`
	SentPackets       []float64 `json:"sentPackets"`
	DroppedPackets    []float64 `json:"droppedPackets"`
	SentBytes         []float64 `json:"sentBytes"`
	ReceivedBytes     []float64 `json:"receivedBytes"`
	ErrorIn           []float64 `json:"errorIn"`
	ErrorOut          []float64 `json:"errorOut"`
	TransferredBytes  []float64 `json:"transferredBytes"`
}

// --- LPAR STORAGE BLOCKS ---

// LparProcessedStorage represents storage information
type LparProcessedStorage struct {
	GenericVirtualAdapters      []LparProcessedGenericVirtualAdapter `json:"genericVirtualAdapters"`
	VirtualFiberChannelAdapters []LparProcessedVfcAdapter            `json:"virtualFiberChannelAdapters"`
}

// LparProcessedGenericVirtualAdapter represents an adapter configuration
type LparProcessedGenericVirtualAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	ViosID           int       `json:"viosId"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// LparProcessedVfcAdapter represents an adapter configuration
type LparProcessedVfcAdapter struct {
	ID               string    `json:"id"`
	WWPN             string    `json:"wwpn"`
	WWPN2            string    `json:"wwpn2"`
	PhysicalLocation string    `json:"physicalLocation"`
	PhysicalPortWWPN string    `json:"physicalPortWWPN"`
	ViosID           int       `json:"viosId"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	RunningSpeed     []float64 `json:"runningSpeed"` // Represented in GBPS
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// =====================================================================
// MANAGED SYSTEM PROCESSED / AGGREGATED METRICS JSON SPECIFICATION
// =====================================================================

// SysProcessedMetricsPayload represents the payload structure
type SysProcessedMetricsPayload struct {
	SystemUtil SysProcessedSystemUtil `json:"systemUtil"`
}

// SysProcessedSystemUtil represents utility information
type SysProcessedSystemUtil struct {
	UtilInfo    SysProcessedUtilInfo     `json:"utilInfo"`
	UtilSamples []SysProcessedUtilSample `json:"utilSamples"`
}

// SysProcessedUtilInfo represents information details
type SysProcessedUtilInfo struct {
	Version          string   `json:"version"`
	MetricType       string   `json:"metricType"`
	Frequency        int      `json:"frequency"`
	StartTimeStamp   string   `json:"startTimeStamp"`
	EndTimeStamp     string   `json:"endTimeStamp"`
	MTMS             string   `json:"mtms"`
	Name             string   `json:"name"`
	UUID             string   `json:"uuid"`
	MetricArrayOrder []string `json:"metricArrayOrder"`
}

// SysProcessedUtilSample represents utility information
type SysProcessedUtilSample struct {
	SampleType         string                   `json:"sampleType"`
	SampleInfo         SysProcessedSampleInfo   `json:"sampleInfo"`
	SystemFirmwareUtil SysProcessedFirmwareUtil `json:"systemFirmwareUtil"`
	ServerUtil         SysProcessedServerUtil   `json:"serverUtil"`
	ViosUtil           []SysProcessedViosUtil   `json:"viosUtil"`
}

// SysProcessedSampleInfo represents information details
type SysProcessedSampleInfo struct {
	TimeStamp              string                  `json:"timeStamp"`
	NumOfSamplesAggregated int                     `json:"numOfSamplesAggregated"`
	Status                 int                     `json:"status"`
	ErrorInfo              []SysProcessedErrorInfo `json:"errorInfo,omitempty"`
}

// SysProcessedErrorInfo represents information details
type SysProcessedErrorInfo struct {
	ErrID          string `json:"errId"`
	ErrMsg         string `json:"errMsg"`
	UUID           string `json:"uuid"`
	ReportedBy     string `json:"reportedBy"`
	OccurenceCount int    `json:"occurenceCount"`
}

// --- SYSTEM RESOURCES (PHYP / HARDWARE) ---

// SysProcessedFirmwareUtil represents utility information
type SysProcessedFirmwareUtil struct {
	UtilizedProcUnits []float64 `json:"utilizedProcUnits"`
	AssignedMem       []float64 `json:"assignedMem"`
}

// SysProcessedServerUtil represents utility information
type SysProcessedServerUtil struct {
	Processor             SysProcessedServerProc       `json:"processor"`
	Memory                SysProcessedServerMem        `json:"memory"`
	PhysicalProcessorPool SysProcessedPhysProcPool     `json:"physicalProcessorPool"`
	SharedMemoryPool      []SysProcessedSharedMemPool  `json:"sharedMemoryPool"`    // Standard IBM Spec
	SharedProcessorPool   []SysProcessedSharedProcPool `json:"sharedProcessorPool"` // Standard IBM Spec

	ResourceGroup []SysProcessedResourceGroup `json:"resourceGroup"`

	Network SysProcessedServerNetwork `json:"network"`
}

// SysProcessedResourceGroup represents data structure
type SysProcessedResourceGroup struct {
	ID                   int                                 `json:"id"`
	Name                 string                              `json:"name"`
	AssignedProcUnits    []float64                           `json:"assignedProcUnits"`
	UtilizedProcUnits    []float64                           `json:"utilizedProcUnits"`
	AvailableProcUnits   []float64                           `json:"availableProcUnits"`
	ConfiguredProcUnits  []float64                           `json:"configuredProcUnits"`
	BorrowedProcUnits    []float64                           `json:"borrowedProcUnits"`
	SharedProcessorPools []SysProcessedSharedProcPoolWrapper `json:"sharedProcessorPools"`
}

// SysProcessedSharedProcPoolWrapper represents a resource pool
type SysProcessedSharedProcPoolWrapper struct {
	SharedProcessorPools []SysProcessedSharedProcPool `json:"sharedProcessorPools"`
}

// SysProcessedServerProc represents data structure
type SysProcessedServerProc struct {
	TotalProcUnits              []float64 `json:"totalProcUnits"`
	UtilizedProcUnits           []float64 `json:"utilizedProcUnits"`
	UtilizedProcUnitsDeductIdle []float64 `json:"utilizedProcUnitsDeductIdle"`
	AvailableProcUnits          []float64 `json:"availableProcUnits"`
	ConfigurableProcUnits       []float64 `json:"configurableProcUnits"`
}

// SysProcessedServerMem represents data structure
type SysProcessedServerMem struct {
	TotalMem             []float64 `json:"totalMem"`
	AvailableMem         []float64 `json:"availableMem"`
	ConfigurableMem      []float64 `json:"configurableMem"`
	AssignedMemToLpars   []float64 `json:"assignedMemToLpars"`
	VirtualPersistentMem []float64 `json:"virtualPersistentMem"`
}

// SysProcessedPhysProcPool represents a resource pool
type SysProcessedPhysProcPool struct {
	AssignedProcUnits   []float64 `json:"assignedProcUnits"`
	UtilizedProcUnits   []float64 `json:"utilizedProcUnits"`
	AvailableProcUnits  []float64 `json:"availableProcUnits"`
	ConfiguredProcUnits []float64 `json:"configuredProcUnits"`
	BorrowedProcUnits   []float64 `json:"borrowedProcUnits"`
}

// SysProcessedSharedMemPool represents a resource pool
type SysProcessedSharedMemPool struct {
	ID                       int       `json:"id"`
	TotalMem                 []float64 `json:"totalMem"`
	AssignedMemToLpars       []float64 `json:"assignedMemToLpars"`
	TotalIOMem               []float64 `json:"totalIOMem"`
	MappedIOMemToLpars       []float64 `json:"mappedIOMemToLpars"`
	AssignedMemToSysFirmware []float64 `json:"assignedMemToSysFirmware"`
}

// SysProcessedSharedProcPool represents a resource pool
type SysProcessedSharedProcPool struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	ResourceGroupID     int       `json:"resourceGroupId,omitempty"` // Traced from live payload
	AssignedProcUnits   []float64 `json:"assignedProcUnits"`
	UtilizedProcUnits   []float64 `json:"utilizedProcUnits"`
	AvailableProcUnits  []float64 `json:"availableProcUnits"`
	ConfiguredProcUnits []float64 `json:"configuredProcUnits"`
	BorrowedProcUnits   []float64 `json:"borrowedProcUnits"`
}

// SysProcessedServerNetwork represents network information
type SysProcessedServerNetwork struct {
	SriovAdapters []SysProcessedSriovAdapter `json:"sriovAdapters"`
	HEAdapters    []SysProcessedHEAdapter    `json:"HEAdapters"`
}

// SysProcessedSriovAdapter represents an adapter configuration
type SysProcessedSriovAdapter struct {
	DrcIndex      string                          `json:"drcIndex"`
	PhysicalPorts []SysProcessedSriovPhysicalPort `json:"physicalPorts"`
}

// SysProcessedSriovPhysicalPort represents a port
type SysProcessedSriovPhysicalPort struct {
	ID               int       `json:"id"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	ErrorIn          []float64 `json:"errorIn"`
	ErrorOut         []float64 `json:"errorOut"`
	TransferredBytes []float64 `json:"transferredBytes"`
}

// SysProcessedHEAdapter represents an adapter configuration
type SysProcessedHEAdapter struct {
	DrcIndex      string                       `json:"drcIndex"`
	PhysicalPorts []SysProcessedHEPhysicalPort `json:"physicalPorts"`
}

// SysProcessedHEPhysicalPort represents a port
type SysProcessedHEPhysicalPort struct {
	ID               int       `json:"id"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	TransferredBytes []float64 `json:"transferredBytes"`
}

// --- VIOS TELEMETRY ---

// SysProcessedViosUtil represents utility information
type SysProcessedViosUtil struct {
	ID            int                     `json:"id"`
	UUID          string                  `json:"uuid"`
	Name          string                  `json:"name"`
	State         string                  `json:"state"`
	AffinityScore float64                 `json:"affinityScore"`
	Memory        SysProcessedViosMem     `json:"memory"`
	Processor     SysProcessedViosProc    `json:"processor"`
	Network       SysProcessedViosNetwork `json:"network"`
	Storage       SysProcessedViosStorage `json:"storage"`
}

// SysProcessedViosMem represents data structure
type SysProcessedViosMem struct {
	AssignedMem          []float64 `json:"assignedMem"`
	UtilizedMem          []float64 `json:"utilizedMem"`
	VirtualPersistentMem []float64 `json:"virtualPersistentMem"`
}

// SysProcessedViosProc represents data structure
type SysProcessedViosProc struct {
	PoolID                      int       `json:"poolId"`
	Weight                      int       `json:"weight"`
	Mode                        string    `json:"mode"`
	MaxVirtualProcessors        []float64 `json:"maxVirtualProcessors"`
	CurrentVirtualProcessors    []float64 `json:"currentVirtualProcessors"`
	MaxProcUnits                []float64 `json:"maxProcUnits"`
	EntitledProcUnits           []float64 `json:"entitledProcUnits"`
	UtilizedProcUnits           []float64 `json:"utilizedProcUnits"`
	UtilizedProcUnitsDeductIdle []float64 `json:"utilizedProcUnitsDeductIdle"`
	UtilizedCappedProcUnits     []float64 `json:"utilizedCappedProcUnits"`
	UtilizedUncappedProcUnits   []float64 `json:"utilizedUncappedProcUnits"`
	IdleProcUnits               []float64 `json:"idleProcUnits"`
	DonatedProcUnits            []float64 `json:"donatedProcUnits"`
	TimeSpentWaitingForDispatch []float64 `json:"timeSpentWaitingForDispatch"`
	TimePerInstructionExecution []float64 `json:"timePerInstructionExecution"`
}

// SysProcessedViosNetwork represents network information
type SysProcessedViosNetwork struct {
	ClientLpars             []string                            `json:"clientLpars"`
	GenericAdapters         []SysProcessedViosGenericAdapter    `json:"genericAdapters"`
	SharedAdapters          []SysProcessedViosSharedAdapter     `json:"sharedAdapters"`
	VirtualEthernetAdapters []SysProcessedViosVirtualEthAdapter `json:"virtualEthernetAdapters"`
	SriovLogicalPorts       []SysProcessedViosSriovLogicalPort  `json:"sriovLogicalPorts"`
}

// SysProcessedViosGenericAdapter represents an adapter configuration
type SysProcessedViosGenericAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	TransferredBytes []float64 `json:"transferredBytes"`
}

// SysProcessedViosSharedAdapter represents an adapter configuration
type SysProcessedViosSharedAdapter struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	ReceivedPackets  []float64 `json:"receivedPackets"`
	SentPackets      []float64 `json:"sentPackets"`
	DroppedPackets   []float64 `json:"droppedPackets"`
	SentBytes        []float64 `json:"sentBytes"`
	ReceivedBytes    []float64 `json:"receivedBytes"`
	TransferredBytes []float64 `json:"transferredBytes"`
	BridgedAdapters  []string  `json:"bridgedAdapters"`
}

// SysProcessedViosVirtualEthAdapter represents an adapter configuration
type SysProcessedViosVirtualEthAdapter struct {
	PhysicalLocation         string    `json:"physicalLocation"`
	VlanID                   int       `json:"vlanId"`
	VswitchID                int       `json:"vswitchId"`
	IsPortVLANID             bool      `json:"isPortVLANID"`
	ReceivedPackets          []float64 `json:"receivedPackets"`
	SentPackets              []float64 `json:"sentPackets"`
	DroppedPackets           []float64 `json:"droppedPackets"`
	SentBytes                []float64 `json:"sentBytes"`
	ReceivedBytes            []float64 `json:"receivedBytes"`
	ReceivedPhysicalPackets  []float64 `json:"receivedPhysicalPackets"`
	SentPhysicalPackets      []float64 `json:"sentPhysicalPackets"`
	DroppedPhysicalPackets   []float64 `json:"droppedPhysicalPackets"`
	SentPhysicalBytes        []float64 `json:"sentPhysicalBytes"`
	ReceivedPhysicalBytes    []float64 `json:"receivedPhysicalBytes"`
	TransferredBytes         []float64 `json:"transferredBytes"`
	TransferredPhysicalBytes []float64 `json:"transferredPhysicalBytes"`
}

// SysProcessedViosSriovLogicalPort represents a port
type SysProcessedViosSriovLogicalPort struct {
	DrcIndex            string    `json:"drcIndex"`
	PhysicalLocation    string    `json:"physicalLocation"`
	PhysicalDrcIndex    string    `json:"physicalDrcIndex"`
	PhysicalPortID      int       `json:"physicalPortId"`
	ClientPartitionUUID string    `json:"clientPartitionUUID"`
	VnicDeviceMode      string    `json:"vnicDeviceMode"`
	ConfigurationType   string    `json:"configurationType"`
	ReceivedPackets     []float64 `json:"receivedPackets"`
	SentPackets         []float64 `json:"sentPackets"`
	DroppedPackets      []float64 `json:"droppedPackets"`
	SentBytes           []float64 `json:"sentBytes"`
	ReceivedBytes       []float64 `json:"receivedBytes"`
	ErrorIn             []float64 `json:"errorIn"`
	ErrorOut            []float64 `json:"errorOut"`
	TransferredBytes    []float64 `json:"transferredBytes"`
}

// SysProcessedViosStorage represents storage information
type SysProcessedViosStorage struct {
	ClientLpars             []string                            `json:"clientLpars"`
	GenericVirtualAdapters  []SysProcessedViosStorageGeneric    `json:"genericVirtualAdapters"`
	GenericPhysicalAdapters []SysProcessedViosStorageGeneric    `json:"genericPhysicalAdapters"`
	FiberChannelAdapters    []SysProcessedViosFCAdapter         `json:"fiberChannelAdapters"`
	SharedStoragePools      []SysProcessedViosSharedStoragePool `json:"sharedStoragePools"`
}

// SysProcessedViosStorageGeneric represents storage information
type SysProcessedViosStorageGeneric struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// SysProcessedViosFCAdapter represents an adapter configuration
type SysProcessedViosFCAdapter struct {
	ID               string    `json:"id"`
	WWPN             string    `json:"wwpn"`
	PhysicalLocation string    `json:"physicalLocation"`
	NumOfPorts       int       `json:"numOfPorts"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	RunningSpeed     []float64 `json:"runningSpeed"` // In GBPS
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// SysProcessedViosSharedStoragePool represents a resource pool
type SysProcessedViosSharedStoragePool struct {
	ID               string    `json:"id"`
	TotalSpace       []float64 `json:"totalSpace"`
	UsedSpace        []float64 `json:"usedSpace"`
	NumOfReads       []float64 `json:"numOfReads"`
	NumOfWrites      []float64 `json:"numOfWrites"`
	ReadBytes        []float64 `json:"readBytes"`
	WriteBytes       []float64 `json:"writeBytes"`
	TransmittedBytes []float64 `json:"transmittedBytes"`
}

// ShortTermMetricsOptions holds the query options for Short Term Monitor (STM) metric calls.
type ShortTermMetricsOptions struct {
	StartTS time.Time
	EndTS   time.Time
}

// =====================================================================
// SHORT TERM MONITOR (STM) RAW METRICS JSON SPECIFICATION
// =====================================================================

// StmRawMetricsPayload represents the top-level structure for Short Term Monitor (STM) raw metrics data
type StmRawMetricsPayload struct {
	SystemUtil StmSystemUtil `json:"systemUtil"`
}

// StmSystemUtil represents utility information
type StmSystemUtil struct {
	UtilInfo   StmUtilInfo   `json:"utilInfo"`
	UtilSample StmUtilSample `json:"utilSample"` // Note: Not an array in STM! Just a single object per file.
}

// StmUtilInfo represents information details
type StmUtilInfo struct {
	Version        string `json:"version"`
	MetricType     string `json:"metricType"`     // E.g., "Raw"
	MonitoringType string `json:"monitoringType"` // E.g., "STM"
	MTMS           string `json:"mtms"`
	Name           string `json:"name"`
}

// StmUtilSample represents utility information
type StmUtilSample struct {
	TimeStamp             string                   `json:"timeStamp"`
	Status                int                      `json:"status"`
	ErrorInfo             []StmErrorInfo           `json:"errorInfo,omitempty"`
	TimeBasedCycles       float64                  `json:"timeBasedCycles"`
	SystemFirmware        StmSystemFirmware        `json:"systemFirmware"`
	Processor             StmSystemProcessor       `json:"processor"`
	Memory                StmSystemMemory          `json:"memory"`
	SharedMemoryPool      []StmSharedMemoryPool    `json:"sharedMemoryPool"`
	PhysicalProcessorPool StmPhysicalProcessorPool `json:"physicalProcessorPool"`
	SharedProcessorPool   []StmSharedProcessorPool `json:"sharedProcessorPool"`
	Network               StmSystemNetwork         `json:"network"`
	LparsUtil             []StmLparUtil            `json:"lparsUtil"`
	ViosUtil              []StmViosUtil            `json:"viosUtil"`
}

// StmErrorInfo represents information details
type StmErrorInfo struct {
	ErrID  string `json:"errId"`
	ErrMsg string `json:"errMsg"`
}

// --- PHYP SYSTEM-LEVEL BLOCKS ---

// StmSystemFirmware represents data structure
type StmSystemFirmware struct {
	UtilizedProcCycles float64 `json:"utilizedProcCycles"`
	AssignedMem        float64 `json:"assignedMem"`
}

// StmSystemProcessor represents processor information
type StmSystemProcessor struct {
	TotalProcUnits        float64 `json:"totalProcUnits"`
	AvailableProcUnits    float64 `json:"availableProcUnits"`
	ConfigurableProcUnits float64 `json:"configurableProcUnits"`
	ProcCyclesPerSecond   float64 `json:"procCyclesPerSecond"`
}

// StmSystemMemory represents memory information
type StmSystemMemory struct {
	TotalMem             float64 `json:"totalMem"`
	AvailableMem         float64 `json:"availableMem"`
	ConfigurableMem      float64 `json:"configurableMem"`
	VirtualPersistentMem float64 `json:"virtualPersistentMem"`
}

// StmSharedMemoryPool represents a resource pool
type StmSharedMemoryPool struct {
	ID                         int     `json:"id"`
	Name                       string  `json:"name"`
	TotalMem                   float64 `json:"totalMem"`
	AssignedMemToLpars         float64 `json:"assignedMemToLpars"`
	AssignedMemToSysFirmware   float64 `json:"assignedMemToSysFirmware"`
	TotalIOMem                 float64 `json:"totalIOMem"`
	MappedIOMemToLpars         float64 `json:"mappedIOMemToLpars"`
	PageFaults                 float64 `json:"pageFaults"`
	PageDelays                 float64 `json:"pageDelays"`
	DedupedMemInPool           float64 `json:"dedupedMemInPool"`
	UtilizedProcCyclesForDedup float64 `json:"utilizedProcCyclesForDedup"`
}

// StmPhysicalProcessorPool represents a resource pool
type StmPhysicalProcessorPool struct {
	TotalPoolCycles            float64 `json:"totalPoolCycles"`
	UtilizedPoolCycles         float64 `json:"utilizedPoolCycles"`
	ConfigurablePoolProcUnits  float64 `json:"configurablePoolProcUnits"`
	CurrAvailablePoolProcUnits float64 `json:"currAvailablePoolProcUnits"`
	BorrowedPoolProcUnits      float64 `json:"borrowedPoolProcUnits"`
}

// StmSharedProcessorPool represents a resource pool
type StmSharedProcessorPool struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	AssignedProcCycles float64 `json:"assignedProcCycles"`
	UtilizedProcCycles float64 `json:"utilizedProcCycles"`
	MaxProcUnits       float64 `json:"maxProcUnits"`
	BorrowedProcUnits  float64 `json:"borrowedProcUnits"`
}

// StmSystemNetwork represents network information
type StmSystemNetwork struct {
	HEAdapters []StmHEAdapter `json:"HEAdapters"`
}

// StmHEAdapter represents an adapter configuration
type StmHEAdapter struct {
	DrcIndex      string              `json:"drcIndex"`
	PhysicalPorts []StmHEPhysicalPort `json:"physicalPorts"`
}

// StmHEPhysicalPort represents a port
type StmHEPhysicalPort struct {
	ID               int     `json:"id"`
	PhysicalLocation string  `json:"physicalLocation"`
	ReceivedPackets  float64 `json:"receivedPackets"`
	SentPackets      float64 `json:"sentPackets"`
	DroppedPackets   float64 `json:"droppedPackets"`
	SentBytes        float64 `json:"sentBytes"`
	ReceivedBytes    float64 `json:"receivedBytes"`
}

// --- LPAR TELEMETRY BLOCK ---

// StmLparUtil represents utility information
type StmLparUtil struct {
	ID             int              `json:"id"`
	UUID           string           `json:"uuid"`
	Type           string           `json:"type"`
	Name           string           `json:"name"`
	State          string           `json:"state"`
	MigrationState string           `json:"migrationState"`
	AffinityScore  float64          `json:"affinityScore"`
	Memory         StmLparMemory    `json:"memory"`
	Processor      StmLparProcessor `json:"processor"`
	Network        StmLparNetwork   `json:"network"`
	Storage        StmLparStorage   `json:"Storage"` // Note the uppercase 'S' as per IBM JSON Spec
}

// StmLparMemory represents memory information
type StmLparMemory struct {
	PoolID               int     `json:"poolId"`
	Weight               int     `json:"weight"`
	LogicalMem           float64 `json:"logicalMem"`
	BackedPhysicalMem    float64 `json:"backedPhysicalMem"`
	TotalIOMem           float64 `json:"totalIOMem"`
	MappedIOMem          float64 `json:"mappedIOMem"`
	DedupedMem           float64 `json:"dedupedMem"`
	VirtualPersistentMem float64 `json:"virtualPersistentMem"`
}

// StmLparProcessor represents processor information
type StmLparProcessor struct {
	PoolID                         int     `json:"poolId"`
	Mode                           string  `json:"mode"`
	MaxVirtualProcessors           float64 `json:"maxVirtualProcessors"`
	MaxProcUnits                   float64 `json:"maxProcUnits"`
	Weight                         int     `json:"weight"`
	EntitledProcCycles             float64 `json:"entitledProcCycles"`
	UtilizedCappedProcCycles       float64 `json:"utilizedCappedProcCycles"`
	UtilizedUnCappedProcCycles     float64 `json:"utilizedUnCappedProcCycles"`
	IdleProcCycles                 float64 `json:"idleProcCycles"`
	DonatedProcCycles              float64 `json:"donatedProcCycles"`
	RunLatchInstructions           float64 `json:"runLatchInstructions,omitempty"`
	RunLatchProcCycles             float64 `json:"runLatchProcCycles,omitempty"`
	TotalInstructions              float64 `json:"totalInstructions,omitempty"`
	TotalInstructionsExecutionTime float64 `json:"totalInstructionsExecutionTime,omitempty"`
	TimeSpentWaitingForProcessor   float64 `json:"timeSpentWaitingForProcessor"`
	NumOfTimesWaitedForProcessor   float64 `json:"numOfTimesWaitedForProcessor"`
	TimeSpentWaitingForDispatch    float64 `json:"timeSpentWaitingForDispatch"`
	NumOfTimesDispatched           float64 `json:"numOfTimesDispatched"`
}

// StmLparNetwork represents network information
type StmLparNetwork struct {
	VirtualEthernetAdapters []StmVirtualEthernetAdapter `json:"virtualEthernetAdapters"`
}

// StmVirtualEthernetAdapter represents an adapter configuration
type StmVirtualEthernetAdapter struct {
	VlanID                  int     `json:"vlanId"`
	VswitchID               int     `json:"vswitchId"`
	PhysicalLocation        string  `json:"physicalLocation"`
	IsPortVLANID            bool    `json:"isPortVLANID"`
	ReceivedPackets         float64 `json:"receivedPackets"`
	SentPackets             float64 `json:"sentPackets"`
	DroppedPackets          float64 `json:"droppedPackets"`
	SentBytes               float64 `json:"sentBytes"`
	ReceivedBytes           float64 `json:"receivedBytes"`
	ReceivedPhysicalPackets float64 `json:"receivedPhysicalPackets"`
	SentPhysicalPackets     float64 `json:"sentPhysicalPackets"`
	DroppedPhysicalPackets  float64 `json:"droppedPhysicalPackets"`
	SentPhysicalBytes       float64 `json:"sentPhysicalBytes"`
	ReceivedPhysicalBytes   float64 `json:"receivedPhysicalBytes"`
}

// StmLparStorage represents storage information
type StmLparStorage struct {
	VirtualFiberChannelAdapters []StmVirtualFCAdapter `json:"virtualFiberChannelAdapters"`
	GenericVirtualAdapters      []StmGenericVAdapter  `json:"genericVirtualAdapters"`
}

// StmVirtualFCAdapter represents an adapter configuration
type StmVirtualFCAdapter struct {
	ViosID           int      `json:"viosId"`
	WWPNPair         []string `json:"wwpnPair"`
	PhysicalLocation string   `json:"physicalLocation"`
}

// StmGenericVAdapter represents an adapter configuration
type StmGenericVAdapter struct {
	ViosID            int    `json:"viosId"`
	PhysicalLocation  string `json:"physicalLocation"`
	ViosAdapterSlotID int    `json:"viosAdapterSlotId"`
}

// --- VIOS TELEMETRY BLOCK ---

// StmViosUtil represents utility information
type StmViosUtil struct {
	ID            int              `json:"id"`
	UUID          string           `json:"uuid"`
	Name          string           `json:"name"`
	State         string           `json:"state"`
	AffinityScore float64          `json:"affinityScore"`
	Memory        StmViosMemory    `json:"memory"`
	Processor     StmLparProcessor `json:"processor"` // Reuses LPAR structure
	Network       StmLparNetwork   `json:"network"`   // Reuses LPAR structure
}

// StmViosMemory represents memory information
type StmViosMemory struct {
	AssignedMem          float64 `json:"assignedMem"`
	VirtualPersistentMem float64 `json:"virtualPersistentMem"`
}

// =====================================================================
// SHORT TERM MONITOR (STM) RAW METRICS - VIOS JSON SPECIFICATION
// =====================================================================

// StmRawViosMetricsPayload represents the payload structure
type StmRawViosMetricsPayload struct {
	SystemUtil StmRawViosSystemUtil `json:"systemUtil"`
}

// StmRawViosSystemUtil represents utility information
type StmRawViosSystemUtil struct {
	UtilInfo   StmRawViosUtilInfo   `json:"utilInfo"`
	UtilSample StmRawViosUtilSample `json:"utilSample"`
}

// StmRawViosUtilInfo represents information details
type StmRawViosUtilInfo struct {
	Version        string `json:"version"`
	MetricType     string `json:"metricType"`
	MonitoringType string `json:"monitoringType"`
	MTMS           string `json:"mtms"`
}

// StmRawViosUtilSample represents utility information
type StmRawViosUtilSample struct {
	TimeStamp string             `json:"timeStamp"`
	Status    int                `json:"status"`
	ErrorInfo []StmErrorInfo     `json:"errorInfo,omitempty"` // Reusing StmErrorInfo from PHYP
	ViosUtil  []StmRawViosDetail `json:"viosUtil"`
}

// StmRawViosDetail represents data structure
type StmRawViosDetail struct {
	ID        interface{}         `json:"id"`
	Name      string              `json:"name"`
	Processor StmRawViosProcessor `json:"processor"`
	Memory    StmRawViosMemory    `json:"memory"`
	Network   StmRawViosNetwork   `json:"network"`
	Storage   StmRawViosStorage   `json:"storage"`
}

// --- VIOS COMPUTE ---

// StmRawViosProcessor represents processor information
type StmRawViosProcessor struct {
	UserCounter     float64 `json:"userCounter"`
	KernelCounter   float64 `json:"kernelCounter"`
	PurrCounter     float64 `json:"purrCounter"`
	SpurrCounter    float64 `json:"spurrCounter"`
	TimeBaseCounter float64 `json:"timeBaseCounter"`
}

// StmRawViosMemory represents memory information
type StmRawViosMemory struct {
	UtilizedMem            float64 `json:"utilizedMem"`
	UsedForNetworkBuffer   float64 `json:"usedForNetworkBuffer"`
	UsedForOtherOperations float64 `json:"usedForOtherOperations"`
	SwapSpaceUsed          float64 `json:"swapSpaceUsed"`
}

// --- VIOS NETWORK ---

// StmRawViosNetwork represents network information
type StmRawViosNetwork struct {
	GenericAdapters []StmRawViosGenericAdapter `json:"genericAdapters"`
	SharedAdapters  []StmRawViosSharedAdapter  `json:"sharedAdapters"`
}

// StmRawViosGenericAdapter represents an adapter configuration
type StmRawViosGenericAdapter struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	PhysicalLocation string  `json:"physicalLocation"`
	ReceivedPackets  float64 `json:"receivedPackets"`
	SentPackets      float64 `json:"sentPackets"`
	DroppedPackets   float64 `json:"droppedPackets"`
	SentBytes        float64 `json:"sentBytes"`
	ReceivedBytes    float64 `json:"receivedBytes"`
}

// StmRawViosSharedAdapter represents an adapter configuration
type StmRawViosSharedAdapter struct {
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	PhysicalLocation string   `json:"physicalLocation"`
	ReceivedPackets  float64  `json:"receivedPackets"`
	SentPackets      float64  `json:"sentPackets"`
	DroppedPackets   float64  `json:"droppedPackets"`
	SentBytes        float64  `json:"sentBytes"`
	ReceivedBytes    float64  `json:"receivedBytes"`
	BridgedAdapters  []string `json:"bridgedAdapters"`
}

// --- VIOS STORAGE ---

// StmRawViosStorage represents storage information
type StmRawViosStorage struct {
	GenericPhysicalAdapters []StmRawViosStorageGeneric    `json:"genericPhysicalAdapters"`
	GenericVirtualAdapters  []StmRawViosStorageGeneric    `json:"genericVirtualAdapters"`
	FiberChannelAdapters    []StmRawViosFCAdapter         `json:"fiberChannelAdapters"`
	SharedStoragePools      []StmRawViosSharedStoragePool `json:"sharedStoragePools"`
	PhysicalDevices         []StmRawViosPhysicalDevice    `json:"physicalDevices"`
	VirtualDevices          []StmRawViosVirtualDevice     `json:"virtualDevices"`
}

// StmRawViosStorageGeneric represents storage information
type StmRawViosStorageGeneric struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	PhysicalLocation string  `json:"physicalLocation"`
	NumOfReads       float64 `json:"numOfReads"`
	NumOfWrites      float64 `json:"numOfWrites"`
	ReadBytes        float64 `json:"readBytes"`
	WriteBytes       float64 `json:"writeBytes"`
}

// StmRawViosFCAdapter represents an adapter configuration
type StmRawViosFCAdapter struct {
	ID               string             `json:"id"`
	WWPN             string             `json:"wwpn"`
	PhysicalLocation string             `json:"physicalLocation"`
	NumOfReads       float64            `json:"numOfReads"`
	NumOfWrites      float64            `json:"numOfWrites"`
	ReadBytes        float64            `json:"readBytes"`
	WriteBytes       float64            `json:"writeBytes"`
	RunningSpeed     float64            `json:"runningSpeed"` // In GBPS
	Ports            []StmRawViosFCPort `json:"ports"`
}

// StmRawViosFCPort represents a port
type StmRawViosFCPort struct {
	ID               string  `json:"id"`
	WWPN             string  `json:"wwpn"`
	PhysicalLocation string  `json:"physicalLocation"`
	NumOfReads       float64 `json:"numOfReads"`
	NumOfWrites      float64 `json:"numOfWrites"`
	ReadBytes        float64 `json:"readBytes"`
	WriteBytes       float64 `json:"writeBytes"`
	RunningSpeed     float64 `json:"runningSpeed"`
}

// StmRawViosSharedStoragePool represents a resource pool
type StmRawViosSharedStoragePool struct {
	ID                 string   `json:"id"`
	PoolDisks          []string `json:"poolDisks"`
	PoolVirtualDevices []string `json:"poolVirtualDevices"`
	NumOfReads         float64  `json:"numOfReads"`
	NumOfWrites        float64  `json:"numOfWrites"`
	TotalSpace         float64  `json:"totalSpace"`
	UsedSpace          float64  `json:"usedSpace"`
	ReadBytes          float64  `json:"readBytes"`
	WriteBytes         float64  `json:"writeBytes"`
}

// StmRawViosPhysicalDevice represents a device
type StmRawViosPhysicalDevice struct {
	ID                           string  `json:"id"`
	UID                          string  `json:"uid"`
	DiskAdapterID                string  `json:"diskAdapterId"`
	PoolID                       string  `json:"poolId"`
	NumOfReads                   float64 `json:"numOfReads"`
	NumOfWrites                  float64 `json:"numOfWrites"`
	ReadBytes                    float64 `json:"readBytes"`
	WriteBytes                   float64 `json:"writeBytes"`
	ReadServiceTime              float64 `json:"readServiceTime"`
	WriteServiceTime             float64 `json:"writeServiceTime"`
	TimeSpentInWaitQueue         float64 `json:"timeSpentInWaitQueue"`
	WaitQueueSize                float64 `json:"waitQueueSize"`
	NumOfTimesServiceQueueIsFull float64 `json:"numOfTimesServiceQueueIsFull"`
}

// StmRawViosVirtualDevice represents a device
type StmRawViosVirtualDevice struct {
	ID                           string  `json:"id"`
	UID                          string  `json:"uid"`
	PoolID                       string  `json:"poolId"`
	TotalSpace                   float64 `json:"totalSpace"`
	UsedSpace                    float64 `json:"usedSpace"`
	NumOfReads                   float64 `json:"numOfReads"`
	NumOfWrites                  float64 `json:"numOfWrites"`
	ReadBytes                    float64 `json:"readBytes"`
	WriteBytes                   float64 `json:"writeBytes"`
	ReadServiceTime              float64 `json:"readServiceTime"`
	WriteServiceTime             float64 `json:"writeServiceTime"`
	TimeSpentInWaitQueue         float64 `json:"timeSpentInWaitQueue"`
	WaitQueueSize                float64 `json:"waitQueueSize"`
	NumOfTimesServiceQueueIsFull float64 `json:"numOfTimesServiceQueueIsFull"`
}

// ValidationResult holds the success and failure messages for redundancy validation checks.
type ValidationResult struct {
	Success []string `json:"success"`
	Failure []string `json:"failure"`
}

// MaintenanceStep holds the unconfiguration success and failure lists during the maintenance teardown.
type MaintenanceStep struct {
	UnConfigureSuccess []string `json:"unConfigureSuccess"`
	UnConfigureFailure []string `json:"unConfigureFailure"`
}

// PrepareMaintenanceResult represents the unmarshaled JSON string found inside the Job's final "result" parameter.
type PrepareMaintenanceResult struct {
	VirtualSCSIValidationResults  ValidationResult `json:"virtualSCSIValidationResults"`
	VirtualFCValidationResults    ValidationResult `json:"virtualFCValidationResults"`
	VirtualLANValidationResults   ValidationResult `json:"virtualLANValidationResults"`
	VirtualNICValidationResults   ValidationResult `json:"virtualNICValidationResults"`
	VirtualSCSIPrepareMaintenance MaintenanceStep  `json:"virtualSCSIPrepareMaintenance"`
	VirtualFCPrepareMaintenance   MaintenanceStep  `json:"virtualFCPrepareMaintenance"`
	VirtualLANPrepareMaintenance  MaintenanceStep  `json:"virtualLANPrepareMaintenance"`
	VirtualNICPrepareMaintenance  MaintenanceStep  `json:"virtualNICPrepareMaintenance"`
}

// UpdateVIOSOptions holds the configuration parameters for updating a Virtual I/O Server.
// Depending on the ResourceType, different fields become mandatory.
type UpdateVIOSOptions struct {
	ResourceType    string // Required: HMC, NFS, SFTP, USB, IBMWebsite
	Name            string // Required: The name of the update VIOS image
	ServerHostOrIP  string // Required for NFS and SFTP
	UserName        string // Required for SFTP
	Password        string // Required for SFTP (if not using SSHKey)
	SSHKey          string // Optional for SFTP: The SSH private key file
	PassPhrase      string // Optional for SFTP: The Passphrase for the SSH key
	RemoteDirectory string // Required for NFS and SFTP: The directory where the file exists
	FileNames       string // Optional: Comma-separated list. If omitted, all files are copied
	MountLocation   string // Optional for NFS: Target mount location
	MountOptions    string // Optional for NFS: Additional mount command options (e.g., "vers=4")
	USBDevice       string // Required for USB: The name of the USB device (e.g., "/dev/sdb1")
	SaveFile        bool   // Flag to indicate if the remote VIOS update must be saved in the HMC
	RestartVIOS     bool   // Flag to indicate if VIOS must be automatically restarted after update
}


// VirtualNetwork represents a Virtual LAN connectivity object across logical partitions on a Managed System.
type VirtualNetwork struct {
	XMLName          xml.Name `xml:"VirtualNetwork"`
	SchemaVersion    string   `xml:"schemaVersion,attr,omitempty"`
	UUID             string   `xml:"Metadata>Atom>AtomID,omitempty"`
	NetworkName      string   `xml:"NetworkName"`
	NetworkVLANID    int      `xml:"NetworkVLANID"`
	TaggedNetwork    bool     `xml:"TaggedNetwork"`
	AssociatedSwitch LinkXML  `xml:"AssociatedSwitch"` // Fixed: Matches the HMC XML schema
}

// CreateVirtualNetworkRequest holds the parameters needed to provision a new Virtual Network.
type CreateVirtualNetworkRequest struct {
	NetworkName   string
	NetworkVLANID int
	TaggedNetwork bool
	VSwitchUUID   string // The UUID of the Virtual Switch to bind this network to
}

// CreateVirtualSwitchRequest holds the parameters needed to provision a new Virtual Switch.
type CreateVirtualSwitchRequest struct {
	SwitchName string
	SwitchMode string // Optional: "Veb" (default) or "Vepa"
}

// LoadGroup represents a collection of Virtual Switches mapped to a specific Port VLAN ID.
type LoadGroup struct {
	PortVLANID      int       `xml:"PortVLANID"`
	VirtualSwitches []LinkXML `xml:"VirtualSwitches>link"`
}

// NetworkBridge represents the REST API wrapper for Shared Ethernet Adapters.
type NetworkBridge struct {
	XMLName                xml.Name                `xml:"NetworkBridge"`
	SchemaVersion          string                  `xml:"schemaVersion,attr,omitempty"`
	UUID                   string                  `xml:"Metadata>Atom>AtomID,omitempty"`
	FailoverEnabled        bool                    `xml:"FailoverEnabled"`
	LoadBalancingEnabled   bool                    `xml:"LoadBalancingEnabled"`
	ControlChannelID       int                     `xml:"ControlChannelID,omitempty"`
	PortVLANID             int                     `xml:"PortVLANID"`
	SharedEthernetAdapters []SharedEthernetAdapter `xml:"SharedEthernetAdapters>SharedEthernetAdapter"`
	LoadGroups             []LoadGroup             `xml:"LoadGroups>LoadGroup"`
}

// CreateNetworkBridgeRequest holds the parameters needed to provision a new Network Bridge.
// CreateNetworkBridgeRequest defines the parameters for provisioning a bridge.
type CreateNetworkBridgeRequest struct {
	PortVLANID             int
	FailoverEnabled        bool
	LoadBalancingEnabled   bool
	ControlChannelID       int
	PrimaryViosUUID        string
	PrimaryBackingDevice   string
	SecondaryViosUUID      string
	SecondaryBackingDevice string
	JumboFramesEnabled     bool
	LargeSend              bool
	LoadGroupVLANs         []int // Converted list of tracking data VLANs (e.g., [1127, 1128])
}