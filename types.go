package hmc

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
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
	XMLName xml.Name `xml:"JobResponse"`
	JobID   string   `xml:"JobID"`
	Status  string   `xml:"Status"`
}

// Logger with prefix for HMC operations
var hmcLogger = log.New(log.Writer(), "[HMC] ", log.LstdFlags)

// HmcRestClient represents the REST client for HMC operations
type HmcRestClient struct {
	hmcIP   string
	session string
	client  *http.Client
}

// NewHmcRestClient initializes a new HmcRestClient with an insecure TLS HTTP client
func NewHmcRestClient(hmcIP string) *HmcRestClient {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
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

// VIOS represents a Virtual I/O Server
type VIOS struct {
	UUID          string `json:"UUID"`
	PartitionName string `json:"PartitionName"`
	RMCState      string `json:"RMCState"`
}

// PhysicalVolume represents a physical volume
type PhysicalVolume struct {
	Description               string `xml:"Description"`
	LocationCode              string `xml:"LocationCode"`
	PersistentReserveKeyValue string `xml:"PersistentReserveKeyValue"`
	ReservePolicy             string `xml:"ReservePolicy"`
	ReservePolicyAlgorithm    string `xml:"ReservePolicyAlgorithm"`
	UniqueDeviceID            string `xml:"UniqueDeviceID"`
	AvailableForUsage         bool   `xml:"AvailableForUsage"`
	VolumeCapacity            int64  `xml:"VolumeCapacity"`
	VolumeName                string `xml:"VolumeName"`
	VolumeState               string `xml:"VolumeState"`
	VolumeUniqueID            string `xml:"VolumeUniqueID"`
	IsFibreChannelBacked      bool   `xml:"IsFibreChannelBacked"`
	IsISCSIBacked             bool   `xml:"IsISCSIBacked"`
	StorageLabel              string `xml:"StorageLabel"`
	DescriptorPage83          string `xml:"DescriptorPage83"`
}

// LogicalPartitionQuick represents the structure of a partition in the quick list
type LogicalPartitionQuick struct {
	ProgressState                  *string `json:"ProgressState"`
	Description                    *string `json:"Description"`
	MemoryMode                     string  `json:"MemoryMode"`
	MigrationState                 string  `json:"MigrationState"`
	PowerManagementMode            *string `json:"PowerManagementMode"`
	OperatingSystemVersion         string  `json:"OperatingSystemVersion"`
	PartitionID                    int     `json:"PartitionID"`
	IsVirtualServiceAttentionLEDOn string  `json:"IsVirtualServiceAttentionLEDOn"`
	AllocatedVirtualProcessors     int     `json:"AllocatedVirtualProcessors"`
	PartitionState                 string  `json:"PartitionState"`
	ResourceMonitoringIPAddress    *string `json:"ResourceMonitoringIPAddress"`
	HasPhysicalIO                  string  `json:"HasPhysicalIO"`
	SystemName                     string  `json:"SystemName"`
	SharingMode                    string  `json:"SharingMode"`
	MigrationDisable               bool    `json:"MigrationDisable"`
	CurrentProcessors              int     `json:"CurrentProcessors"`
	LastActivatedProfile           string  `json:"LastActivatedProfile"`
	CurrentUncappedWeight          int     `json:"CurrentUncappedWeight"`
	RemoteRestartState             string  `json:"RemoteRestartState"`
	PartitionType                  string  `json:"PartitionType"`
	PartitionName                  string  `json:"PartitionName"`
	RMCState                       string  `json:"RMCState"`
	OperatingSystemType            string  `json:"OperatingSystemType"`
	CurrentMemory                  int     `json:"CurrentMemory"`
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

// ManagedSystemQuick represents the structure of a managed system in the quick list
type ManagedSystemQuick struct {
	SystemName            string      `json:"SystemName"`
	UUID                  string      `json:"UUID"`
	State                 string      `json:"State"`
	IPAddress             string      `json:"IPAddress"`
	MTMS                  string      `json:"MTMS"`
	SystemType            string      `json:"SystemType"`
	SystemFirmware        string      `json:"SystemFirmware"`
	MaximumPartitions     int         `json:"MaximumPartitions"`
	InstalledSystemMemory int         `json:"InstalledSystemMemory"`
	InstalledSystemProcessors float64 `json:"InstalledSystemProcessorUnits"` // Use float64 for scientific notation like 6E+1
	CurrentAvailableMemory     float64 `json:"CurrentAvailableSystemMemory"`
	CurrentAvailableProcessors float64 `json:"CurrentAvailableSystemProcessorUnits"`
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

// =====================================================================
// DATA STRUCTURES FOR COMPREHENSIVE VIOS DETAILS
// =====================================================================

// VirtualIOServerDetails represents detailed information about a Virtual I/O Server.
type VirtualIOServerDetails struct {
	UUID                        string
	PartitionID                 string
	PartitionName               string
	PartitionState              string
	PartitionType               string
	SystemName                  string
	OperatingSystemVersion      string
	ResourceMonitoringIPAddress string
	LogicalSerialNumber         string
	IsBootable                  string
	Uptime                      string

	Memory    VIOSMemoryConfig
	Processor VIOSProcessorConfig
	Storage   VIOSStorageConfig
	Network   VIOSNetworkConfig
}

type VIOSMemoryConfig struct {
	DesiredMemory string
	MaximumMemory string
	MinimumMemory string
}

type VIOSProcessorConfig struct {
	HasDedicatedProcessors string
	SharingMode            string
	DesiredProcessors      string
	MaximumProcessors      string
	MinimumProcessors      string
}

type VIOSStorageConfig struct {
	PhysicalVolumes   []VIOSPhysicalVolume
	VFCMappings       []VIOSVFCMapping
	FibreChannelPorts []VIOSFibreChannelPort
}

type VIOSPhysicalVolume struct {
	VolumeName     string
	VolumeCapacity string
	VolumeState    string
	UniqueDeviceID string
	LocationCode   string
}

type VIOSFibreChannelPort struct {
	PortName     string
	LocationCode string
	WWPN         string
	WWNN         string
}

type VIOSVFCMapping struct {
	ServerAdapterSlot string
	ClientPartitionID string
	ClientAdapterSlot string
	MapPort           string
	PortWWPN          string
	PortWWNN          string
}

type VIOSNetworkConfig struct {
	SharedEthernetAdapters []VIOSSharedEthernetAdapter
	TrunkAdapters          []VIOSTrunkAdapter
}

type VIOSSharedEthernetAdapter struct {
	DeviceName         string
	HighAvailability   string
	PortVLANID         string
	BackingDevice      string
	ConfigurationState string
}

type VIOSTrunkAdapter struct {
	DeviceName        string
	MACAddress        string
	PortVLANID        string
	VirtualSlotNumber string
}


// VirtualSCSIServerAdapter represents a Virtual SCSI Server Adapter (vhost) on a VIOS.
type VirtualSCSIServerAdapter struct {
	UUID                                string
	AdapterURI                          string // The direct URL to this specific adapter (useful for DELETE operations)
	AdapterType                         string
	DynamicReconfigurationConnectorName string
	LocationCode                        string
	LocalPartitionID                    string
	RequiredAdapter                     string
	VariedOn                            string
	VirtualSlotNumber                   string
	RemoteLogicalPartitionID            string
	RemoteSlotNumber                    string
}

// =====================================================================
// VOLUME GROUP DATA STRUCTURES
// =====================================================================

// VolumeGroup represents a Volume Group configured on a Virtual I/O Server.
type VolumeGroup struct {
	UUID                  string
	GroupName             string
	AvailableSize         string
	FreeSpace             string
	GroupCapacity         string
	GroupSerialID         string
	MaximumLogicalVolumes string
	UniqueDeviceID        string
	PhysicalVolumes       []VGPhysicalVolume
	OpticalMedia          []VirtualOpticalMedia
	VirtualDisks          []VirtualDisk
	HasMediaRepository    bool
	MediaRepositoryName   string // NEW: Name of the repository (e.g., "VMLibrary")
	MediaRepositorySize   string // NEW: Size of the repository in GB
}

// VirtualDisk represents a Logical Volume created inside the Volume Group.
type VirtualDisk struct {
	DiskName       string
	DiskCapacity   string // Note: The HMC API usually returns this in GB (e.g., "10")
	DiskLabel      string
	UniqueDeviceID string
}

// VGPhysicalVolume represents a physical disk associated with a Volume Group.
type VGPhysicalVolume struct {
	VolumeName             string
	VolumeCapacity         string
	VolumeState            string
	UniqueDeviceID         string
	VolumeUniqueID         string
	LocationCode           string
	Description            string
	IsFibreChannelBacked   string
	ReservePolicy          string // NEW
	ReservePolicyAlgorithm string // NEW
	AvailableForUsage      string // NEW
	IsISCSIBacked          string // NEW
	StorageLabel           string // NEW
	DescriptorPage83       string // NEW
}

// VirtualOpticalMedia represents an ISO/media file stored in the Volume Group's media repository.
type VirtualOpticalMedia struct {
	MediaName string
	MediaUDID string
	MountType string
	Size      string
}

// =====================================================================
// VIOS SCSI MAPPING DATA STRUCTURES (FULL)
// =====================================================================

// ViosSCSIMappingDetails represents a complete end-to-end VSCSI mapping.
type ViosSCSIMappingDetails struct {
	AssociatedLparURI string
	ClientAdapter     VSCSIClientAdapter
	ServerAdapter     VSCSIServerAdapter
	Storage           VSCSIStorage
	TargetDevice      VSCSITargetDevice
}

// VSCSIClientAdapter holds all properties of the client-side adapter (LPAR side).
type VSCSIClientAdapter struct {
	AdapterType                         string
	DynamicReconfigurationConnectorName string
	LocationCode                        string
	LocalPartitionID                    string
	RequiredAdapter                     string
	VariedOn                            string
	VirtualSlotNumber                   string
	RemoteLogicalPartitionID            string
	RemoteSlotNumber                    string
	ServerLocationCode                  string
}

// VSCSIServerAdapter holds all properties of the server-side adapter (VIOS vhost).
type VSCSIServerAdapter struct {
	AdapterType                         string
	DynamicReconfigurationConnectorName string
	LocationCode                        string
	LocalPartitionID                    string
	RequiredAdapter                     string
	VariedOn                            string
	VirtualSlotNumber                   string
	AdapterName                         string // e.g., "vhost3"
	BackingDeviceName                   string // e.g., "hdisk3" or "vopt_..."
	RemoteLogicalPartitionID            string
	RemoteSlotNumber                    string
	ServerLocationCode                  string
	UniqueDeviceID                      string
}

// VSCSIStorage holds properties for either Physical Volumes or Virtual Optical Media.
type VSCSIStorage struct {
	StorageType               string // "PhysicalVolume" or "VirtualOpticalMedia"
	
	// Virtual Optical Media Fields
	MediaName                 string
	MediaUDID                 string
	MountType                 string
	Size                      string
	
	// Physical Volume Fields
	Description               string
	LocationCode              string
	PersistentReserveKeyValue string
	ReservePolicy             string
	ReservePolicyAlgorithm    string
	UniqueDeviceID            string
	AvailableForUsage         string
	VolumeCapacity            string
	VolumeName                string
	VolumeState               string
	VolumeUniqueID            string
	IsFibreChannelBacked      string
	IsISCSIBacked             string
	StorageLabel              string
	DescriptorPage83          string
}

// VSCSITargetDevice holds properties for the virtual target device (vtd).
type VSCSITargetDevice struct {
	DeviceType         string // "VirtualOpticalTargetDevice" or "PhysicalVolumeVirtualTargetDevice"
	LogicalUnitAddress string
	TargetName         string // e.g., "vtscsi0" or "vtopt1"
	UniqueDeviceID     string
}