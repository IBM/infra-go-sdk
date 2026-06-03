package hmc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/beevik/etree"
)

// GetLparAggregatedMetrics retrieves the Atom feed of aggregated performance metrics for a specific LPAR.
func (c *RestClient) GetLparAggregatedMetrics(ctx context.Context, sysUUID, lparUUID string, opts *AggregatedMetricsOptions, debug bool) ([]PcmMetricsSnapshot, error) {
	if opts == nil || opts.StartTS.IsZero() {
		return nil, fmt.Errorf("StartTS is a mandatory parameter for retrieving aggregated metrics")
	}

	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/LogicalPartition/%s/AggregatedMetrics", c.hmcIP, sysUUID, lparUUID)
	//baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/LogicalPartition/%s/ProcessedMetrics", c.hmcIP, sysUUID, lparUUID)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Manually build the raw query string to prevent Go from encoding ':' to '%3A'
	timeFormat := "2006-01-02T15:04:05Z"
	rawQuery := fmt.Sprintf("StartTS=%s", opts.StartTS.UTC().Format(timeFormat))

	if !opts.EndTS.IsZero() {
		rawQuery += fmt.Sprintf("&EndTS=%s", opts.EndTS.UTC().Format(timeFormat))
	}
	if opts.NoOfSamples > 0 {
		rawQuery += fmt.Sprintf("&NoOfSamples=%d", opts.NoOfSamples)
	}

	// Inject the literal string directly into the RawQuery property
	req.URL.RawQuery = rawQuery

	if debug {
		c.Logger.Debug("Fetching LPAR Aggregated Metrics Feed", "url", req.URL.String())
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/xml") // Safely accept both XML formats

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var snapshots []PcmMetricsSnapshot

	for _, entry := range doc.FindElements("//entry") {
		var snap PcmMetricsSnapshot

		if pub := entry.FindElement("published"); pub != nil {
			snap.Published = pub.Text()
		}
		if upd := entry.FindElement("updated"); upd != nil {
			snap.Updated = upd.Text()
		}

		for _, link := range entry.FindElements("link") {
			if link.SelectAttrValue("type", "") == "application/json" {
				snap.JSONLink = link.SelectAttrValue("href", "")
				break
			}
		}

		for _, cat := range entry.FindElements("category") {
			term := cat.SelectAttrValue("term", "")
			if term == "LogicalPartition" {
				snap.Category = term
			} else {
				snap.Frequency = term
			}
		}

		snapshots = append(snapshots, snap)
	}

	if debug {
		c.Logger.Info("Successfully parsed PCM Aggregated Metrics feed", "snapshotCount", len(snapshots))
	}

	return snapshots, nil
}

// FetchPcmMetricsPayload downloads and unmarshals the JSON metrics snapshot from a PCM Atom link.
// It bypasses strict MIME-type checking to prevent Tomcat 406 Not Acceptable errors on static JSON files.
func (c *RestClient) FetchPcmMetricsPayload(ctx context.Context, jsonURL string, debug bool) (*PcmMetricsPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)

	req.Header.Set("Accept", "*/*")

	c.logRawTraffic("REQUEST (GET)", jsonURL, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", jsonURL, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	var payload PcmMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.Logger.Error("Failed to unmarshal JSON metrics payload", "error", err)
		return nil, fmt.Errorf("failed to unmarshal JSON metrics payload: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully fetched and parsed PCM metrics JSON", "url", jsonURL)
	}

	return &payload, nil
}

