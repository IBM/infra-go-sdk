package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
	"github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/communication"
)

// DNS-only configuration template
const dnsConfigTemplate = `# ============================================
# DNS Configuration for Cluster: {{.ClusterName}}
# Type: {{.Type}}
# OCP Version: {{.OCPVersion}}
# Generated: {{.Timestamp}}
# ============================================

# DNS A Records for cluster nodes
{{- range .Nodes}}
address=/{{.Hostname}}/{{.IP}}
{{- end}}

# DNS A Records for OpenShift services
address=/api.{{.ClusterName}}.{{.BaseDomain}}/{{.VIP}}
address=/api-int.{{.ClusterName}}.{{.BaseDomain}}/{{.VIP}}
address=/.apps.{{.ClusterName}}.{{.BaseDomain}}/{{.VIP}}

{{- if not .IsSNO}}
# etcd SRV Records (for multi-node clusters only)
{{- range $index, $master := .Masters}}
srv-host=_etcd-server-ssl._tcp.{{$.ClusterName}}.{{$.BaseDomain}},etcd-{{$index}}.{{$.ClusterName}}.{{$.BaseDomain}},2380,0,{{$index}}
{{- end}}
{{- end}}
`

// DHCP-only configuration template
const dhcpConfigTemplate = `# ============================================
# DHCP Configuration for Cluster: {{.ClusterName}}
# Type: {{.Type}}
# OCP Version: {{.OCPVersion}}
# Generated: {{.Timestamp}}
# ============================================

# Network Bindings & Logging
interface={{.Interface}}
log-dhcp
dhcp-authoritative

# Static Subnet Definition (covers the whole {{.NetworkCIDR}} network)
dhcp-range={{.NetworkAddr}},static,{{.Netmask}},12h

# Network Options
dhcp-option=tag:{{.ClusterName}},option:router,{{.Gateway}}
dhcp-option=tag:{{.ClusterName}},option:dns-server,{{.HelperIP}}
dhcp-option=tag:{{.ClusterName}},option:domain-name,{{.ClusterName}}.{{.BaseDomain}}

# Static DHCP assignments with MAC-to-IP bindings
{{- range .Nodes}}
{{- if .MACAddress}}
dhcp-host={{.MACAddress}},set:{{$.ClusterName}},{{.IP}},{{.Name}},infinite
{{- end}}
{{- end}}
`

// PXE/TFTP-only configuration template
const pxeConfigTemplate = `# ============================================
# PXE/TFTP Configuration for Cluster: {{.ClusterName}}
# Type: {{.Type}}
# OCP Version: {{.OCPVersion}}
# Generated: {{.Timestamp}}
# ============================================

# TFTP/PXE Boot Configuration
enable-tftp
tftp-root=/var/lib/tftpboot
dhcp-boot=tag:{{.ClusterName}},{{.ClusterName}}/core.elf,,{{.HelperIP}}
`

// DNSmasqGenerator generates dnsmasq configuration for a cluster
type DNSmasqGenerator struct {
	ctx     *ClusterContext
	verbose bool
}

// NewDNSmasqGenerator creates a new dnsmasq generator
func NewDNSmasqGenerator(ctx *ClusterContext, verbose bool) *DNSmasqGenerator {
	return &DNSmasqGenerator{
		ctx:     ctx,
		verbose: verbose,
	}
}

