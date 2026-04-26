package hmc

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/beevik/etree"
)

// LPAR_TEMPLATE_NS is the namespace for PartitionTemplate as used in the Python code
const LPAR_TEMPLATE_NS = `PartitionTemplate xmlns="http://www.ibm.com/xmlns/systems/power/firmware/templates/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2"`

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
	XMLName           xml.Name              `xml:"JobResponse"`
	RequestURL        JobResponseURL        `xml:"RequestURL"`
	TargetUuid        string                `xml:"TargetUuid"`
	JobID             string                `xml:"JobID"`
	TimeStarted       string                `xml:"TimeStarted"`
	TimeCompleted     string                `xml:"TimeCompleted"`
	Status            string                `xml:"Status"`
	JobRequestInstance JobResponseRequest   `xml:"JobRequestInstance"`
	Progress          JobResponseProgress   `xml:"Progress"`
	Results           JobResponseResults    `xml:"Results"`
}

// JobResponseURL represents the URL to which the JobRequest was submitted
type JobResponseURL struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr"`
	Title string `xml:"title,attr"`
}

// JobResponseRequest represents the job request instance details in a response
type JobResponseRequest struct {
	RequestedOperation JobResponseOperation   `xml:"RequestedOperation"`
	JobParameters      JobResponseParameters  `xml:"JobParameters"`
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

// Logger with prefix for HMC operations
var hmcLogger = log.New(io.Discard, "[HMC] ", log.LstdFlags)

// ReinitLogger reinitializes the HMC logger with a custom writer
// Pass io.Discard to suppress output, or a file/buffer to capture logs
func ReinitLogger(w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	hmcLogger = log.New(w, "[HMC] ", log.LstdFlags)
}

// HmcRestClient represents the REST client for HMC operations
type HmcRestClient struct {
	hmcIP   string
	session string
	client  *http.Client
	Logger  *Logger // <-- Add your new structured logger here
}

// NewHmcRestClient initializes a new HmcRestClient with an insecure TLS HTTP client
func NewHmcRestClient(hmcIP string) *HmcRestClient {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	
	// Create a default logger. We also set a prefix so all HMC logs are clearly marked!
	defaultLogger := NewDefaultLogger()
	defaultLogger.SetPrefix("[HMC]")

	return &HmcRestClient{
		hmcIP:  hmcIP,
		client: client,
		Logger: defaultLogger,
	}
}

// EnableVerboseLogging turns on debug-level output for the HMC client
func (c *HmcRestClient) EnableVerboseLogging() {
    c.Logger.EnableDebug()
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
	AllocatedVirtualProcessors     float64 `json:"AllocatedVirtualProcessors"`
	
	PartitionState                 string  `json:"PartitionState"`
	ResourceMonitoringIPAddress    *string `json:"ResourceMonitoringIPAddress"`
	HasPhysicalIO                  string  `json:"HasPhysicalIO"`
	SystemName                     string  `json:"SystemName"`
	SharingMode                    string  `json:"SharingMode"`
	MigrationDisable               bool    `json:"MigrationDisable"`
	
	// CHANGED TO float64
	CurrentProcessors              float64 `json:"CurrentProcessors"`
	
	LastActivatedProfile           string  `json:"LastActivatedProfile"`
	CurrentUncappedWeight          int     `json:"CurrentUncappedWeight"`
	RemoteRestartState             string  `json:"RemoteRestartState"`
	PartitionType                  string  `json:"PartitionType"`
	PartitionName                  string  `json:"PartitionName"`
	RMCState                       string  `json:"RMCState"`
	OperatingSystemType            string  `json:"OperatingSystemType"`
	
	// CHANGED TO float64
	CurrentMemory                  float64 `json:"CurrentMemory"`
	
	HasDedicatedProcessors         string  `json:"HasDedicatedProcessors"`
	AssociatedManagedSystem        string  `json:"AssociatedManagedSystem"`
	ReferenceCode                  string  `json:"ReferenceCode"`
	CurrentProcessingUnits         float64 `json:"CurrentProcessingUnits"`
	UUID                           string  // Manually set, not from JSON
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
	SystemName                               string  `json:"SystemName"`
	UUID                                     string  `json:"UUID"`
	State                                    string  `json:"State"`
	StateDetail                              string  `json:"StateDetail"`
	IPAddress                                string  `json:"IPAddress"`
	MTMS                                     string  `json:"MTMS"`
	SystemType                               string  `json:"SystemType"`
	SystemFirmware                           string  `json:"SystemFirmware"`
	SystemLocation                           *string `json:"SystemLocation"` // Using pointer for null values
	Description                              *string `json:"Description"`
	
	// Memory Metrics (MB)
	ConfigurableSystemMemory                 float64 `json:"ConfigurableSystemMemory"`
	CurrentAvailableSystemMemory             float64 `json:"CurrentAvailableSystemMemory"`
	InstalledSystemMemory                    float64 `json:"InstalledSystemMemory"`
	PermanentSystemMemory                    float64 `json:"PermanentSystemMemory"`
	MemoryDefragmentationState               string  `json:"MemoryDefragmentationState"`
	
	// Processor Metrics
	ConfigurableSystemProcessorUnits         float64 `json:"ConfigurableSystemProcessorUnits"`
	CurrentAvailableSystemProcessorUnits     float64 `json:"CurrentAvailableSystemProcessorUnits"`
	InstalledSystemProcessorUnits            float64 `json:"InstalledSystemProcessorUnits"`
	PermanentSystemProcessors                float64 `json:"PermanentSystemProcessors"`
	ProcessorThrottling                      string  `json:"ProcessorThrottling"` // String "true"/"false"
	
	// Versioning & Levels
	ActivatedLevel                           string  `json:"ActivatedLevel"`
	ActivatedServicePackNameAndLevel         string  `json:"ActivatedServicePackNameAndLevel"`
	DeferredLevel                            *string `json:"DeferredLevel"`
	DeferredServicePackNameAndLevel          *string `json:"DeferredServicePackNameAndLevel"`
	ServiceProcessorVersion                  string  `json:"ServiceProcessorVersion"`
	BMCVersion                               *string `json:"BMCVersion"`
	PNORVersion                              *string `json:"PNORVersion"`
	
	// Capabilities & Flags (All reported as strings "true"/"false")
	CapacityOnDemandProcessorCapable         string  `json:"CapacityOnDemandProcessorCapable"`
	CapacityOnDemandMemoryCapable            string  `json:"CapacityOnDemandMemoryCapable"`
	ManufacturingDefaultConfigurationEnabled string  `json:"ManufacturingDefaultConfigurationEnabled"`
	PhysicalSystemAttentionLEDState          string  `json:"PhysicalSystemAttentionLEDState"`
	IsClassicHMCManagement                   string  `json:"IsClassicHMCManagement"`
	IsPowerVMManagementController            string  `json:"IsPowerVMManagementController"`
	IsNotPowerVMManagementController         string  `json:"IsNotPowerVMManagementController"`
	IsPowerVMManagementMaster                string  `json:"IsPowerVMManagementMaster"`
	IsNotPowerVMManagementMaster             string  `json:"IsNotPowerVMManagementMaster"`
	
	// Miscellaneous
	MaximumPartitions                        int     `json:"MaximumPartitions"`
	ReferenceCode                            string  `json:"ReferenceCode"`
	MergedReferenceCode                      string  `json:"MergedReferenceCode"`
	MeteredPoolID                            *string `json:"MeteredPoolID"`
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

// Define the collection struct for unmarshaling
type PhysicalVolumeCollection struct {
	XMLName         xml.Name         `xml:"PhysicalVolume_Collection"`
	PhysicalVolumes []PhysicalVolume `xml:"PhysicalVolume"`
}


type IOAdapterInfo struct {
    Description                     string
    LogicalPartitionAssignmentCapable bool
    DeviceName                      string
}
// --- Append Below to logicalpartition.go ---

// StorageMap holds the dynamically discovered VIOS and Volume details for an LPAR
type StorageMap struct {
	ViosUUID        string
	ViosName        string
	VolumeName      string
	VolumeUDID      string // Very useful for matching against SVC VDisk UID
	ServerAdapter   string // Virtual SCSI adapter on VIOS side (e.g., vhost0)
	ClientAdapter   string // Virtual SCSI adapter on client/LPAR side (e.g., vtscsi0)
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

// VirtualIOServer represents the complete XML payload of a Virtual I/O Server.
// RENAMED from VirtualIOServerDetailed
type VirtualIOServerDetailed struct {
	XMLName       xml.Name `xml:"VirtualIOServer"`
	SchemaVersion string   `xml:"schemaVersion,attr"`

	// --- Metadata ---
	Metadata struct {
		AtomID      string `xml:"Atom>AtomID"`
		AtomCreated string `xml:"Atom>AtomCreated"`
	} `xml:"Metadata"`

	// --- Basic Info & Identifiers ---
	PartitionUUID          string `xml:"PartitionUUID"`
	PartitionID            int    `xml:"PartitionID"`
	PartitionName          string `xml:"PartitionName"`
	PartitionType          string `xml:"PartitionType"`
	SystemName             string `xml:"SystemName"`
	LogicalSerialNumber    string `xml:"LogicalSerialNumber"`
	OperatingSystemType    string `xml:"OperatingSystemType"`
	OperatingSystemVersion string `xml:"OperatingSystemVersion"`
	ReferenceCode          string `xml:"ReferenceCode"`
	LastActivatedProfile   string `xml:"LastActivatedProfile"`
	DefaultProfileName     string `xml:"DefaultProfileName"`

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
	CurrentHardwarePageTableRatio         int     `xml:"CurrentHardwarePageTableRatio"`
	CurrentHugePageCount                  int     `xml:"CurrentHugePageCount"`
	CurrentMaximumHugePageCount           int     `xml:"CurrentMaximumHugePageCount"`
	CurrentMaximumMemory                  float64 `xml:"CurrentMaximumMemory"`
	CurrentMemory                         float64 `xml:"CurrentMemory"`
	CurrentMinimumHugePageCount           int     `xml:"CurrentMinimumHugePageCount"`
	CurrentMinimumMemory                  float64 `xml:"CurrentMinimumMemory"`
	DesiredMemory                         float64 `xml:"DesiredMemory"`
	ExpansionFactor                       float64 `xml:"ExpansionFactor"`
	HardwarePageTableRatio                int     `xml:"HardwarePageTableRatio"`
	MaximumMemory                         float64 `xml:"MaximumMemory"`
	MemoryEncryptionHardwareAccessEnabled bool    `xml:"MemoryEncryptionHardwareAccessEnabled"`
	MemoryExpansionEnabled                bool    `xml:"MemoryExpansionEnabled"`
	MemoryExpansionHardwareAccessEnabled  bool    `xml:"MemoryExpansionHardwareAccessEnabled"`
	MinimumMemory                         float64 `xml:"MinimumMemory"`
	PhysicalPageTableRatio                int     `xml:"PhysicalPageTableRatio"`
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

type DedicatedProcessorConfiguration struct {
	DesiredProcessors float64 `xml:"DesiredProcessors"`
	MaximumProcessors float64 `xml:"MaximumProcessors"`
	MinimumProcessors float64 `xml:"MinimumProcessors"`
}

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

type ProfileIOSlot struct {
	AssociatedIOSlot AssociatedIOSlot `xml:"AssociatedIOSlot"`
}

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

type HardwareAcceleratorQoS struct {
	Metadata struct {
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
}

type VirtualIOServerCapabilities struct {
	GPFSCapable         bool `xml:"GPFSCapable"`
	IsTierCapable       bool `xml:"IsTierCapable"`
	IsTierMirrorCapable bool `xml:"IsTierMirrorCapable"`
}

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

type VirtualFibreChannelMapping struct {
	AssociatedLogicalPartition LinkXML       `xml:"AssociatedLogicalPartition"`
	ClientAdapter              ClientAdapter `xml:"ClientAdapter"`
	ServerAdapter              ServerAdapter `xml:"ServerAdapter"`
	Port                       Port          `xml:"Port"`
}

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

type VirtualSCSIMapping struct {
	AssociatedLogicalPartition LinkXML       `xml:"AssociatedLogicalPartition"`
	ClientAdapter              ClientAdapter `xml:"ClientAdapter"` // Reusing ClientAdapter Struct
	ServerAdapter              ServerAdapter `xml:"ServerAdapter"` // Reusing ServerAdapter Struct
	Storage                    Storage       `xml:"Storage"`
	TargetDevice               TargetDevice  `xml:"TargetDevice"`
}

type Storage struct {
	PhysicalVolume      PhysicalVolume      `xml:"PhysicalVolume"`
	VirtualOpticalMedia VirtualOpticalMedia `xml:"VirtualOpticalMedia"`
	VirtualDisk         VirtualDisk         `xml:"VirtualDisk"`
}

type TargetDevice struct {
	PhysicalVolumeVirtualTargetDevice PhysicalVolumeVirtualTargetDevice `xml:"PhysicalVolumeVirtualTargetDevice"`
	VirtualOpticalTargetDevice        VirtualOpticalTargetDevice        `xml:"VirtualOpticalTargetDevice"`
	LogicalVolumeVirtualTargetDevice  LogicalVolumeVirtualTargetDevice  `xml:"LogicalVolumeVirtualTargetDevice"`
}

type LogicalVolumeVirtualTargetDevice struct {
	LogicalUnitAddress string `xml:"LogicalUnitAddress"`
	TargetName         string `xml:"TargetName"`
	UniqueDeviceID     string `xml:"UniqueDeviceID"`
}

type PhysicalVolumeVirtualTargetDevice struct {
	LogicalUnitAddress string `xml:"LogicalUnitAddress"`
	TargetName         string `xml:"TargetName"`
	UniqueDeviceID     string `xml:"UniqueDeviceID"`
}

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
	OsType           string  // NEW: e.g., "AIX/Linux", "OS400", or "Virtual IO Server"
	MinMem           int     // MB
	DesiredMem       int     // MB
	MaxMem           int     // MB
	MinProcUnits     float64 
	DesiredProcUnits float64
	MaxProcUnits     float64
	MinVcpus         int
	DesiredVcpus     int
	MaxVcpus         int
	SharingMode      string  // "uncapped" or "capped"
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
	XMLName                             xml.Name  `xml:"ClientNetworkAdapter"`
	UUID                                string    `xml:"Metadata>Atom>AtomID"`
	DynamicReconfigurationConnectorName string    `xml:"DynamicReconfigurationConnectorName"`
	LocationCode                        string    `xml:"LocationCode"`
	LocalPartitionID                    string    `xml:"LocalPartitionID"`
	RequiredAdapter                     string    `xml:"RequiredAdapter"`
	VariedOn                            string    `xml:"VariedOn"`
	VirtualSlotNumber                   string    `xml:"VirtualSlotNumber"`
	AllowedOperatingSystemMACAddresses  string    `xml:"AllowedOperatingSystemMACAddresses"`
	MACAddress                          string    `xml:"MACAddress"`
	PortVLANID                          string    `xml:"PortVLANID"`
	QualityOfServicePriorityEnabled     string    `xml:"QualityOfServicePriorityEnabled"`
	TaggedVLANSupported                 string    `xml:"TaggedVLANSupported"`
	VirtualSwitchID                     string    `xml:"VirtualSwitchID"`
	VirtualSwitchName                   string    `xml:"VirtualSwitchName"`
	HCNID                               string    `xml:"HCNID"`
	
	// Kept your original naming, but mapped to LinkXML to capture the 'href' attribute
	AssociatedVirtualSwitchURI          LinkXML   `xml:"AssociatedVirtualSwitch>link"`
	VirtualNetworkURIs                  []LinkXML `xml:"VirtualNetworks>link"`
}

// NetworkBootDeviceCollection represents the collection of network boot devices returned by HMC
type NetworkBootDeviceCollection struct {
	XMLName       xml.Name            `xml:"NetworkBootDevice_Collection"`
	SchemaVersion string              `xml:"schemaVersion,attr"`
	Metadata      struct {
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	Devices []NetworkBootDeviceXML `xml:"NetworkBootDevice"`
}

// NetworkBootDeviceXML represents a network boot device as returned by HMC REST API
type NetworkBootDeviceXML struct {
	XMLName          xml.Name `xml:"NetworkBootDevice"`
	SchemaVersion    string   `xml:"schemaVersion,attr"`
	Metadata         struct {
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	BootDevice       string `xml:"BootDevice"`       // Device name (e.g., "ent")
	IsPhysicalDevice string `xml:"IsPhysicalDevice"` // "true" or "false" as string
	LocationCode     string `xml:"LocationCode"`     // Physical location code with port suffix
	MACAddressValue  string `xml:"MACAddressValue"`  // MAC address
}

// NetworkBootDevice represents a network boot device from an LPAR profile.
type NetworkBootDevice struct {
	DeviceName   string `json:"device_name"`     // Name of the network device (e.g., "ent")
	DeviceType   string `json:"device_type"`     // Type: "physical" or "virtual"
	LocationCode string `json:"location_code"`   // Physical location code of the adapter
	MACAddress   string `json:"mac_address"`     // MAC address of the adapter
	AdapterID    string `json:"adapter_id"`      // Adapter identifier (if available)
	BootPriority int    `json:"boot_priority"`   // Boot priority order (if available)
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
	ProfileName                          string `xml:"ProfileName"`
	ProfileType                          string `xml:"ProfileType"`
	SettingID                            string `xml:"SettingID"`
	
	// Partition Association
	AssociatedPartition                  AssociatedPartitionLink `xml:"AssociatedPartition"`
	
	// Configuration Flags
	AffinityGroupID                      string `xml:"AffinityGroupID"`
	AssignAllResources                   string `xml:"AssignAllResources"`
	AutoStart                            string `xml:"AutoStart"`
	BootMode                             string `xml:"BootMode"`
	ConnectionMonitoringEnabled          string `xml:"ConnectionMonitoringEnabled"`
	DesiredProcessorCompatibilityMode    string `xml:"DesiredProcessorCompatibilityMode"`
	RedundantErrorPathReportingEnabled   string `xml:"RedundantErrorPathReportingEnabled"`
	
	// I/O Configuration
	MaximumVirtualIOSlots                string `xml:"IOConfigurationInstance>MaximumVirtualIOSlots"`
	
	// Processor Configuration
	ProcessorConfig                      ProfileProcessorConfig `xml:"ProcessorAttributes"`
	
	// Memory Configuration
	MemoryConfig                         ProfileMemoryConfig `xml:"ProfileMemory"`
}

// ProfileProcessorConfig holds processor-related configuration for a profile
type ProfileProcessorConfig struct {
	HasDedicatedProcessors               string `xml:"HasDedicatedProcessors"`
	SharingMode                          string `xml:"SharingMode"`
	
	// Shared Processor Configuration
	SharedConfig                         SharedProcessorConfig `xml:"SharedProcessorConfiguration"`
	
	// Dedicated Processor Configuration
	DedicatedConfig                      DedicatedProcessorConfig `xml:"DedicatedProcessorConfiguration"`
}

// SharedProcessorConfig holds shared processor configuration
type SharedProcessorConfig struct {
	DesiredProcessingUnits               string `xml:"DesiredProcessingUnits"`
	DesiredVirtualProcessors             string `xml:"DesiredVirtualProcessors"`
	MaximumProcessingUnits               string `xml:"MaximumProcessingUnits"`
	MaximumVirtualProcessors             string `xml:"MaximumVirtualProcessors"`
	MinimumProcessingUnits               string `xml:"MinimumProcessingUnits"`
	MinimumVirtualProcessors             string `xml:"MinimumVirtualProcessors"`
	SharedProcessorPoolID                string `xml:"SharedProcessorPoolID"`
	SharedProcessorPoolName              string `xml:"SharedProcessorPoolName"`
	UncappedWeight                       string `xml:"UncappedWeight"`
}

// DedicatedProcessorConfig holds dedicated processor configuration
type DedicatedProcessorConfig struct {
	DesiredProcessors                    string `xml:"DesiredProcessors"`
	MaximumProcessors                    string `xml:"MaximumProcessors"`
	MinimumProcessors                    string `xml:"MinimumProcessors"`
}

// ProfileMemoryConfig holds memory-related configuration for a profile
type ProfileMemoryConfig struct {
	DesiredMemory                        string `xml:"DesiredMemory"`
	MaximumMemory                        string `xml:"MaximumMemory"`
	MinimumMemory                        string `xml:"MinimumMemory"`
	
	// Advanced Memory Features
	ActiveMemoryExpansionEnabled         string `xml:"ActiveMemoryExpansionEnabled"`
	ActiveMemorySharingEnabled           string `xml:"ActiveMemorySharingEnabled"`
	
	// Huge Pages
	DesiredHugePageCount                 string `xml:"DesiredHugePageCount"`
	MaximumHugePageCount                 string `xml:"MaximumHugePageCount"`
	MinimumHugePageCount                 string `xml:"MinimumHugePageCount"`
	
	// Page Table Configuration
	ExpansionFactor                      string `xml:"ExpansionFactor"`
	HardwarePageTableRatio               string `xml:"HardwarePageTableRatio"`
	DesiredPhysicalPageTableRatio        string `xml:"DesiredPhysicalPageTableRatio"`
}


// =====================================================================
// EXHAUSTIVE MANAGED SYSTEM XML STRUCTURES
// =====================================================================

type LinkXML struct {
	Href string `xml:"href,attr"`
}

type MachineTypeModelAndSerialNumber struct {
	MachineType  string `xml:"MachineType"`
	Model        string `xml:"Model"`
	SerialNumber string `xml:"SerialNumber"`
}

type ManagedSystemDetailed struct {
	XMLName                                  xml.Name                            `xml:"ManagedSystem"`
	MetadataID                               string                              `xml:"Metadata>Atom>AtomID"`
	ActivatedLevel                           string                              `xml:"ActivatedLevel"`
	ActivatedServicePackNameAndLevel         string                              `xml:"ActivatedServicePackNameAndLevel"`
	IPLConfig                                SystemIPLConfiguration              `xml:"AssociatedIPLConfiguration"`
	AssociatedLogicalPartitions              []LinkXML                           `xml:"AssociatedLogicalPartitions>link"`
	Capabilities                             SystemCapabilities                  `xml:"AssociatedSystemCapabilities"`
	IOConfig                                 SystemIOConfiguration               `xml:"AssociatedSystemIOConfiguration"`
	MemoryConfig                             SystemMemoryConfiguration           `xml:"AssociatedSystemMemoryConfiguration"`
	ProcessorConfig                          SystemProcessorConfiguration        `xml:"AssociatedSystemProcessorConfiguration"`
	SecurityConfig                           SystemSecurityConfiguration         `xml:"AssociatedSystemSecurity"`
	AssociatedVirtualIOServers               []LinkXML                           `xml:"AssociatedVirtualIOServers>link"`
	DetailedState                            string                              `xml:"DetailedState"`
	MTMS                                     MachineTypeModelAndSerialNumber     `xml:"MachineTypeModelAndSerialNumber"`
	ManufacturingDefaultConfigurationEnabled bool                                `xml:"ManufacturingDefaultConfigurationEnabled"`
	MaximumPartitions                        float64                                 `xml:"MaximumPartitions"`
	MaximumPowerControlPartitions            float64                                 `xml:"MaximumPowerControlPartitions"`
	SRIOVAdapters                            []SRIOVAdapter                      `xml:"SRIOVAdapters>IOAdapterChoice>SRIOVAdapter"`
	MaximumRemoteRestartPartitions           int                                 `xml:"MaximumRemoteRestartPartitions"`
	MaximumSharedProcessorCapablePartitionID int                                 `xml:"MaximumSharedProcessorCapablePartitionID"`
	MaximumSuspendablePartitions             int                                 `xml:"MaximumSuspendablePartitions"`
	MaximumBackingDevicesPerVNIC             int                                 `xml:"MaximumBackingDevicesPerVNIC"`
	PhysicalSystemAttentionLEDState          bool                                `xml:"PhysicalSystemAttentionLEDState"`
	VirtualSystemAttentionLEDState           bool                                `xml:"VirtualSystemAttentionLEDState"`
	PrimaryIPAddress                         string                              `xml:"PrimaryIPAddress"`
	Hostname                                 string                              `xml:"Hostname"`
	ServiceProcessorFailoverEnabled          bool                                `xml:"ServiceProcessorFailoverEnabled"`
	ServiceProcessorFailoverReason           string                              `xml:"ServiceProcessorFailoverReason"`
	ServiceProcessorFailoverState            string                              `xml:"ServiceProcessorFailoverState"`
	ServiceProcessorVersion                  string                              `xml:"ServiceProcessorVersion"`
	State                                    string                              `xml:"State"`
	StateDetail                              string                              `xml:"StateDetail"`
	SystemName                               string                              `xml:"SystemName"`
	SystemTime                               string                              `xml:"SystemTime"`
	MigrationInfo                            SystemMigrationInformation          `xml:"SystemMigrationInformation"`
	ReferenceCode                            string                              `xml:"ReferenceCode"`
	MergedReferenceCode                      string                              `xml:"MergedReferenceCode"`
	SystemFirmware                           string                              `xml:"SystemFirmware"`
	EnergyManagementConfig                   SystemEnergyManagementConfiguration `xml:"EnergyManagementConfiguration"`
	IsPowerVMManagementMaster                bool                                `xml:"IsPowerVMManagementMaster"`
	IsPowerVMManagementController            bool                                `xml:"IsPowerVMManagementController"`
	IsClassicHMCManagement                   bool                                `xml:"IsClassicHMCManagement"`
	IsPowerVMManagementWithoutMaster         bool                                `xml:"IsPowerVMManagementWithoutMaster"`
	IsPowerVMManagementWithoutController     bool                                `xml:"IsPowerVMManagementWithoutController"`
	IsManagementPartitionPowerVMManagementMaster bool                            `xml:"IsManagementPartitionPowerVMManagementMaster"`
	IsManagementPartitionPowerVMManagementController bool                        `xml:"IsManagementPartitionPowerVMManagementController"`
	IsHMCPowerVMManagementMaster             bool                                `xml:"IsHMCPowerVMManagementMaster"`
	IsHMCPowerVMManagementController         bool                                `xml:"IsHMCPowerVMManagementController"`
	IsNotPowerVMManagementMaster             bool                                `xml:"IsNotPowerVMManagementMaster"`
	IsNotPowerVMManagementController         bool                                `xml:"IsNotPowerVMManagementController"`
	IsPowerVMManagementNormalMaster          bool                                `xml:"IsPowerVMManagementNormalMaster"`
	IsPowerVMManagementNormalController      bool                                `xml:"IsPowerVMManagementNormalController"`
	IsPowerVMManagementPersistentMaster      bool                                `xml:"IsPowerVMManagementPersistentMaster"`
	IsPowerVMManagementPersistentController  bool                                `xml:"IsPowerVMManagementPersistentController"`
	IsPowerVMManagementTemporaryMaster       bool                                `xml:"IsPowerVMManagementTemporaryMaster"`
	IsPowerVMManagementTemporaryController   bool                                `xml:"IsPowerVMManagementTemporaryController"`
	IsPowerVMManagementPartitionEnabled      bool                                `xml:"IsPowerVMManagementPartitionEnabled"`
	HardwareAccelerators                     []HardwareAcceleratorType           `xml:"SupportedHardwareAcceleratorTypes>HardwareAcceleratorType"`
	CurrentStealableProcUnits                float64                             `xml:"CurrentStealableProcUnits"`
	CurrentStealableMemory                   float64                             `xml:"CurrentStealableMemory"`
	MinimumKeyStoreSize                      int                                 `xml:"MinimumKeyStoreSize"`
	MaximumkeyStoreSize                      int                                 `xml:"MaximumkeyStoreSize"`
	Uptime                                   string                              `xml:"Uptime"`
	Description                              string                              `xml:"Description"`
	SystemType                               string                              `xml:"SystemType"`
	ProcessorThrottling                      bool                                `xml:"ProcessorThrottling"`
	SupportedVTPMVersions                    string                              `xml:"SupportedVTPMVersions"`
	PersistentMemoryConfig                   SystemPersistentMemoryConfiguration `xml:"AssociatedPersistentMemoryConfiguration"`
	ASMGeneralPasswordUpdatedRequired        bool                                `xml:"ASMGeneralPasswordUpdatedRequired"`
	ASMAdminPasswordUpdatedRequired          bool                                `xml:"ASMAdminPasswordUpdatedRequired"`
	PlatformPasswordUpdateRequired           bool                                `xml:"PlatformPasswordUpdateRequired"`
}

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

type SystemCapabilities struct {
	ActiveLogicalPartitionMobilityCapable             bool `xml:"ActiveLogicalPartitionMobilityCapable"`
	ActiveLogicalPartitionSharedIdeProcessorsCapable  bool `xml:"ActiveLogicalPartitionSharedIdeProcessorsCapable"`
	ActiveMemoryDeduplicationCapable                  bool `xml:"ActiveMemoryDeduplicationCapable"`
	ActiveMemoryExpansionCapable                      bool `xml:"ActiveMemoryExpansionCapable"`
	ActiveMemoryMirroringCapable                      bool `xml:"ActiveMemoryMirroringCapable"`
	ActiveMemorySharingCapable                        bool `xml:"ActiveMemorySharingCapable"`
	AddressBroadcastPolicyCapable                     bool `xml:"AddressBroadcastPolicyCapable"`
	AIXCapable                                        string `xml:"AIXCapable"`
	AutorecoveryPowerOnCapable                        bool `xml:"AutorecoveryPowerOnCapable"`
	BarrierSynchronizationRegisterCapable             bool `xml:"BarrierSynchronizationRegisterCapable"`
	CapacityOnDemandMemoryCapable                     bool `xml:"CapacityOnDemandMemoryCapable"`
	CapacityOnDemandProcessorCapable                  bool `xml:"CapacityOnDemandProcessorCapable"`
	CapacityOnDemandOnOffProcessorCapable             bool `xml:"CapacityOnDemandOnOffProcessorCapable"`
	CapacityOnDemandOnOffMemoryCapable                bool `xml:"CapacityOnDemandOnOffMemoryCapable"`
	CapacityOnDemandTrialProcessorCapable             bool `xml:"CapacityOnDemandTrialProcessorCapable"`
	CapacityOnDemandTrialMemoryCapable                bool `xml:"CapacityOnDemandTrialMemoryCapable"`
	CapacityOnDemandUtilityProcessorCapable           bool `xml:"CapacityOnDemandUtilityProcessorCapable"`
	CAPICapable                                       bool `xml:"CAPICapable"`
	CustomLogicalPartitionPlacementCapable            bool `xml:"CustomLogicalPartitionPlacementCapable"`
	ElectronicErrorReportingCapable                   bool `xml:"ElectronicErrorReportingCapable"`
	ExternalIntrusionDetectionCapable                 bool `xml:"ExternalIntrusionDetectionCapable"`
	FirmwarePowerSaverCapable                         bool `xml:"FirmwarePowerSaverCapable"`
	HardwareDiscoveryCapable                          bool `xml:"HardwareDiscoveryCapable"`
	HardwareMemoryCompressionCapable                  bool `xml:"HardwareMemoryCompressionCapable"`
	HardwareMemoryEncryptionCapable                   bool `xml:"HardwareMemoryEncryptionCapable"`
	HardwarePowerSaverCapable                         bool `xml:"HardwarePowerSaverCapable"`
	HostChannelAdapterCapable                         bool `xml:"HostChannelAdapterCapable"`
	HugePageMemoryCapable                             bool `xml:"HugePageMemoryCapable"`
	HugePageMemoryOverrideCapable                     bool `xml:"HugePageMemoryOverrideCapable"`
	IBMiCapable                                       bool `xml:"IBMiCapable"`
	IBMiLogicalPartitionMobilityCapable               bool `xml:"IBMiLogicalPartitionMobilityCapable"`
	IBMiLogicalPartitionSuspendCapable                bool `xml:"IBMiLogicalPartitionSuspendCapable"`
	IBMiNetworkInstallCapable                         bool `xml:"IBMiNetworkInstallCapable"`
	IBMiRestrictedIOModeCapable                       bool `xml:"IBMiRestrictedIOModeCapable"`
	IBMiNetworkInstallVlanCapable                     bool `xml:"IBMiNetworkInstallVlanCapable"`
	InactiveLogicalPartitionMobilityCapable           bool `xml:"InactiveLogicalPartitionMobilityCapable"`
	IntelligentPlatformManagementInterfaceCapable     bool `xml:"IntelligentPlatformManagementInterfaceCapable"`
	LinuxCapable                                      bool `xml:"LinuxCapable"`
	LogicalHostEthernetAdapterCapable                 bool `xml:"LogicalHostEthernetAdapterCapable"`
	LogicalPartitionAffinityGroupCapable              bool `xml:"LogicalPartitionAffinityGroupCapable"`
	LogicalPartitionAvailabilityPriorityCapable       bool `xml:"LogicalPartitionAvailabilityPriorityCapable"`
	LogicalPartitionEnergyManagementCapable           bool `xml:"LogicalPartitionEnergyManagementCapable"`
	LogicalPartitionProcessorCompatibilityModeCapable bool `xml:"LogicalPartitionProcessorCompatibilityModeCapable"`
	LogicalPartitionRemoteRestartCapable              bool `xml:"LogicalPartitionRemoteRestartCapable"`
	LogicalPartitionSuspendCapable                    bool `xml:"LogicalPartitionSuspendCapable"`
	MemoryMirroringCapable                            bool `xml:"MemoryMirroringCapable"`
	MicroLogicalPartitionCapable                      bool `xml:"MicroLogicalPartitionCapable"`
	PowerVMLogicalPartitionSimplifiedRemoteRestartCapable bool `xml:"PowerVMLogicalPartitionSimplifiedRemoteRestartCapable"`
	RedundantErrorPathReportingCapable                bool `xml:"RedundantErrorPathReportingCapable"`
	RemoteRestartToggleCapable                        bool `xml:"RemoteRestartToggleCapable"`
	ServiceProcessorConcurrentMaintenanceCapable      bool `xml:"ServiceProcessorConcurrentMaintenanceCapable"`
	ServiceProcessorFailoverCapable                   bool `xml:"ServiceProcessorFailoverCapable"`
	ServiceProcessorAutonomicIPLCapable               bool `xml:"ServiceProcessorAutonomicIPLCapable"`
	SharedEthernetFailoverCapable                     bool `xml:"SharedEthernetFailoverCapable"`
	SharedProcessorPoolCapable                        bool `xml:"SharedProcessorPoolCapable"`
	SRIOVCapable                                      bool `xml:"SRIOVCapable"`
	SRIOVRoCECapable                                  bool `xml:"SRIOVRoCECapable"`
	SwitchNetworkInterfaceMessagePassingCapable       bool `xml:"SwitchNetworkInterfaceMessagePassingCapable"`
	SystemPartitionProcessorLimitCapable              bool `xml:"SystemPartitionProcessorLimitCapable"`
	Telnet5250ApplicationCapable                      bool `xml:"Telnet5250ApplicationCapable"`
	TurboCoreCapable                                  bool `xml:"TurboCoreCapable"`
	VirtualEthernetAdapterDynamicLogicalPartitionCapable bool `xml:"VirtualEthernetAdapterDynamicLogicalPartitionCapable"`
	VirtualEthernetQualityOfServiceCapable            bool `xml:"VirtualEthernetQualityOfServiceCapable"`
	VirtualFiberChannelCapable                        bool `xml:"VirtualFiberChannelCapable"`
	VirtualIOServerCapable                            bool `xml:"VirtualIOServerCapable"`
	VirtualizationEngineTechnologiesActivationCapable bool `xml:"VirtualizationEngineTechnologiesActivationCapable"`
	VirtualServerNetworkingPhase2Capable              bool `xml:"VirtualServerNetworkingPhase2Capable"`
	VirtualSwitchCapable                              bool `xml:"VirtualSwitchCapable"`
	VirtualTrustedPlatformModuleCapable               bool `xml:"VirtualTrustedPlatformModuleCapable"`
	VirtualTrustedPlatformModule20Capable             bool `xml:"VirtualTrustedPlatformModule20Capable"`
	VLANStatisticsCapable                             bool `xml:"VLANStatisticsCapable"`
	VirtualEthernetCustomMACAddressCapable            bool `xml:"VirtualEthernetCustomMACAddressCapable"`
	ManagementVLANForControlChannelCapable            bool `xml:"ManagementVLANForControlChannelCapable"`
	VirtualNICDedicatedSRIOVCapable                   bool `xml:"VirtualNICDedicatedSRIOVCapable"`
	VirtualNICSharedSRIOVCapable                      bool `xml:"VirtualNICSharedSRIOVCapable"`
	DynamicPlatformOptimizationCapable                bool `xml:"DynamicPlatformOptimizationCapable"`
	VirtualNICFailOverCapable                         bool `xml:"VirtualNICFailOverCapable"`
	AdvancedBootListSupportCapable                    bool `xml:"AdvancedBootListSupportCapable"`
	DynamicSimplifiedRemoteRestartToggleCapable       bool `xml:"DynamicSimplifiedRemoteRestartToggleCapable"`
	IBMiNativeIOCapable                               bool `xml:"IBMiNativeIOCapable"`
	CustomPhysicalPageTableRatioCapable               bool `xml:"CustomPhysicalPageTableRatioCapable"`
	HardwareAcceleratorCapable                        bool `xml:"HardwareAcceleratorCapable"`
	PlatformMemoryMirroringCapableIfLicensed          bool `xml:"PlatformMemoryMirroringCapableIfLicensed"`
	PlatformMemoryMirroringLicensed                   bool `xml:"PlatformMemoryMirroringLicensed"`
	PlatformMemoryMirroringCapabilityKnown            bool `xml:"PlatformMemoryMirroringCapabilityKnown"`
	PartitionSecureBootCapable                        bool `xml:"PartitionSecureBootCapable"`
	DedicatedProcessorPartitionCapable                bool `xml:"DedicatedProcessorPartitionCapable"`
	PersistentMemoryCapable                           bool `xml:"PersistentMemoryCapable"`
	SRIOVMigrationCapable                             bool `xml:"SRIOVMigrationCapable"`
	VirtualSerialNumberCapable                        bool `xml:"VirtualSerialNumberCapable"`
	CoDVSNCoExistCapable                              bool `xml:"CoDVSNCoExistCapable"`
	PartitionKeyStoreCapable                          bool `xml:"PartitionKeyStoreCapable"`
	IBMiHardwareAcceleratorCapable                    bool `xml:"IBMiHardwareAcceleratorCapable"`
	AIXUpdateAccessKeyCapable                         bool `xml:"AIXUpdateAccessKeyCapable"`
	NewPowerSavingModesNamesSupported                 bool `xml:"NewPowerSavingModesNamesSupported"`
	IBMiNetworkInstalliSCSICapable                    bool `xml:"IBMiNetworkInstalliSCSICapable"`
	PartitionDynamicKeySecureBootCapable              bool `xml:"PartitionDynamicKeySecureBootCapable"`
	SRIOVAdapterConfigOptionsCapable                  bool `xml:"SRIOVAdapterConfigOptionsCapable"`
	KvmOnPowerVMCapable                               bool `xml:"KvmOnPowerVMCapable"`
	MultipathNVMeCapable                              bool `xml:"MultipathNVMeCapable"`
}

type SystemIOConfiguration struct {
	AvailableWWPNs      string                    `xml:"AvailableWWPNs"`
	MaximumIOPools      int                       `xml:"MaximumIOPools"`
	WWPNPrefix          string                    `xml:"WWPNPrefix"`
	IOAdapters          []IOAdapterXML            `xml:"IOAdapters>IOAdapterChoice>IOAdapter"`
	IOBuses             []IOBusXML                `xml:"IOBuses>IOBus"`
	VirtualNetwork      SystemVirtualNetworkConfig `xml:"AssociatedSystemVirtualNetwork"`
	SRIOVAdapters   []SRIOVAdapter             `xml:"SRIOVAdapters>IOAdapterChoice>SRIOVAdapter"`
}

type SystemVirtualNetworkConfig struct {
	VirtualEthernetAdapterMACAddressPrefix string    `xml:"VirtualEthernetAdapterMACAddressPrefix"`
	NetworkBridges                         []LinkXML `xml:"NetworkBridges>link"`
	VirtualNetworks                        []LinkXML `xml:"VirtualNetworks>link"`
	VirtualSwitches                        []LinkXML `xml:"VirtualSwitches>link"`
}

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

type IOBusXML struct {
	IOBusID                   string      `xml:"IOBusID"`
	BackplanePhysicalLocation string      `xml:"BackplanePhysicalLocation"`
	ConnectorIndex            string      `xml:"BusDynamicReconfigurationConnectorIndex"`
	ConnectorName             string      `xml:"BusDynamicReconfigurationConnectorName"`
	IOSlots                   []IOSlotXML `xml:"IOSlots>IOSlot"`
}

type IOSlotXML struct {
	BusGroupingRequired      bool         `xml:"BusGroupingRequired"`
	Description              string       `xml:"Description"`
	FeatureCodes             []string     `xml:"FeatureCodes"`
	IOUnitPhysicalLocation   string       `xml:"IOUnitPhysicalLocation"`
	PartitionID              int          `xml:"PartitionID"`
	PartitionName            string       `xml:"PartitionName"`
	PartitionUUID            string       `xml:"PartitionUUID"`
	PartitionType            string       `xml:"PartitionType"`
	PCAdapterID              string       `xml:"PCAdapterID"`
	PCIClass                 string       `xml:"PCIClass"`
	PCIDeviceID              string       `xml:"PCIDeviceID"`
	PCISubsystemDeviceID     string       `xml:"PCISubsystemDeviceID"`
	PCIManufacturerID        string       `xml:"PCIManufacturerID"`
	PCIRevisionID            string       `xml:"PCIRevisionID"`
	PCIVendorID              string       `xml:"PCIVendorID"`
	PCISubsystemVendorID     string       `xml:"PCISubsystemVendorID"`
	RelatedIBMiIOSlot        IBMiIOSlot   `xml:"RelatedIBMiIOSlot"`
	RelatedIOAdapter         IOAdapterXML `xml:"RelatedIOAdapter>IOAdapter"`
	ConnectorIndex           string       `xml:"SlotDynamicReconfigurationConnectorIndex"`
	ConnectorName            string       `xml:"SlotDynamicReconfigurationConnectorName"`
	PhysicalLocationCode     string       `xml:"SlotPhysicalLocationCode"`
	SRIOVCapableDevice       bool         `xml:"SRIOVCapableDevice"`
	SRIOVCapableSlot         bool         `xml:"SRIOVCapableSlot"`
	SRIOVLogicalPortsLimit   int          `xml:"SRIOVLogicalPortsLimit"`
}

type IBMiIOSlot struct {
	AlternateLoadSourceAttached  bool `xml:"AlternateLoadSourceAttached"`
	ConsoleCapable               bool `xml:"ConsoleCapable"`
	DirectOperationsConsoleCapable bool `xml:"DirectOperationsConsoleCapable"`
	IOP                          bool `xml:"IOP"`
	IOPInfoStale                 bool `xml:"IOPInfoStale"`
	IOPoolID                     string `xml:"IOPoolID"`
	LANConsoleCapable            bool `xml:"LANConsoleCapable"`
	LoadSourceAttached           bool `xml:"LoadSourceAttached"`
	LoadSourceCapable            bool `xml:"LoadSourceCapable"`
	OperationsConsoleAttached    bool `xml:"OperationsConsoleAttached"`
	OperationsConsoleCapable     bool `xml:"OperationsConsoleCapable"`
}

type SystemMemoryConfiguration struct {
	AllowedHardwarePageTableRations         []string `xml:"AllowedHardwarePageTableRations"`
	AllowedMemoryDeduplicationTableRatios   string   `xml:"AllowedMemoryDeduplicationTableRatios"`
	AllowedMemoryRegionSize                 string   `xml:"AllowedMemoryRegionSize"`
	ConfigurableHugePages                   int      `xml:"ConfigurableHugePages"`
	ConfigurableSystemMemory                float64  `xml:"ConfigurableSystemMemory"`
	ConfiguredMirroredMemory                float64  `xml:"ConfiguredMirroredMemory"`
	CurrentAvailableHugePages               int      `xml:"CurrentAvailableHugePages"`
	CurrentAvailableMirroredMemory          float64  `xml:"CurrentAvailableMirroredMemory"`
	CurrentAvailableSystemMemory            float64  `xml:"CurrentAvailableSystemMemory"`
	CurrentLogicalMemoryBlockSize           int      `xml:"CurrentLogicalMemoryBlockSize"`
	CurrentMemoryMirroringMode              string   `xml:"CurrentMemoryMirroringMode"`
	CurrentMirroredMemory                   float64  `xml:"CurrentMirroredMemory"`
	DeconfiguredSystemMemory                float64  `xml:"DeconfiguredSystemMemory"`
	DefaultHardwarePageTableRatio           int      `xml:"DefaultHardwarePageTableRatio"`
	DefaultHardwarePagingTableRatioForDedicatedMemoryPartition int `xml:"DefaultHardwarePagingTableRatioForDedicatedMemoryPartition"`
	DefaultMemoryDeduplicationTableRatio    int      `xml:"DefaultMemoryDeduplicationTableRatio"`
	HugePageCount                           int      `xml:"HugePageCount"`
	HugePageSize                            int      `xml:"HugePageSize"`
	InstalledSystemMemory                   float64  `xml:"InstalledSystemMemory"`
	MaximumHugePages                        int      `xml:"MaximumHugePages"`
	MaximumMemoryPoolCount                  int      `xml:"MaximumMemoryPoolCount"`
	MaximumMirroredMemoryDefragmented       float64  `xml:"MaximumMirroredMemoryDefragmented"`
	MaximumPagingVirtualIOServersPerSharedMemoryPool int `xml:"MaximumPagingVirtualIOServersPerSharedMemoryPool"`
	MemoryDefragmentationState              string   `xml:"MemoryDefragmentationState"`
	MemoryMirroringState                    string   `xml:"MemoryMirroringState"`
	MemoryRegionSize                        int      `xml:"MemoryRegionSize"`
	MemoryUsedByHypervisor                  float64  `xml:"MemoryUsedByHypervisor"`
	MirrorableMemoryWithDefragmentation     float64  `xml:"MirrorableMemoryWithDefragmentation"`
	MirrorableMemoryWithoutDefragmentation  float64  `xml:"MirrorableMemoryWithoutDefragmentation"`
	MirroredMemoryUsedByHypervisor          float64  `xml:"MirroredMemoryUsedByHypervisor"`
	PendingAvailableHugePages               int      `xml:"PendingAvailableHugePages"`
	PendingAvailableSystemMemory            float64  `xml:"PendingAvailableSystemMemory"`
	PendingLogicalMemoryBlockSize           int      `xml:"PendingLogicalMemoryBlockSize"`
	PendingMemoryMirroringMode              string   `xml:"PendingMemoryMirroringMode"`
	PendingMemoryRegionSize                 int      `xml:"PendingMemoryRegionSize"`
	RequestedHugePages                      int      `xml:"RequestedHugePages"`
	TemporaryMemoryForLogicalPartitionMobilityInUse bool `xml:"TemporaryMemoryForLogicalPartitionMobilityInUse"`
	DefaultPhysicalPageTableRatio           int      `xml:"DefaultPhysicalPageTableRatio"`
	AllowedPhysicalPageTableRatios          []int    `xml:"AllowedPhysicalPageTableRatios"`
	PermanentSystemMemory                   float64  `xml:"PermanentSystemMemory"`
	CurrentAssignedMemoryToPartitions       float64  `xml:"CurrentAssignedMemoryToPartitions"`
	PendingLogicalMemoryRegionSize          int      `xml:"PendingLogicalMemoryRegionSize"`
}

type SystemProcessorConfiguration struct {
	ConfigurableSystemProcessorUnits               float64   `xml:"ConfigurableSystemProcessorUnits"`
	CurrentAvailableSystemProcessorUnits           float64   `xml:"CurrentAvailableSystemProcessorUnits"`
	CurrentMaximumProcessorsPerAIXOrLinuxPartition int       `xml:"CurrentMaximumProcessorsPerAIXOrLinuxPartition"`
	CurrentMaximumProcessorsPerIBMiPartition       int       `xml:"CurrentMaximumProcessorsPerIBMiPartition"`
	CurrentMaximumAllowedProcessorsPerPartition    int       `xml:"CurrentMaximumAllowedProcessorsPerPartition"`
	CurrentMaximumProcessorsPerVirtualIOServerPartition int  `xml:"CurrentMaximumProcessorsPerVirtualIOServerPartition"`
	CurrentMaximumVirtualProcessorsPerAIXOrLinuxPartition int `xml:"CurrentMaximumVirtualProcessorsPerAIXOrLinuxPartition"`
	CurrentMaximumVirtualProcessorsPerIBMiPartition int      `xml:"CurrentMaximumVirtualProcessorsPerIBMiPartition"`
	CurrentMaximumVirtualProcessorsPerVirtualIOServerPartition int `xml:"CurrentMaximumVirtualProcessorsPerVirtualIOServerPartition"`
	DeconfiguredSystemProcessorUnits               float64   `xml:"DeconfiguredSystemProcessorUnits"`
	InstalledSystemProcessorUnits                  float64   `xml:"InstalledSystemProcessorUnits"`
	MaximumProcessorUnitsPerIBMiPartition          float64   `xml:"MaximumProcessorUnitsPerIBMiPartition"`
	MaximumAllowedVirtualProcessorsPerPartition    int       `xml:"MaximumAllowedVirtualProcessorsPerPartition"`
	MinimumProcessorUnitsPerVirtualProcessor       float64   `xml:"MinimumProcessorUnitsPerVirtualProcessor"`
	NumberOfAllOSProcessorUnits                    float64   `xml:"NumberOfAllOSProcessorUnits"`
	NumberOfLinuxOnlyProcessorUnits                float64   `xml:"NumberOfLinuxOnlyProcessorUnits"`
	NumberOfLinuxOrVIOSOnlyProcessorUnits          float64   `xml:"NumberOfLinuxOrVIOSOnlyProcessorUnits"`
	NumberOfVirtualIOServerProcessorUnits          float64   `xml:"NumberOfVirtualIOServerProcessorUnits"`
	PendingAvailableSystemProcessorUnits           float64   `xml:"PendingAvailableSystemProcessorUnits"`
	SharedProcessorPoolCount                       int       `xml:"SharedProcessorPoolCount"`
	SupportedPartitionProcessorCompatibilityModes  []string  `xml:"SupportedPartitionProcessorCompatibilityModes"`
	TemporaryProcessorUnitsForLogicalPartitionMobilityInUse bool `xml:"TemporaryProcessorUnitsForLogicalPartitionMobilityInUse"`
	SharedProcessorPools                           []LinkXML `xml:"SharedProcessorPool>link"`
	PermanentSystemProcessors                      float64   `xml:"PermanentSystemProcessors"`
}

type SystemSecurityConfiguration struct {
	VirtualTrustedPlatformModuleKeyLength                  int `xml:"VirtualTrustedPlatformModuleKeyLength"`
	VirtualTrustedPlatformModuleKeyStatus                  int `xml:"VirtualTrustedPlatformModuleKeyStatus"`
	VirtualTrustedPlatformModuleVersion                    int `xml:"VirtualTrustedPlatformModuleVersion"`
	MaximumSupportedVirtualTrustedPlatformModulePartitions int `xml:"MaximumSupportedVirtualTrustedPlatformModulePartitions"`
	AvailableVirtualTrustedPlatformModulePartitions        int `xml:"AvailableVirtualTrustedPlatformModulePartitions"`
}

type SystemMigrationInformation struct {
	AllowInactiveSourceStorageVios                 string `xml:"AllowInactiveSourceStorageVios"`
	MaximumInactiveMigrations                      int    `xml:"MaximumInactiveMigrations"`
	MaximumActiveMigrations                        int    `xml:"MaximumActiveMigrations"`
	NumberOfInactiveMigrationsInProgress           int    `xml:"NumberOfInactiveMigrationsInProgress"`
	NumberOfActiveMigrationsInProgress             int    `xml:"NumberOfActiveMigrationsInProgress"`
	MaximumFirmwareActiveMigrations                int    `xml:"MaximumFirmwareActiveMigrations"`
	LogicalPartitionAffinityCheckCapable           bool   `xml:"LogicalPartitionAffinityCheckCapable"`
	MaximumFirmwareInactiveMigrations              int    `xml:"MaximumFirmwareInactiveMigrations"`
	InactiveLogicalPartitionMigrationCapable       bool   `xml:"InactiveLogicalPartitionMigrationCapable"`
	ActiveLogicalPartitionMigrationCapable         bool   `xml:"ActiveLogicalPartitionMigrationCapable"`
	IBMiLogicalPartitionMigrationCapable           bool   `xml:"IBMiLogicalPartitionMigrationCapable"`
	LogicalPartitionPersistentMemoryMigrationCapable bool `xml:"LogicalPartitionPersistentMemoryMigrationCapable"`
	LogicalPartitionRedundantMspsMigrationCapable  bool   `xml:"LogicalPartitionRedundantMspsMigrationCapable"`
	LogicalPartitionVSwitchChangeMigrationCapable  bool   `xml:"LogicalPartitionVSwitchChangeMigrationCapable"`
	LogicalPartitionSRIOVMigrationCapable          bool   `xml:"LogicalPartitionSRIOVMigrationCapable"`
	NPIVValidationPolicy                           string `xml:"NPIVValidationPolicy"`
	InactiveProfileMigrationPolicy                 string `xml:"InactiveProfileMigrationPolicy"`
}

type SystemEnergyManagementConfiguration struct {
	CurrentPowerSavingMode        string                     `xml:"CurrentPowerSavingMode"`
	RequiredPowerSavingMode       string                     `xml:"RequiredPowerSavingMode"`
	SupportedPowerSavingModeTypes []string                   `xml:"SupportedPowerSavingModeTypes"`
	IdlePowerSaverMode            bool                       `xml:"IdlePowerSaverMode"`
	DynamicPowerSavingTunables    PowerSavingTunablesDynamic `xml:"DynamicPowerSavingTunables"`
	IdlePowerSavingTunables       PowerSavingTunablesIdle    `xml:"IdlePowerSavingTunables"`
}

type PowerSavingTunablesDynamic struct {
	UtilizationThresholdForIncreasingFrequency string `xml:"UtilizationThresholdForIncreasingFrequency"`
	UtilizationThresholdForDecreasingFrequency string `xml:"UtilizationThresholdForDecreasingFrequency"`
	SamplesForComputingUtilzationStatistics    int    `xml:"SamplesForComputingUtilzationStatistics"`
	StepSizeForGoingUpInFrequency              string `xml:"StepSizeForGoingUpInFrequency"`
	StepSizeForGoingDownInFrequency            string `xml:"StepSizeForGoingDownInFrequency"`
	DeltaPercentageForDeterminingActiveCores   string `xml:"DeltaPercentageForDeterminingActiveCores"`
	UtilizationThresholdToDetermineActiveCoresWithSlack string `xml:"UtilizationThresholdToDetermineActiveCoresWithSlack"`
	CoreFrequencyDeltaState                    bool   `xml:"CoreFrequencyDeltaState"`
	CoreMaximumDeltaFrequency                  string `xml:"CoreMaximumDeltaFrequency"`
	MinimumUtilizationThresholdForIncreasingFrequency string `xml:"MinimumUtilizationThresholdForIncreasingFrequency"`
	MinimumUtilizationThresholdForDecreasingFrequency string `xml:"MinimumUtilizationThresholdForDecreasingFrequency"`
	MinimumSamplesForComputingUtilzationStatistics int `xml:"MinimumSamplesForComputingUtilzationStatistics"`
	MinimumStepSizeForGoingUpInFrequency         string `xml:"MinimumStepSizeForGoingUpInFrequency"`
	MinimumStepSizeForGoingDownInFrequency       string `xml:"MinimumStepSizeForGoingDownInFrequency"`
	MinimumDeltaPercentageForDeterminingActiveCores string `xml:"MinimumDeltaPercentageForDeterminingActiveCores"`
	MinimumUtilizationThresholdToDetermineActiveCoresWithSlack string `xml:"MinimumUtilizationThresholdToDetermineActiveCoresWithSlack"`
	MinimumCoreMaximumDeltaFrequency             string `xml:"MinimumCoreMaximumDeltaFrequency"`
	MaximumUtilizationThresholdForIncreasingFrequency string `xml:"MaximumUtilizationThresholdForIncreasingFrequency"`
	MaximumUtilizationThresholdForDecreasingFrequency string `xml:"MaximumUtilizationThresholdForDecreasingFrequency"`
	MaximumSamplesForComputingUtilzationStatistics int `xml:"MaximumSamplesForComputingUtilzationStatistics"`
	MaximumStepSizeForGoingUpInFrequency         string `xml:"MaximumStepSizeForGoingUpInFrequency"`
	MaximumStepSizeForGoingDownInFrequency       string `xml:"MaximumStepSizeForGoingDownInFrequency"`
	MaximumDeltaPercentageForDeterminingActiveCores string `xml:"MaximumDeltaPercentageForDeterminingActiveCores"`
	MaximumUtilizationThresholdToDetermineActiveCoresWithSlack string `xml:"MaximumUtilizationThresholdToDetermineActiveCoresWithSlack"`
	MaximumCoreMaximumDeltaFrequency             string `xml:"MaximumCoreMaximumDeltaFrequency"`
}

type PowerSavingTunablesIdle struct {
	DelayTimeToEnterIdlePower                    int    `xml:"DelayTimeToEnterIdlePower"`
	DelayTimeToExitIdlePower                     int    `xml:"DelayTimeToExitIdlePower"`
	UtilizationThresholdToEnterIdlePower         string `xml:"UtilizationThresholdToEnterIdlePower"`
	UtilizationThresholdToExitIdlePower          string `xml:"UtilizationThresholdToExitIdlePower"`
	MinimumDelayTimeToEnterIdlePower             int    `xml:"MinimumDelayTimeToEnterIdlePower"`
	MinimumDelayTimeToExitIdlePower              int    `xml:"MinimumDelayTimeToExitIdlePower"`
	MinimumUtilizationThresholdToEnterIdlePower  string `xml:"MinimumUtilizationThresholdToEnterIdlePower"`
	MinimumUtilizationThresholdToExitIdlePower   string `xml:"MinimumUtilizationThresholdToExitIdlePower"`
	MaximumDelayTimeToEnterIdlePower             int    `xml:"MaximumDelayTimeToEnterIdlePower"`
	MaximumDelayTimeToExitIdlePower              int    `xml:"MaximumDelayTimeToExitIdlePower"`
	MaximumUtilizationThresholdToEnterIdlePower  string `xml:"MaximumUtilizationThresholdToEnterIdlePower"`
	MaximumUtilizationThresholdToExitIdlePower   string `xml:"MaximumUtilizationThresholdToExitIdlePower"`
}

type HardwareAcceleratorType struct {
	Type                string `xml:"Type"`
	TotalQoS            int    `xml:"TotalQoS"`
	CurrentAvailableQoS int    `xml:"CurrentAvailableQoS"`
	PendingAvailableQoS int    `xml:"PendingAvailableQoS"`
}

type SystemPersistentMemoryConfiguration struct {
	MaximumPersistentMemoryVolumes          int    `xml:"MaximumPersistentMemoryVolumes"`
	CurrentPersistentMemoryVolumes          int    `xml:"CurrentPersistentMemoryVolumes"`
	MaximumAixLinuxPersistentMemoryVolumes  int    `xml:"MaximumAixLinuxPersistentMemoryVolumes"`
	MaximumOS400PersistentMemoryVolumes     int    `xml:"MaximumOS400PersistentMemoryVolumes"`
	MaximumVIOSPersistentMemoryVolumes      int    `xml:"MaximumVIOSPersistentMemoryVolumes"`
	DramPersistentMemoryVolumeBlockSize     int    `xml:"DramPersistentMemoryVolumeBlockSize"`
	DramPersistentMemoryVolumesSize         int    `xml:"DramPersistentMemoryVolumesSize"`
	DramPersistentMemoryVolumesCurrentSize  int    `xml:"DramPersistentMemoryVolumesCurrentSize"`
	SupportedPersistentMemoryDeviceTypes    string `xml:"SupportedPersistentMemoryDeviceTypes"`
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
	AdapterID     string              `xml:"AdapterID"`
	LocationCode  string              `xml:"PhysicalLocation"`
	AdapterMode   string              `xml:"AdapterMode"`  
	AdapterState  string              `xml:"AdapterState"` 
	IsFunctional  string              `xml:"IsFunctional"` 
	Description   string              `xml:"Description"`
	
	// ✨ Here is the slice linking to the struct above! ✨
	// Go will use the ">" syntax to step through the wrapper tag automatically.
	PhysicalPorts []SRIOVPhysicalPort `xml:"SRIOVEthernetPhysicalPorts>SRIOVEthernetPhysicalPort"`
}

// SRIOVLogicalPortRequest represents the payload required to provision an SR-IOV port.
// ⚠️ WARNING: IBM HMC schema strictly enforces the exact sequence defined in their XSD.
// The order below matches the precise schema sequence expected by the HMC.
type SRIOVLogicalPortRequest struct {
	XMLName                      xml.Name `xml:"SRIOVEthernetLogicalPort"`
	SchemaVersion                string   `xml:"schemaVersion,attr"`
	XMLNS                        string   `xml:"xmlns,attr"`

	// --- STRICT SCHEMA SEQUENCE ORDER ---
	AdapterID                    string   `xml:"AdapterID"`
	IsPromiscous                 *string  `xml:"IsPromiscous,omitempty"` // Note IBM's spelling
	ConfiguredCapacity           *string  `xml:"ConfiguredCapacity,omitempty"`
	PhysicalPortID               string   `xml:"PhysicalPortID"`
	PortVLANID                   *string  `xml:"PortVLANID,omitempty"`
	AllowedMACAddresses          *string  `xml:"AllowedMACAddresses,omitempty"`
	IEEE8021QAllowablePriorities *string  `xml:"IEEE8021QAllowablePriorities,omitempty"`
	AllowedVLANs                 *string  `xml:"AllowedVLANs,omitempty"`
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
	XMLName        xml.Name `xml:"LogicalPartition"`
	SchemaVersion  string   `xml:"schemaVersion,attr"` // Captures the version attribute

	// --- Fixed Metadata Mapping ---
	MetadataID      string `xml:"Metadata>Atom>AtomID"`
	MetadataCreated string `xml:"Metadata>Atom>AtomCreated"`

	// --- Fixed Uptime Mapping (To capture the 'group' attribute) ---
	Uptime struct {
		Value float64 `xml:",chardata"`
		Group string  `xml:"group,attr"`
	} `xml:"Uptime"`

	AllowPerformanceDataCollection       bool    `xml:"AllowPerformanceDataCollection"`
	AssociatedPartitionProfile           LinkXML `xml:"AssociatedPartitionProfile"`
	DefaultProfileName                   string  `xml:"DefaultProfileName"`
	AvailabilityPriority                 int     `xml:"AvailabilityPriority"`
	CurrentProcessorCompatibilityMode    string  `xml:"CurrentProcessorCompatibilityMode"`
	CurrentProfileSync                   string  `xml:"CurrentProfileSync"`
	IsBootable                           bool    `xml:"IsBootable"`
	IsConnectionMonitoringEnabled        bool    `xml:"IsConnectionMonitoringEnabled"`
	IsOperationInProgress                bool    `xml:"IsOperationInProgress"`
	IsRedundantErrorPathReportingEnabled bool    `xml:"IsRedundantErrorPathReportingEnabled"`
	IsTimeReferencePartition             bool    `xml:"IsTimeReferencePartition"`
	IsVirtualServiceAttentionLEDOn       bool    `xml:"IsVirtualServiceAttentionLEDOn"`
	IsVirtualTrustedPlatformModuleEnabled bool   `xml:"IsVirtualTrustedPlatformModuleEnabled"`
	KeylockPosition                      string  `xml:"KeylockPosition"`
	LogicalSerialNumber                  string  `xml:"LogicalSerialNumber"`
	OperatingSystemVersion               string  `xml:"OperatingSystemVersion"`
	PartitionCapabilities                LparCapabilities `xml:"PartitionCapabilities"`
	PartitionID                          int     `xml:"PartitionID"`
	PartitionIOConfiguration             LparIOConfiguration `xml:"PartitionIOConfiguration"`
	PartitionMemoryConfiguration         LparMemoryConfiguration `xml:"PartitionMemoryConfiguration"`
	PartitionName                        string  `xml:"PartitionName"`
	PartitionProcessorConfiguration      LparProcessorConfiguration `xml:"PartitionProcessorConfiguration"`
	PartitionProfiles                    []LinkXML `xml:"PartitionProfiles>link"`
	PartitionState                       string  `xml:"PartitionState"`
	PartitionType                        string  `xml:"PartitionType"`
	PartitionUUID                        string  `xml:"PartitionUUID"`
	PendingProcessorCompatibilityMode    string  `xml:"PendingProcessorCompatibilityMode"`
	ProcessorPool                        LinkXML `xml:"ProcessorPool"`

	// --- Fixed: Changed to float64 to prevent unmarshal errors with scientific notation ---
	ProgressPartitionDataRemaining float64 `xml:"ProgressPartitionDataRemaining"`
	ProgressPartitionDataTotal     float64 `xml:"ProgressPartitionDataTotal"`

	ResourceMonitoringControlState string `xml:"ResourceMonitoringControlState"`
	ResourceMonitoringIPAddress    string `xml:"ResourceMonitoringIPAddress"`
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

	MACAddressPrefix           string                    `xml:"MACAddressPrefix"`
	IsServicePartition         bool                      `xml:"IsServicePartition"`
	PowerVMManagementCapable   bool                      `xml:"PowerVMManagementCapable"`
	ReferenceCode              string                    `xml:"ReferenceCode"`
	AssignAllResources         bool                      `xml:"AssignAllResources"`
	HardwareAcceleratorQoS     HardwareAcceleratorQoSXML `xml:"HardwareAcceleratorQoS"`
	LastActivatedProfile       string                    `xml:"LastActivatedProfile"`
	HasPhysicalIO              bool                      `xml:"HasPhysicalIO"`
	OperatingSystemType        string                    `xml:"OperatingSystemType"`
	PendingSecureBoot          int                       `xml:"PendingSecureBoot"`
	CurrentSecureBoot          int                       `xml:"CurrentSecureBoot"`
	KeyStoreSize               int                       `xml:"KeyStoreSize"`
	BootMode                   string                    `xml:"BootMode"`
	SystemName                 string                    `xml:"SystemName"`
	PowerOnWithHypervisor      bool                      `xml:"PowerOnWithHypervisor"`
	PersistentMemoryConfiguration LparPersistentMemoryConfiguration `xml:"AssociatedPersistentMemoryConfiguration"`
	MigrationStorageViosDataStatus    string `xml:"MigrationStorageViosDataStatus"`
	MigrationStorageViosDataTimestamp string `xml:"MigrationStorageViosDataTimestamp"`
	RemoteRestartCapable              bool   `xml:"RemoteRestartCapable"`
	SimplifiedRemoteRestartCapable    bool   `xml:"SimplifiedRemoteRestartCapable"`
	HasDedicatedProcessorsForMigration bool   `xml:"HasDedicatedProcessorsForMigration"`
	SuspendCapable                    bool   `xml:"SuspendCapable"`
	MigrationDisable                  bool   `xml:"MigrationDisable"`
	MigrationState                    string `xml:"MigrationState"`
	RemoteRestartState                string `xml:"RemoteRestartState"`
	BootListInformation               LparBootListInformation `xml:"BootListInformation"`
	VirtualSerialNumber               string `xml:"VirtualSerialNumber"`
	KvmCapable                        bool   `xml:"KvmCapable"`
}

type HardwareAcceleratorQoSXML struct {
	// Captures the parent element
}

type LparCapabilities struct {
	DynamicLogicalPartitionIOCapable                        bool `xml:"DynamicLogicalPartitionIOCapable"`
	DynamicLogicalPartitionMemoryCapable                    bool `xml:"DynamicLogicalPartitionMemoryCapable"`
	DynamicLogicalPartitionVIOSCapable                      bool `xml:"DynamicLogicalPartitionVIOSCapable"`
	DynamicLogicalPartitionProcessorCapable                 bool `xml:"DynamicLogicalPartitionProcessorCapable"`
	InternalAndExternalIntrusionDetectionCapable            bool `xml:"InternalAndExternalIntrusionDetectionCapable"`
	ResourceMonitoringControlOperatingSystemShutdownCapable bool `xml:"ResourceMonitoringControlOperatingSystemShutdownCapable"`
}

type LparIOConfiguration struct {
	MaximumVirtualIOSlots        int       `xml:"MaximumVirtualIOSlots"`
	CurrentMaximumVirtualIOSlots int       `xml:"CurrentMaximumVirtualIOSlots"`
	ProfileIOSlots               []LinkXML `xml:"ProfileIOSlots>link"`
}

type LparMemoryConfiguration struct {
	ActiveMemoryExpansionEnabled          bool    `xml:"ActiveMemoryExpansionEnabled"`
	ActiveMemorySharingEnabled            bool    `xml:"ActiveMemorySharingEnabled"`
	DesiredHugePageCount                  int     `xml:"DesiredHugePageCount"`
	DesiredMemory                         float64 `xml:"DesiredMemory"`
	ExpansionFactor                       float64 `xml:"ExpansionFactor"`
	HardwarePageTableRatio                int     `xml:"HardwarePageTableRatio"`
	MaximumHugePageCount                  int     `xml:"MaximumHugePageCount"`
	MaximumMemory                         float64 `xml:"MaximumMemory"`
	MinimumHugePageCount                  int     `xml:"MinimumHugePageCount"`
	MinimumMemory                         float64 `xml:"MinimumMemory"`
	CurrentExpansionFactor                float64 `xml:"CurrentExpansionFactor"`
	CurrentHardwarePageTableRatio         int     `xml:"CurrentHardwarePageTableRatio"`
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
	PhysicalPageTableRatio                int     `xml:"PhysicalPageTableRatio"`
}

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

type LparCurrentSharedProcessorConfiguration struct {
	AllocatedVirtualProcessors      float64 `xml:"AllocatedVirtualProcessors"` // Changed to float64
	CurrentMaximumProcessingUnits   float64 `xml:"CurrentMaximumProcessingUnits"`
	CurrentMinimumProcessingUnits   float64 `xml:"CurrentMinimumProcessingUnits"`
	CurrentProcessingUnits          float64 `xml:"CurrentProcessingUnits"`
	CurrentSharedProcessorPoolID    int     `xml:"CurrentSharedProcessorPoolID"`
	CurrentUncappedWeight           float64 `xml:"CurrentUncappedWeight"`      // Changed to float64
	CurrentMinimumVirtualProcessors int     `xml:"CurrentMinimumVirtualProcessors"`
	CurrentMaximumVirtualProcessors int     `xml:"CurrentMaximumVirtualProcessors"`
	RuntimeProcessingUnits          float64 `xml:"RuntimeProcessingUnits"`
	RuntimeUncappedWeight           float64 `xml:"RuntimeUncappedWeight"`      // Changed to float64
}

type LparDedicatedProcessorConfiguration struct {
	CurrentProcessors float64 `xml:"CurrentProcessors"`
	DesiredProcessors float64 `xml:"DesiredProcessors"`
	MaximumProcessors float64 `xml:"MaximumProcessors"`
	MinimumProcessors float64 `xml:"MinimumProcessors"`
}

type LparPersistentMemoryConfiguration struct {
	MaximumPersistentMemoryVolumes     int `xml:"MaximumPersistentMemoryVolumes"`
	CurrentPersistentMemoryVolumes     int `xml:"CurrentPersistentMemoryVolumes"`
	MaximumDramPersistentMemoryVolumes int `xml:"MaximumDramPersistentMemoryVolumes"`
	CurrentDramPersistentMemoryVolumes int `xml:"CurrentDramPersistentMemoryVolumes"`
}

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
	IsPromiscuous      bool   `xml:"IsPromiscous"`       // Note IBM's spelling "IsPromiscous" without the 'u'
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