// GetManagedSystemAggregatedMetrics retrieves the Atom feed of aggregated performance metrics for a Managed System.
func (c *RestClient) GetManagedSystemAggregatedMetrics(ctx context.Context, systemUUID string, opts *ManagedSystemMetricsOptions, debug bool) ([]PcmMetricsSnapshot, error) {
	if opts == nil || opts.StartTS.IsZero() {
		return nil, fmt.Errorf("StartTS is a mandatory parameter for retrieving system aggregated metrics")
	}

	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/AggregatedMetrics", c.hmcIP, systemUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	timeFormat := "2006-01-02T15:04:05Z"
	rawQuery := fmt.Sprintf("StartTS=%s", opts.StartTS.UTC().Format(timeFormat))

	if !opts.EndTS.IsZero() {
		rawQuery += fmt.Sprintf("&EndTS=%s", opts.EndTS.UTC().Format(timeFormat))
	}
	if opts.NoOfSamples > 0 {
		rawQuery += fmt.Sprintf("&NoOfSamples=%d", opts.NoOfSamples)
	}
	if opts.Feed != "" {
		rawQuery += fmt.Sprintf("&Feed=%s", opts.Feed)
	}

	req.URL.RawQuery = rawQuery

	if debug {
		c.Logger.Debug("Fetching Managed System Aggregated Metrics Feed", "url", req.URL.String())
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/xml")

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var snapshots []PcmMetricsSnapshot
	elements := doc.FindElements("//entry")

	for _, entry := range elements {
		var snap PcmMetricsSnapshot
		isManagedSystemFile := false

		// Verify category terms to filter out mixed-in LPAR metadata links
		for _, cat := range entry.FindElements("category") {
			term := cat.SelectAttrValue("term", "")
			if term == "ManagedSystem" {
				snap.Category = term
				isManagedSystemFile = true
			} else if term != "LogicalPartition" {
				snap.Frequency = term
			}
		}

		// Skip LPAR discovery records
		if !isManagedSystemFile {
			continue
		}

		if pub := entry.FindElement("published"); pub != nil {
			snap.Published = pub.Text()
		}
		if upd := entry.FindElement("updated"); upd != nil {
			snap.Updated = upd.Text()
		}

		for _, link := range entry.FindElements("link") {
			href := link.SelectAttrValue("href", "")
			typ := link.SelectAttrValue("type", "")
			if typ == "application/json" || typ == "application/JSON" || strings.Contains(strings.ToLower(href), ".json") {
				snap.JSONLink = href
				break
			}
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// FetchSystemPcmMetricsPayload downloads and parses the system-wide JSON utilization data from a specific snapshot link.
func (c *RestClient) FetchSystemPcmMetricsPayload(ctx context.Context, jsonURL string, debug bool) (*SysPcmMetricsPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate requests context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*") // Bypass HTTP 406 MIME checking

	c.logRawTraffic("REQUEST (GET)", jsonURL, "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP transport request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading stream body payload: %v", err)
	}

	c.logRawTraffic("RESPONSE", jsonURL, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected fetch with status: %s", resp.Status)
	}

	var payload SysPcmMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("json metrics model compilation fallback: %v", err)
	}

	return &payload, nil
}

// GetManagedSystemLtmFeed retrieves the Atom feed containing links to raw 30-second LTM metrics.
// It maps snapshots separately for PHYP and individual VIOS profiles.
//
// Reference: HMC REST API - Long Term Monitor Metrics (LTM)
func (c *RestClient) GetManagedSystemLtmFeed(ctx context.Context, systemUUID string, opts *LtmMetricsOptions) ([]PcmMetricsSnapshot, error) {
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/RawMetrics/LongTermMonitor", c.hmcIP, systemUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct LTM request: %v", err)
	}

	// Dynamic safe query string assignment (Prevents Go encoding colons to %3A)
	timeFormat := "2006-01-02T15:04:05Z"
	var rawQuery string

	if opts != nil {
		if !opts.StartTS.IsZero() {
			rawQuery = fmt.Sprintf("StartTS=%s", opts.StartTS.UTC().Format(timeFormat))
		}
		if !opts.EndTS.IsZero() {
			if rawQuery != "" {
				rawQuery += "&"
			}
			rawQuery += fmt.Sprintf("EndTS=%s", opts.EndTS.UTC().Format(timeFormat))
		}
	}
	req.URL.RawQuery = rawQuery

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/xml")

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LTM transport transmission failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading LTM feed response byte buffer: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected LTM raw metrics query with status: %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed stripping LTM XML namespaces: %v", err)
	}

	var snapshots []PcmMetricsSnapshot
	elements := doc.FindElements("//entry")

	for _, entry := range elements {
		var snap PcmMetricsSnapshot

		if pub := entry.FindElement("published"); pub != nil {
			snap.Published = pub.Text()
		}
		if upd := entry.FindElement("updated"); upd != nil {
			snap.Updated = upd.Text()
		}

		// Extract the Source Category: "PHYP" or "vios_[id]"
		if cat := entry.FindElement("category"); cat != nil {
			snap.Category = cat.SelectAttrValue("term", "Unknown")
		}

		// Pull out direct download JSON links
		for _, link := range entry.FindElements("link") {
			href := link.SelectAttrValue("href", "")
			typ := link.SelectAttrValue("type", "")
			if typ == "application/json" || typ == "application/JSON" || strings.Contains(strings.ToLower(href), ".json") {
				snap.JSONLink = href
				break
			}
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// FetchLtmPhypMetricsPayload downloads and unmarshals raw point-in-time Hypervisor JSON metrics.
// It applies a wildcard Accept header to isolate operations from local server blocks.
func (c *RestClient) FetchLtmPhypMetricsPayload(ctx context.Context, jsonURL string) (*LtmPhypPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize request context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*") // Guards against HTTP 406 Not Acceptable execution blocks

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transport transaction aborted: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading streaming payload data buffer: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hmc processing failed with target status: %s", resp.Status)
	}

	var payload LtmPhypPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to map raw telemetry to go structures: %v", err)
	}

	return &payload, nil
}

