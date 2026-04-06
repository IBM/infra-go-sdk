package services

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	. "github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer/types"
)

const haproxyTemplate = `# ==========================================
# Cluster: {{.ClusterName}}
# Type: {{.Type}}
# OCP Version: {{.OCPVersion}}
# VIP: {{.VIP}} (Single VIP for all services)
# Generated: {{.Timestamp}}
# ==========================================

# API Server (Port 6443)
frontend {{.ClusterName}}-openshift-api-server
    bind {{.VIP}}:6443
    default_backend {{.ClusterName}}-openshift-api-server
    mode tcp
    option tcplog

backend {{.ClusterName}}-openshift-api-server
    balance source
    mode tcp
{{- if .IsSNO}}
    server {{.SNONode.Hostname}} {{.SNONode.IP}}:6443 check
{{- else}}
{{- range .Masters}}
    server {{.Name}} {{.IP}}:6443 check
{{- end}}
{{- end}}

# Machine Config Server (Port 22623)
frontend {{.ClusterName}}-machine-config-server
    bind {{.VIP}}:22623
    default_backend {{.ClusterName}}-machine-config-server
    mode tcp
    option tcplog

backend {{.ClusterName}}-machine-config-server
    balance source
    mode tcp
{{- if .IsSNO}}
    server {{.SNONode.Hostname}} {{.SNONode.IP}}:22623 check
{{- else}}
{{- range .Masters}}
    server {{.Name}} {{.IP}}:22623 check
{{- end}}
{{- end}}

# Ingress HTTP (Port 80)
frontend {{.ClusterName}}-ingress-http
    bind {{.VIP}}:80
    default_backend {{.ClusterName}}-ingress-http
    mode tcp
    option tcplog

backend {{.ClusterName}}-ingress-http
    balance source
    mode tcp
{{- if .IsSNO}}
    server {{.SNONode.Hostname}}-http {{.SNONode.IP}}:80 check
{{- else}}
{{- if .Workers}}
{{- range .Workers}}
    server {{.Name}}-http-router0 {{.IP}}:80 check
{{- end}}
{{- else}}
{{- range .Masters}}
    server {{.Name}}-http-router0 {{.IP}}:80 check
{{- end}}
{{- end}}
{{- end}}

# Ingress HTTPS (Port 443)
frontend {{.ClusterName}}-ingress-https
    bind {{.VIP}}:443
    default_backend {{.ClusterName}}-ingress-https
    mode tcp
    option tcplog

backend {{.ClusterName}}-ingress-https
    balance source
    mode tcp
{{- if .IsSNO}}
    server {{.SNONode.Hostname}}-https {{.SNONode.IP}}:443 check
{{- else}}
{{- if .Workers}}
{{- range .Workers}}
    server {{.Name}}-https-router0 {{.IP}}:443 check
{{- end}}
{{- else}}
{{- range .Masters}}
    server {{.Name}}-https-router0 {{.IP}}:443 check
{{- end}}
{{- end}}
{{- end}}
`

// HAProxyGenerator generates HAProxy configuration for a cluster
type HAProxyGenerator struct {
	ctx     *ClusterContext
	verbose bool
}

// NewHAProxyGenerator creates a new HAProxy generator
func NewHAProxyGenerator(ctx *ClusterContext, verbose bool) *HAProxyGenerator {
	return &HAProxyGenerator{
		ctx:     ctx,
		verbose: verbose,
	}
}

// Generate generates the complete HAProxy configuration
func (h *HAProxyGenerator) Generate() (string, error) {
	tmpl, err := template.New("haproxy").Parse(haproxyTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse haproxy template: %w", err)
	}

	cfg := h.ctx.ClusterConfig

	// Ensure SNO node hostname is populated (fallback to cluster name if empty)
	snoNode := cfg.SNONode
	if snoNode != nil && snoNode.Hostname == "" {
		snoNode = &SNONodeConfig{
			Name:       snoNode.Name,
			Hostname:   h.ctx.Name,
			IP:         snoNode.IP,
			SystemName: snoNode.SystemName,
			LPAR:       snoNode.LPAR,
			MACAddress: snoNode.MACAddress,
		}
	}

	// Create data structure for the template
	data := struct {
		ClusterName string
		Type        string
		OCPVersion  string
		VIP         string
		Timestamp   string
		IsSNO       bool
		SNONode     *SNONodeConfig
		Masters     []NodeConfig
		Workers     []NodeConfig
	}{
		ClusterName: h.ctx.Name,
		Type:        h.ctx.Type,
		OCPVersion:  h.ctx.OCPVersion,
		VIP:         h.ctx.VIP,
		Timestamp:   time.Now().Format(time.RFC3339),
		IsSNO:       cfg.IsSNO(),
		SNONode:     snoNode,
	}

	if !data.IsSNO && cfg.Masters != nil {
		data.Masters = cfg.Masters.Nodes
	}
	if !data.IsSNO && cfg.Workers != nil {
		data.Workers = cfg.Workers.Nodes
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute haproxy template: %w", err)
	}

	return buf.String(), nil
}

// GetConfigPath returns the path where this config should be written
func (h *HAProxyGenerator) GetConfigPath() string {
	return fmt.Sprintf("/etc/haproxy/conf.d/10-%s.cfg", h.ctx.Name)
}