// prepareTemplateData prepares common data for all dnsmasq templates
func (d *DNSmasqGenerator) prepareTemplateData() map[string]interface{} {
	network := d.ctx.ClusterConfig.Network

	// Extract network address from CIDR (e.g., "192.0.2.0/20" -> "192.0.2.0")
	networkAddr := network.NetworkCIDR
	if idx := strings.Index(networkAddr, "/"); idx > 0 {
		networkAddr = networkAddr[:idx]
	}

	// Fetch all nodes and forcefully lowercase the MAC addresses so dnsmasq matches them perfectly
	nodes := d.ctx.ClusterConfig.GetAllNodes()
	for i := range nodes {
		nodes[i].MACAddress = strings.ToLower(nodes[i].MACAddress)
	}

	data := map[string]interface{}{
		"ClusterName": d.ctx.Name,
		"Type":        d.ctx.Type,
		"OCPVersion":  d.ctx.OCPVersion,
		"Timestamp":   time.Now().Format(time.RFC3339),
		"Interface":   d.ctx.HelperNode.NetworkInterface,
		"NetworkCIDR": network.NetworkCIDR,
		"NetworkAddr": networkAddr,
		"Netmask":     network.Netmask,
		"Gateway":     network.Gateway,
		"HelperIP":    d.ctx.HelperNode.IP,
		"BaseDomain":  network.BaseDomain,
		"VIP":         d.ctx.VIP,
		"IsSNO":       d.ctx.ClusterConfig.IsSNO(),
		"Nodes":       nodes,
	}

	if !d.ctx.ClusterConfig.IsSNO() && d.ctx.ClusterConfig.Masters != nil {
		data["Masters"] = d.ctx.ClusterConfig.Masters.Nodes
	}

	return data
}

// GenerateDNS generates DNS-only configuration
func (d *DNSmasqGenerator) GenerateDNS() (string, error) {
	tmpl, err := template.New("dns").Parse(dnsConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse DNS template: %w", err)
	}

	data := d.prepareTemplateData()

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute DNS template: %w", err)
	}

	return buf.String(), nil
}

// GenerateDHCP generates DHCP-only configuration
func (d *DNSmasqGenerator) GenerateDHCP() (string, error) {
	tmpl, err := template.New("dhcp").Parse(dhcpConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse DHCP template: %w", err)
	}

	data := d.prepareTemplateData()

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute DHCP template: %w", err)
	}

	return buf.String(), nil
}

// GeneratePXE generates PXE/TFTP-only configuration
func (d *DNSmasqGenerator) GeneratePXE() (string, error) {
	tmpl, err := template.New("pxe").Parse(pxeConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse PXE template: %w", err)
	}

	data := d.prepareTemplateData()

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute PXE template: %w", err)
	}

	return buf.String(), nil
}

// GetDNSConfigPath returns the path for DNS configuration
func (d *DNSmasqGenerator) GetDNSConfigPath() string {
	return fmt.Sprintf("/etc/dnsmasq.d/10-%s-dns.conf", d.ctx.Name)
}

// GetDHCPConfigPath returns the path for DHCP configuration
func (d *DNSmasqGenerator) GetDHCPConfigPath() string {
	return fmt.Sprintf("/etc/dnsmasq.d/20-%s-dhcp.conf", d.ctx.Name)
}

// GetPXEConfigPath returns the path for PXE configuration
func (d *DNSmasqGenerator) GetPXEConfigPath() string {
	return fmt.Sprintf("/etc/dnsmasq.d/30-%s-pxe.conf", d.ctx.Name)
}

// Cleanup removes all dnsmasq configuration files for the cluster
func (d *DNSmasqGenerator) Cleanup(sshClient *communication.SSHClient) error {
	fmt.Printf("Cleaning up dnsmasq configuration files for cluster '%s'...\n", d.ctx.Name)

	// Remove DNS configuration file
	dnsPath := d.GetDNSConfigPath()
	if err := sshClient.RemoveFile(dnsPath); err != nil {
		fmt.Printf("  ⚠ Warning: Failed to remove DNS config %s: %v\n", dnsPath, err)
	} else {
		fmt.Printf("  ✓ Removed DNS config: %s\n", dnsPath)
	}

	// Remove DHCP configuration file
	dhcpPath := d.GetDHCPConfigPath()
	if err := sshClient.RemoveFile(dhcpPath); err != nil {
		fmt.Printf("  ⚠ Warning: Failed to remove DHCP config %s: %v\n", dhcpPath, err)
	} else {
		fmt.Printf("  ✓ Removed DHCP config: %s\n", dhcpPath)
	}

	// Remove PXE configuration file
	pxePath := d.GetPXEConfigPath()
	if err := sshClient.RemoveFile(pxePath); err != nil {
		fmt.Printf("  ⚠ Warning: Failed to remove PXE config %s: %v\n", pxePath, err)
	} else {
		fmt.Printf("  ✓ Removed PXE config: %s\n", pxePath)
	}

	return nil
}