// FetchLtmViosMetricsPayload downloads and unmarshals raw point-in-time VIOS JSON metrics.
// This is used for LTM streams where the Feed category starts with "vios_".
func (c *RestClient) FetchLtmViosMetricsPayload(ctx context.Context, jsonURL string) (*LtmViosPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize request context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*") // Bypass HTTP 406 MIME checking on raw JSON files

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transport transaction aborted: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading streaming payload data buffer: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hmc processing failed with target status: %s", resp.Status)
	}

	var payload LtmViosPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to map raw vios telemetry to go structures: %v", err)
	}

	return &payload, nil
}

// GetManagedSystemPcmPreferences retrieves the current metrics collection preferences for a managed system.
func (c *RestClient) GetManagedSystemPcmPreferences(ctx context.Context, systemUUID string) (*ManagedSystemPcmPreference, error) {
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/preferences", c.hmcIP, systemUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/xml, text/xml, */*")

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected GET request with status: %s", resp.Status)
	}

	// 1. Strip namespaces so we can query smoothly
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	// Find the root element
	root := doc.FindElement("//ManagedSystemPcmPreference")
	if root == nil {
		return nil, fmt.Errorf("ManagedSystemPcmPreference element not found in response")
	}

	// 3. Safely serialize the isolated element back to XML bytes using a temporary Document
	prefDoc := etree.NewDocument()
	prefDoc.SetRoot(root.Copy())
	strippedXML, err := prefDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize preferences XML: %v", err)
	}

	// 4. Unmarshal the clean bytes into our Go struct
	var prefs ManagedSystemPcmPreference
	if err := xml.Unmarshal(strippedXML, &prefs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal preferences: %v", err)
	}

	return &prefs, nil
}

// SetManagedSystemPcmPreferences updates the metrics collection preferences on the HMC.
// Uses a pristine GET-Modify-POST pattern to satisfy IBM's strict JAXB XML schema requirements.
func (c *RestClient) SetManagedSystemPcmPreferences(ctx context.Context, systemUUID string, prefs *ManagedSystemPcmPreference) error {
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/preferences", c.hmcIP, systemUUID)

	// 1. Fetch pristine XML to preserve namespaces and Read-Only Required (ROR) attributes
	getReq, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create pre-flight GET request: %v", err)
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/xml, text/xml, */*")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return fmt.Errorf("pre-flight GET request failed: %v", err)
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != http.StatusOK {
		return fmt.Errorf("pre-flight GET failed with status %s: %s", getResp.Status, string(rawXML))
	}

	// 2. Parse the pristine XML using etree
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	// 3. Extract the core ManagedSystemPcmPreference element (bypassing Atom feed wrappers natively)
	prefElem := doc.FindElement(".//*[local-name()='ManagedSystemPcmPreference']")
	if prefElem == nil {
		return fmt.Errorf("ManagedSystemPcmPreference element not found in pristine XML")
	}

	// 4. Helper function to safely update or inject the XML tag
	updateTag := func(tagName string, value bool) {
		tag := prefElem.FindElement(".//*[local-name()='" + tagName + "']")
		if tag != nil {
			tag.SetText(fmt.Sprintf("%v", value))
		} else {
			newTag := prefElem.CreateElement(tagName)
			newTag.CreateAttr("kb", "UOD")
			newTag.CreateAttr("kxe", "false")
			newTag.SetText(fmt.Sprintf("%v", value))
		}
	}

	// 5. Update the DOM tree with our new configuration state
	updateTag("AggregationEnabled", prefs.AggregationEnabled)
	updateTag("ComputeLTMEnabled", prefs.ComputeLTMEnabled)
	updateTag("EnergyMonitorEnabled", prefs.EnergyMonitorEnabled)
	updateTag("LongTermMonitorEnabled", prefs.LongTermMonitorEnabled)
	updateTag("ShortTermMonitorEnabled", prefs.ShortTermMonitorEnabled)

	// 6. Serialize the isolated configuration element back to an XML string
	postDoc := etree.NewDocument()
	postDoc.SetRoot(prefElem.Copy())
	postXML, _ := postDoc.WriteToString()

	// 7. POST the updated XML back to the HMC
	postReq, err := http.NewRequestWithContext(ctx, "POST", baseURL, strings.NewReader(postXML))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %v", err)
	}
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/xml")
	postReq.Header.Set("Accept", "application/xml, text/xml, */*")

	c.logRawTraffic("REQUEST (POST)", baseURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return fmt.Errorf("HTTP POST request failed: %v", err)
	}
	defer postResp.Body.Close()

	postBody, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", baseURL, string(postBody))

	// HMC returns 200 OK or 204 No Content for a successful preference update
	if postResp.StatusCode >= 400 {
		return fmt.Errorf("HMC rejected POST request with status %s: %s", postResp.Status, string(postBody))
	}

	return nil
}

// GetManagementConsolePcmPreferences retrieves the global metrics collection preferences for the HMC and all connected systems.
func (c *RestClient) GetManagementConsolePcmPreferences(ctx context.Context, debug bool) (*ManagementConsolePcmPreference, error) {
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/preferences", c.hmcIP)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/xml, text/xml, */*") // Bypass 406 Not Acceptable

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected GET request with status: %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	// Navigate deeply to find the global console preference block
	root := doc.FindElement("//ManagementConsolePcmPreference")
	if root == nil {
		return nil, fmt.Errorf("ManagementConsolePcmPreference element not found in response")
	}

	prefDoc := etree.NewDocument()
	prefDoc.SetRoot(root.Copy())
	strippedXML, err := prefDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize preferences XML: %v", err)
	}

	var prefs ManagementConsolePcmPreference
	if err := xml.Unmarshal(strippedXML, &prefs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global preferences: %v", err)
	}

	return &prefs, nil
}