// CleanupLeases removes DHCP lease entries for the cluster's nodes
func (d *DNSmasqGenerator) CleanupLeases(sshClient *communication.SSHClient) error {
	leaseFile := "/var/lib/dnsmasq/dnsmasq.leases"

	// Check if lease file exists
	checkCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'missing'", leaseFile)
	output, err := sshClient.ExecuteCommand(checkCmd)
	if err != nil || strings.TrimSpace(output) != "exists" {
		fmt.Println("  ℹ Dnsmasq lease file not found, skipping cleanup")
		return nil
	}

	// Get MAC addresses from state (most reliable)
	var macAddresses []string
	if d.ctx.State != nil && len(d.ctx.State.CreatedLPARs) > 0 {
		for _, lpar := range d.ctx.State.CreatedLPARs {
			if lpar.MACAddress != "" {
				// Normalize MAC address format (lowercase, with colons)
				mac := strings.ToLower(strings.ReplaceAll(lpar.MACAddress, "-", ":"))
				macAddresses = append(macAddresses, mac)
			}
		}
	}

	// Get hostnames and IPs from configuration as backup
	nodes := d.ctx.ClusterConfig.GetAllNodes()
	var hostnames []string
	var ips []string
	for _, node := range nodes {
		if node.Hostname != "" {
			hostnames = append(hostnames, node.Hostname)
		}
		if node.Name != "" && node.Name != node.Hostname {
			hostnames = append(hostnames, node.Name)
		}
		if node.IP != "" {
			ips = append(ips, node.IP)
		}
	}

	if len(macAddresses) == 0 && len(hostnames) == 0 && len(ips) == 0 {
		fmt.Println("  ℹ No MAC addresses, hostnames, or IPs found to clean from lease file")
		return nil
	}

	// Count total entries to remove
	totalRemoved := 0

	// Remove lease entries by MAC address (most reliable)
	for _, mac := range macAddresses {
		// Escape special characters for sed
		escapedMAC := strings.ReplaceAll(mac, ":", "\\:")
		cmd := fmt.Sprintf("sudo sed -i '/%s/d' %s", escapedMAC, leaseFile)
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			fmt.Printf("  ⚠ Warning: Failed to remove lease for MAC %s: %v\n", mac, err)
		} else {
			fmt.Printf("  ✓ Removed lease entry for MAC: %s\n", mac)
			totalRemoved++
		}
	}

	// Also remove by IP address (reliable identifier)
	for _, ip := range ips {
		cmd := fmt.Sprintf("sudo sed -i '/%s/d' %s", ip, leaseFile)
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			fmt.Printf("  ⚠ Warning: Failed to remove lease for IP %s: %v\n", ip, err)
		} else {
			fmt.Printf("  ✓ Removed lease entry for IP: %s\n", ip)
			totalRemoved++
		}
	}

	// Also remove by hostname as additional safety
	for _, hostname := range hostnames {
		cmd := fmt.Sprintf("sudo sed -i '/%s/d' %s", hostname, leaseFile)
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			fmt.Printf("  ⚠ Warning: Failed to remove lease for hostname %s: %v\n", hostname, err)
		} else {
			fmt.Printf("  ✓ Removed lease entry for hostname: %s\n", hostname)
			totalRemoved++
		}
	}

	if totalRemoved > 0 {
		fmt.Printf("  ✓ Total lease entries removed: %d\n", totalRemoved)
	} else {
		fmt.Println("  ℹ No matching lease entries found to remove")
	}

	return nil
}