// SetManagementConsolePcmPreferences updates the global PCM configuration for the HMC and its managed systems.
// It uses a pristine GET-Modify-POST pattern to satisfy IBM's strict XML schema requirements.
func (c *RestClient) SetManagementConsolePcmPreferences(ctx context.Context, prefs *ManagementConsolePcmPreference, debug bool) error {
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/preferences", c.hmcIP)

	// 1. Fetch pristine XML to preserve namespaces and Read-Only Required (ROR) attributes
	getReq, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create pre-flight GET request: %v", err)
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/xml, text/xml, */*")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return fmt.Errorf("pre-flight GET request failed: %v", err)
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != http.StatusOK {
		return fmt.Errorf("pre-flight GET failed with status %s: %s", getResp.Status, string(rawXML))
	}

	// 2. Parse the pristine XML
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return fmt.Errorf("failed to parse pristine XML: %v", err)
	}

	rootElem := doc.FindElement(".//*[local-name()='ManagementConsolePcmPreference']")
	if rootElem == nil {
		return fmt.Errorf("ManagementConsolePcmPreference element not found in pristine XML")
	}

	// 3. Update the global metrics storage duration
	if durElem := rootElem.FindElement(".//*[local-name()='AggregatedMetricsStorageDuration']"); durElem != nil {
		durElem.SetText(fmt.Sprintf("%d", prefs.AggregatedMetricsStorageDuration))
	}

	// 4. Safely update individual managed system preferences inside the global list
	for _, sysPref := range prefs.ManagedSystemPcmPreferences {
		sysNodes := rootElem.FindElements(".//*[local-name()='ManagedSystemPcmPreference']")

		for _, sysNode := range sysNodes {
			idNode := sysNode.FindElement(".//*[local-name()='AtomID']")
			if idNode != nil && idNode.Text() == sysPref.MetadataID {

				// Helper to securely inject or update the boolean tags inside the target system node
				updateTag := func(tagName string, value bool) {
					tag := sysNode.FindElement(".//*[local-name()='" + tagName + "']")
					if tag != nil {
						tag.SetText(fmt.Sprintf("%v", value))
					} else {
						newTag := sysNode.CreateElement(tagName)
						newTag.CreateAttr("kb", "UOD")
						newTag.CreateAttr("kxe", "false")
						newTag.SetText(fmt.Sprintf("%v", value))
					}
				}

				updateTag("AggregationEnabled", sysPref.AggregationEnabled)
				updateTag("ComputeLTMEnabled", sysPref.ComputeLTMEnabled)
				updateTag("EnergyMonitorEnabled", sysPref.EnergyMonitorEnabled)
				updateTag("LongTermMonitorEnabled", sysPref.LongTermMonitorEnabled)
				updateTag("ShortTermMonitorEnabled", sysPref.ShortTermMonitorEnabled)
				break
			}
		}
	}

	// 5. Serialize and POST the updated XML payload back to the HMC
	postDoc := etree.NewDocument()
	postDoc.SetRoot(rootElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, err := http.NewRequestWithContext(ctx, "POST", baseURL, strings.NewReader(postXML))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %v", err)
	}
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/xml")
	postReq.Header.Set("Accept", "application/xml, text/xml, */*")

	c.logRawTraffic("REQUEST (POST)", baseURL, postXML)

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return fmt.Errorf("HTTP POST request failed: %v", err)
	}
	defer postResp.Body.Close()

	postBody, _ := io.ReadAll(postResp.Body)
	c.logRawTraffic("RESPONSE", baseURL, string(postBody))

	if postResp.StatusCode >= 400 {
		return fmt.Errorf("HMC rejected POST request with status %s: %s", postResp.Status, string(postBody))
	}

	return nil
}

// GetLogicalPartitionProcessedMetrics retrieves the Atom feed of processed performance metrics (30-second intervals) for an LPAR.
func (c *RestClient) GetLogicalPartitionProcessedMetrics(ctx context.Context, systemUUID, partitionUUID string, opts *LparProcessedMetricsOptions, debug bool) ([]PcmMetricsSnapshot, error) {
	if opts == nil || opts.StartTS.IsZero() {
		return nil, fmt.Errorf("StartTS is a mandatory parameter for retrieving LPAR processed metrics")
	}

	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/LogicalPartition/%s/ProcessedMetrics", c.hmcIP, systemUUID, partitionUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	timeFormat := "2006-01-02T15:04:05Z"
	rawQuery := fmt.Sprintf("StartTS=%s", opts.StartTS.UTC().Format(timeFormat))

	if !opts.EndTS.IsZero() {
		rawQuery += fmt.Sprintf("&EndTS=%s", opts.EndTS.UTC().Format(timeFormat))
	}
	if opts.NoOfSamples > 0 {
		rawQuery += fmt.Sprintf("&NoOfSamples=%d", opts.NoOfSamples)
	}

	req.URL.RawQuery = rawQuery

	if debug {
		c.Logger.Debug("Fetching LPAR Processed Metrics Feed", "url", req.URL.String())
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/xml")

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		if debug {
			c.Logger.Debug("No processed metrics found for the specified time range (HTTP 204)")
		}
		return []PcmMetricsSnapshot{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var snapshots []PcmMetricsSnapshot
	elements := doc.FindElements("//entry")

	for _, entry := range elements {
		var snap PcmMetricsSnapshot

		if pub := entry.FindElement("published"); pub != nil {
			snap.Published = pub.Text()
		}
		if upd := entry.FindElement("updated"); upd != nil {
			snap.Updated = upd.Text()
		}

		for _, link := range entry.FindElements("link") {
			href := link.SelectAttrValue("href", "")
			typ := link.SelectAttrValue("type", "")

			if typ == "application/json" || typ == "application/JSON" || strings.Contains(strings.ToLower(href), ".json") {
				snap.JSONLink = href
				break
			}
		}

		for _, cat := range entry.FindElements("category") {
			term := cat.SelectAttrValue("term", "")
			if term == "LogicalPartition" {
				snap.Category = term
			} else {
				snap.Frequency = term
			}
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// FetchLparProcessedMetricsPayload downloads and safely unmarshals a specific 30-second Processed LPAR JSON file.
// It applies a wildcard Accept header to isolate operations from local server MIME-type checking blocks.
func (c *RestClient) FetchLparProcessedMetricsPayload(ctx context.Context, jsonURL string) (*LparProcessedMetricsPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize request context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*") // Guards against HTTP 406 Not Acceptable execution blocks

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transport transaction aborted: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading streaming payload data buffer: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hmc processing failed with target status: %s", resp.Status)
	}

	var payload LparProcessedMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to map raw LPAR telemetry to go structures: %v", err)
	}

	return &payload, nil
}

// EnableLparPerformanceCollection enables the AllowPerformanceDataCollection flag on a specific LPAR
// using a pristine GET-Modify-POST pattern. This is required for the hypervisor to release LPAR-specific telemetry.
func (c *RestClient) EnableLparPerformanceCollection(ctx context.Context, lparUUID string, debug bool) error {
	url := fmt.Sprintf("https://%s/rest/api/uom/LogicalPartition/%s", c.hmcIP, lparUUID)

	if debug {
		c.Logger.Debug("Checking LPAR Performance Data Collection permission", "lparUUID", lparUUID)
	}

	// 1. Pristine GET
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("X-API-Session", c.session)
	getReq.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml")

	getResp, err := c.client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()

	rawXML, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != 200 {
		return fmt.Errorf("GET failed: %s", string(rawXML))
	}

	// 2. Parse and Modify
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawXML); err != nil {
		return err
	}

	lparElem := doc.FindElement(".//*[local-name()='LogicalPartition']")
	if lparElem == nil {
		return fmt.Errorf("LogicalPartition element not found")
	}

	perfTag := lparElem.FindElement(".//*[local-name()='AllowPerformanceDataCollection']")
	if perfTag != nil {
		if perfTag.Text() == "true" {
			if debug {
				c.Logger.Debug("Performance Data Collection is already enabled for this LPAR")
			}
			return nil // Already enabled, nothing to do
		}
		perfTag.SetText("true")
	} else {
		newTag := lparElem.CreateElement("AllowPerformanceDataCollection")
		newTag.CreateAttr("kb", "UOD")
		newTag.CreateAttr("kxe", "false")
		newTag.SetText("true")
	}

	if debug {
		c.Logger.Info("Enabling AllowPerformanceDataCollection on LPAR natively...")
	}

	// 3. POST back
	postDoc := etree.NewDocument()
	postDoc.SetRoot(lparElem.Copy())
	postXML, _ := postDoc.WriteToString()

	postReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(postXML))
	if err != nil {
		return err
	}
	postReq.Header.Set("X-API-Session", c.session)
	postReq.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=LogicalPartition")
	postReq.Header.Set("Accept", "application/atom+xml")

	postResp, err := c.client.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)

	// Graceful RMC Error Handling (In case the LPAR is powered off)
	if postResp.StatusCode >= 400 {
		bodyStr := string(body)
		if strings.Contains(bodyStr, "HSCL2957") || strings.Contains(bodyStr, "HSCL294D") {
			if debug {
				c.Logger.Warn("Collection enabled in profile, but DLPAR push timed out (LPAR likely offline).")
			}
			return nil
		}
		return fmt.Errorf("POST failed (%s): %s", postResp.Status, bodyStr)
	}

	return nil
}

// GetManagedSystemProcessedMetrics retrieves the Atom feed of processed performance metrics (30-second intervals) for the entire Managed System.
func (c *RestClient) GetManagedSystemProcessedMetrics(ctx context.Context, systemUUID string, opts *AggregatedMetricsOptions, debug bool) ([]PcmMetricsSnapshot, error) {
	if opts == nil || opts.StartTS.IsZero() {
		return nil, fmt.Errorf("StartTS is a mandatory parameter for retrieving system processed metrics")
	}

	// Base API Path for Managed System
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/ProcessedMetrics", c.hmcIP, systemUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Strictly format the timestamps to Zulu (Z) to satisfy Tomcat
	timeFormat := "2006-01-02T15:04:05Z"
	rawQuery := fmt.Sprintf("StartTS=%s", opts.StartTS.UTC().Format(timeFormat))

	if !opts.EndTS.IsZero() {
		rawQuery += fmt.Sprintf("&EndTS=%s", opts.EndTS.UTC().Format(timeFormat))
	}
	if opts.NoOfSamples > 0 {
		rawQuery += fmt.Sprintf("&NoOfSamples=%d", opts.NoOfSamples)
	}

	req.URL.RawQuery = rawQuery

	if debug {
		c.Logger.Debug("Fetching Managed System Processed Metrics Feed", "url", req.URL.String())
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/xml")

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Graceful handling of empty cache windows
	if resp.StatusCode == http.StatusNoContent {
		if debug {
			c.Logger.Debug("No processed metrics found for the specified time range (HTTP 204)")
		}
		return []PcmMetricsSnapshot{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var snapshots []PcmMetricsSnapshot
	for _, entry := range doc.FindElements("//entry") {
		var snap PcmMetricsSnapshot

		if pub := entry.FindElement("published"); pub != nil {
			snap.Published = pub.Text()
		}
		if upd := entry.FindElement("updated"); upd != nil {
			snap.Updated = upd.Text()
		}

		for _, link := range entry.FindElements("link") {
			href := link.SelectAttrValue("href", "")
			typ := link.SelectAttrValue("type", "")
			if typ == "application/json" || typ == "application/JSON" || strings.Contains(strings.ToLower(href), ".json") {
				snap.JSONLink = href
				break
			}
		}

		for _, cat := range entry.FindElements("category") {
			term := cat.SelectAttrValue("term", "")
			if term == "ManagedSystem" {
				snap.Category = term
			} else {
				snap.Frequency = term
			}
		}

		snapshots = append(snapshots, snap)
	}

	if debug {
		c.Logger.Info("Successfully parsed System Processed metrics feed", "snapshotCount", len(snapshots))
	}

	return snapshots, nil
}

// FetchSysProcessedMetricsPayload downloads and unmarshals a specific Managed System JSON metrics file.
// Natively accepts both 30-second Processed files and the larger Aggregated files.
func (c *RestClient) FetchSysProcessedMetricsPayload(ctx context.Context, jsonURL string, debug bool) (*SysProcessedMetricsPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*") // Avoid 406 Not Acceptable errors

	if debug {
		c.Logger.Debug("Downloading System Metrics JSON payload", "url", jsonURL)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transaction failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response payload: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected payload download with status: %s", resp.Status)
	}

	var payload SysProcessedMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to map raw system telemetry to struct: %v", err)
	}

	return &payload, nil
}

// GetShortTermMonitorMetrics retrieves the Atom feed of Short Term Monitor (5-second intervals) performance metrics.
// Note: STM metrics are only retained by the HMC for 30 minutes. The Atom feed returns distinct JSON links for PHYP and each VIOS.
func (c *RestClient) GetShortTermMonitorMetrics(ctx context.Context, systemUUID string, opts *ShortTermMetricsOptions, debug bool) ([]PcmMetricsSnapshot, error) {
	baseURL := fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/RawMetrics/ShortTermMonitor", c.hmcIP, systemUUID)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	var rawQuery string
	timeFormat := "2006-01-02T15:04:05Z"

	if opts != nil && !opts.StartTS.IsZero() {
		rawQuery = fmt.Sprintf("StartTS=%s", opts.StartTS.UTC().Format(timeFormat))
	}
	if opts != nil && !opts.EndTS.IsZero() {
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += fmt.Sprintf("EndTS=%s", opts.EndTS.UTC().Format(timeFormat))
	}

	req.URL.RawQuery = rawQuery

	if debug {
		c.Logger.Debug("Fetching Short Term Monitor Metrics Feed", "url", req.URL.String())
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml, application/xml")

	c.logRawTraffic("REQUEST (GET)", req.URL.String(), "")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		if debug {
			c.Logger.Debug("No STM metrics found for the specified time range (HTTP 204)")
		}
		return []PcmMetricsSnapshot{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", req.URL.String(), string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	var snapshots []PcmMetricsSnapshot
	for _, entry := range doc.FindElements("//entry") {
		var snap PcmMetricsSnapshot

		if pub := entry.FindElement("published"); pub != nil {
			snap.Published = pub.Text()
		}
		if upd := entry.FindElement("updated"); upd != nil {
			snap.Updated = upd.Text()
		}

		for _, link := range entry.FindElements("link") {
			href := link.SelectAttrValue("href", "")
			typ := link.SelectAttrValue("type", "")
			if typ == "application/json" || typ == "application/JSON" || strings.Contains(strings.ToLower(href), ".json") {
				snap.JSONLink = href
				break
			}
		}

		// STM uses Category to indicate if the file is for the Hypervisor ("PHYP") or a VIOS ("vios_1", "vios_2")
		for _, cat := range entry.FindElements("category") {
			term := cat.SelectAttrValue("term", "")
			if term != "" {
				snap.Category = term
			}
		}

		snapshots = append(snapshots, snap)
	}

	if debug {
		c.Logger.Info("Successfully parsed STM metrics feed", "snapshotCount", len(snapshots))
	}

	return snapshots, nil
}

// FetchStmRawMetricsPayload downloads and unmarshals a highly granular 5-second STM JSON metrics file.
func (c *RestClient) FetchStmRawMetricsPayload(ctx context.Context, jsonURL string, debug bool) (*StmRawMetricsPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*") // Avoid 406 Not Acceptable errors

	if debug {
		c.Logger.Debug("Downloading STM Metrics JSON payload", "url", jsonURL)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transaction failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response payload: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected payload download with status: %s", resp.Status)
	}

	var payload StmRawMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to map raw STM telemetry to struct: %v", err)
	}

	return &payload, nil
}

// FetchStmRawViosMetricsPayload downloads and unmarshals the highly granular 5-second STM JSON metrics file specifically for Virtual I/O Servers.
func (c *RestClient) FetchStmRawViosMetricsPayload(ctx context.Context, jsonURL string, debug bool) (*StmRawViosMetricsPayload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request context: %v", err)
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "*/*")

	if debug {
		c.Logger.Debug("Downloading VIOS STM Metrics JSON payload", "url", jsonURL)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transaction failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response payload: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC rejected payload download with status: %s", resp.Status)
	}

	var payload StmRawViosMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to map raw VIOS STM telemetry to struct: %v", err)
	}

	return &payload, nil
}
