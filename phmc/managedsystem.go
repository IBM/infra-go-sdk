package hmc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// GetManagedSystemQuick retrieves a surgical JSON summary of a single system.
func (c *RestClient) GetManagedSystemQuick(ctx context.Context, systemUUID string, debug bool) (*ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s/quick", c.hmcIP, systemUUID)

	if debug {
		c.Logger.Debug("Fetching managed system quick summary", "systemUUID", systemUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Ensure we get the raw JSON object
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("HMC error %d: %s", resp.StatusCode, string(body))
	}

	var system ManagedSystemQuick
	if err := json.Unmarshal(body, &system); err != nil {
		c.Logger.Error("Failed to unmarshal exhaustive JSON", "error", err)
		return nil, fmt.Errorf("failed to unmarshal exhaustive JSON: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully captured all elements for system", "systemName", system.SystemName)
	}

	return &system, nil
}

// GetManagedSystemByName fetches the managed system UUID and comprehensive details by its friendly name.
func (c *RestClient) GetManagedSystemByName(ctx context.Context, systemName string, debug bool) (string, *ManagedSystemDetailed, error) {
	if systemName == "" {
		return "", nil, fmt.Errorf("systemName cannot be empty")
	}

	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/search/(SystemName=='%s')", c.hmcIP, systemName)
	if debug {
		c.Logger.Debug("Fetching comprehensive managed system by name", "systemName", systemName, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	// Using atom+xml to ensure we get the proper feed/entry wrapper that the search endpoint returns
	req.Header.Set("Accept", "application/atom+xml")
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if debug {
		c.Logger.Debug("GetManagedSystemByName response status", "status", resp.Status)
	}

	if resp.StatusCode == 204 {
		if debug {
			c.Logger.Debug("No managed system found for name", "systemName", systemName)
		}
		return "", nil, nil // No content found
	}
	if resp.StatusCode != 200 {
		c.Logger.Error("Unexpected status code", "status", resp.StatusCode)
		return "", nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetManagedSystemByName API response body", "body", string(body))
	}

	// 1. Strip the namespaces using your helper
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to strip namespaces from XML response: %v", err)
	}

	// 2. Extract the UUID from the AtomID field
	uuidElem := doc.FindElement("//AtomID")
	if uuidElem == nil {
		return "", nil, fmt.Errorf("AtomID not found in response")
	}
	uuid := uuidElem.Text()

	// 3. Extract ONLY the core ManagedSystem element
	msElem := doc.FindElement("//ManagedSystem")
	if msElem == nil {
		return "", nil, fmt.Errorf("ManagedSystem not found in response")
	}

	// 4. Serialize the isolated element back to bytes
	msDoc := etree.NewDocument()
	msDoc.SetRoot(msElem.Copy())
	msBytes, err := msDoc.WriteToBytes()
	if err != nil {
		return "", nil, fmt.Errorf("failed to serialize isolated ManagedSystem element: %v", err)
	}

	// 5. Unmarshal directly into our exhaustive Go struct
	var detailedSystem ManagedSystemDetailed
	if err := xml.Unmarshal(msBytes, &detailedSystem); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal XML into ManagedSystemDetailed struct: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully resolved System to UUID and parsed exhaustive configuration", "systemName", systemName, "uuid", uuid)
	}

	return uuid, &detailedSystem, nil
}

// GetMaximumPartitions retrieves the MaximumPartitions for a system by UUID
func (c *RestClient) GetMaximumPartitions(ctx context.Context, systemUUID string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)

	if debug {
		c.Logger.Debug("Fetching MaximumPartitions", "systemUUID", systemUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")
	req.Header.Set("X-API-Session", c.session)

	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status)
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	var system System
	if err := xml.Unmarshal(body, &system); err != nil {
		c.Logger.Error("XML unmarshal failed", "error", err)
		return "", fmt.Errorf("XML unmarshal failed: %v", err)
	}

	if system.MaxPartitions == "" {
		return "", fmt.Errorf("MaximumPartitions not found for system %s", systemUUID)
	}

	if debug {
		c.Logger.Info("Successfully retrieved MaximumPartitions", "maxPartitions", system.MaxPartitions)
	}

	return system.MaxPartitions, nil
}

// GetManagedSystems retrieves the list of managed systems as an XML document
func (c *RestClient) GetManagedSystems(ctx context.Context, debug bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem", c.hmcIP)
	if debug {
		c.Logger.Debug("Fetching managed systems", "url", url)
	}

	// Create and configure the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.uom+xml;type=ManagedSystem")

	timeoutCtx, cancel := context.WithTimeout(ctx, 3600*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if debug {
		c.Logger.Debug("GetManagedSystems response status", "status", resp.Status)
	}

	// Handle 204 No Content
	if resp.StatusCode == http.StatusNoContent {
		if debug {
			c.Logger.Debug("No managed systems found (204 No Content)")
		}
		return nil, nil
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("request failed with status: %s, body: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status: %s. Enable debug mode to see full response", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	if debug {
		c.Logger.Debug("GetManagedSystems response body", "body", string(body))
	}

	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	managedSystems := doc.FindElement("//ManagedSystem")
	if managedSystems == nil {
		return nil, fmt.Errorf("ManagedSystem element not found in response")
	}

	if debug {
		c.Logger.Info("Successfully retrieved managed systems XML")
	}

	return managedSystems, nil
}

// GetManagedSystemQuickAll fetches all systems using the high-performance JSON endpoint.
func (c *RestClient) GetManagedSystemQuickAll(ctx context.Context, debug bool) ([]ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/quick/All", c.hmcIP)

	if debug {
		c.Logger.Debug("Fetching all managed systems via Quick endpoint", "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Request JSON explicitly
	req = req.WithContext(ctx)

	c.logRawTraffic("REQUEST (GET)", url, "")

	resp, err := c.client.Do(req)
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.Logger.Error("HMC error", "status", resp.Status, "body", string(body))
		return nil, fmt.Errorf("HMC error (%s): %s", resp.Status, string(body))
	}

	// We decode directly since we don't need to read body string for logging unless requested,
	// but to keep wire logging consistent we should read it
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.logRawTraffic("RESPONSE", url, string(body))

	var systems []ManagedSystemQuick
	if err := json.Unmarshal(body, &systems); err != nil {
		c.Logger.Error("Failed to decode Quick/All JSON", "error", err)
		return nil, fmt.Errorf("failed to decode Quick/All JSON: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully fetched all systems via Quick endpoint", "count", len(systems))
	}

	return systems, nil
}

// GetManagedSystem retrieves the comprehensive, deeply parsed XML details of a Managed System.
func (c *RestClient) GetManagedSystem(ctx context.Context, systemUUID string, debug bool) (*ManagedSystemDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)
	if debug {
		c.Logger.Debug("Fetching comprehensive XML details for managed system", "systemUUID", systemUUID, "url", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	c.logRawTraffic("REQUEST (GET)", url, "")

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

	c.logRawTraffic("RESPONSE", url, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Request failed", "status", resp.Status)
		if debug {
			return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("request failed with status %s. Enable debug mode to see full response", resp.Status)
	}

	// 1. Strip the namespaces using the existing helper to make unmarshaling clean
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// 2. Extract ONLY the core ManagedSystem element (bypassing the <entry> atom wrapper)
	msElem := doc.FindElement("//ManagedSystem")
	if msElem == nil {
		return nil, fmt.Errorf("ManagedSystem root element not found in XML response")
	}

	// 3. Serialize the isolated element back to bytes
	msDoc := etree.NewDocument()
	msDoc.SetRoot(msElem.Copy())
	msBytes, err := msDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize isolated ManagedSystem element: %v", err)
	}

	// 4. Unmarshal directly into our comprehensive Go struct
	var detailedSystem ManagedSystemDetailed
	if err := xml.Unmarshal(msBytes, &detailedSystem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML into ManagedSystemDetailed struct: %v", err)
	}

	if debug {
		c.Logger.Info("Successfully parsed comprehensive details for System", "systemName", detailedSystem.SystemName)
	}

	return &detailedSystem, nil
}

// CliRunner executes an OS-level command by tunneling it through the HMC CLIRunner job.
// This can be used to run HMC CLI commands or viosvrcmd commands to execute commands on VIOS partitions.
// It returns the stdout of the command as a string, and an error if the job fails.
func (c *RestClient) CliRunner(ctx context.Context, cmdString string, debug bool) (string, error) {
	// 1. Fetch the Management Console UUID
	mcURL := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole", c.hmcIP)

	if debug {
		c.Logger.Debug("Fetching Management Console UUID", "url", mcURL)
	}

	req, err := http.NewRequest("GET", mcURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	c.logRawTraffic("REQUEST (GET)", mcURL, "")

	resp, err := c.client.Do(req.WithContext(timeoutCtx))
	if err != nil {
		c.Logger.Error("Failed to fetch Management Console", "error", err)
		return "", fmt.Errorf("failed to fetch Management Console: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	c.logRawTraffic("RESPONSE", mcURL, string(body))

	if resp.StatusCode != http.StatusOK {
		c.Logger.Error("Failed to get Management Console", "status", resp.StatusCode)
		if debug {
			return "", fmt.Errorf("failed to get Management Console (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("failed to get Management Console (HTTP %d). Enable debug mode to see full response", resp.StatusCode)
	}

	mcDoc, err := xmlStripNamespace(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse Management Console XML: %v", err)
	}

	// Extract the Management Console UUID
	var mcUUID string
	if entryElem := mcDoc.FindElement("//entry/id"); entryElem != nil {
		mcUUID = entryElem.Text()
	} else if uuidElem := mcDoc.FindElement("//ManagementConsole/Metadata/Atom/AtomID"); uuidElem != nil {
		mcUUID = uuidElem.Text()
	} else if uuidElem := mcDoc.FindElement("//UUID"); uuidElem != nil {
		mcUUID = uuidElem.Text()
	}

	if mcUUID == "" {
		if debug {
			c.Logger.Debug("Management Console response", "body", string(body))
		}
		return "", fmt.Errorf("could not resolve Management Console UUID from response")
	}

	if debug {
		c.Logger.Debug("Resolved Management Console UUID", "mcUUID", mcUUID)
	}

	// 2. Target the Management Console's CLIRunner Job endpoint
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole/%s/do/CLIRunner", c.hmcIP, mcUUID)

	if debug {
		c.Logger.Debug("Executing HMC CLI Command", "cmdString", cmdString, "url", url)
	}

	// 3. Construct the CLIRunner Job Payload
	payload := fmt.Sprintf(`<JobRequest:JobRequest xmlns:JobRequest="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
       <Metadata>
           <Atom/>
       </Metadata>
       <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
           <Metadata>
               <Atom/>
           </Metadata>
           <OperationName kxe="false" kb="ROR">CLIRunner</OperationName>
           <GroupName kxe="false" kb="ROR">ManagementConsole</GroupName>
       </RequestedOperation>
       <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
           <Metadata>
               <Atom/>
           </Metadata>
           <JobParameter schemaVersion="V1_0">
               <Metadata>
                   <Atom/>
               </Metadata>
               <ParameterName kxe="false" kb="ROR">cmd</ParameterName>
               <ParameterValue kxe="false" kb="CUR">%s</ParameterValue>
           </JobParameter>
           <JobParameter schemaVersion="V1_0">
               <Metadata>
                   <Atom/>
               </Metadata>
               <ParameterName kxe="false" kb="ROR">acknowledgeThisAPIMayGoAwayInTheFuture</ParameterName>
               <ParameterValue kxe="false" kb="CUR">true</ParameterValue>
           </JobParameter>
       </JobParameters>
</JobRequest:JobRequest>`, cmdString)

	// 4. Submit the JobRequest via PUT
	req2, err := http.NewRequest("PUT", url, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create CLIRunner request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req2.Header.Set("X-API-Session", c.session)
	req2.Header.Set("Accept", "application/atom+xml")

	timeoutCtx2, cancel2 := context.WithTimeout(ctx, 120*time.Second)
	defer cancel2()

	c.logRawTraffic("REQUEST (PUT)", url, payload)

	resp2, err := c.client.Do(req2.WithContext(timeoutCtx2))
	if err != nil {
		c.Logger.Error("HTTP request failed", "error", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read CLIRunner response: %v", err)
	}

	c.logRawTraffic("RESPONSE", url, string(body2))

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusAccepted && resp2.StatusCode != http.StatusCreated {
		c.Logger.Error("CLIRunner failed", "status", resp2.Status, "body", string(body2))
		return "", fmt.Errorf("CLIRunner failed with status %s: %s", resp2.Status, string(body2))
	}

	doc2, err := xmlStripNamespace(body2)
	if err != nil {
		return "", fmt.Errorf("failed to parse CLIRunner response XML: %v", err)
	}

	jobIDElem := doc2.FindElement("//JobID")
	if jobIDElem == nil {
		jobIDElem = doc2.FindElement("//JobResponse/JobID")
	}

	var cmdOutput string

	if jobIDElem != nil {
		jobID := jobIDElem.Text()
		if debug {
			c.Logger.Info("CLIRunner Job submitted, waiting for completion...", "jobID", jobID)
		}

		// 5. Wait for job completion and capture the resulting document
		jobResp, err := c.FetchJobStatus(ctx, jobID, false, 10, debug)
		if err != nil {
			return "", fmt.Errorf("CLIRunner job failed: %v", err)
		}

		// 6. Extract the stdout from the Job Results parameters
		if jobResp != nil {
			for _, param := range jobResp.Results.Parameters {
				// FIX: Support both "stdout" and "result" parameter names to handle HMC firmware variations
				if param.ParameterName == "stdout" || param.ParameterName == "result" {
					if cmdOutput == "" {
						cmdOutput = param.ParameterValue
					} else {
						cmdOutput += "\n" + param.ParameterValue
					}
				} else if param.ParameterName == "stderr" && param.ParameterValue != "" && debug {
					c.Logger.Warn("CLIRunner stderr output", "stderr", param.ParameterValue)
				}
			}
		}

		if debug {
			c.Logger.Info("CLIRunner job completed successfully")
		}
	} else {
		return "", fmt.Errorf("JobID not found in CLIRunner response: %s", string(body2))
	}

	return cmdOutput, nil
}

// generateVIOSAdminPassword creates a deterministic password based on HMC IP
// This ensures the same password is used across all PowerShift invocations
func (c *RestClient) generateVIOSAdminPassword() string {
	salt := "PowerShift-VIOS-Admin-2026"
	data := c.hmcIP + salt

	hash := sha256.Sum256([]byte(data))
	// Convert to base64 and take first 20 characters
	password := base64.StdEncoding.EncodeToString(hash[:])[:20]

	// Add special characters to meet HMC password requirements
	return password + "!Ps"
}

// GetVIOSAdminCredentials returns the viosadmin credentials
// Assumes viosadmin user already exists on HMC
func (c *RestClient) GetVIOSAdminCredentials() (username, password string) {
	return "viosadmin", c.generateVIOSAdminPassword()
}

// CheckVIOSAdminUser checks if viosadmin user exists on HMC
// Returns true if user exists, false otherwise
func (c *RestClient) CheckVIOSAdminUser(ctx context.Context, hmcUsername, hmcPassword string, debug bool) (bool, error) {
	// Use lshmcusr with --filter to check if viosadmin exists
	// Format: lshmcusr --filter "names=viosadmin"
	cmd := `lshmcusr --filter "names=viosadmin"`

	if debug {
		c.Logger.Debug("Checking if viosadmin user exists on HMC")
	}

	output, err := CliRunnerViaSSH(c.hmcIP, hmcUsername, hmcPassword, cmd, debug)
	if err != nil {
		// If lshmcusr fails, return error
		if debug {
			c.Logger.Debug("Failed to check viosadmin user", "error", err)
		}
		return false, fmt.Errorf("lshmcusr command failed: %w", err)
	}

	// If output contains "name=viosadmin", user exists
	if strings.Contains(output, "name=viosadmin") {
		if debug {
			c.Logger.Debug("viosadmin user exists on HMC")
		}
		return true, nil
	}

	// Empty output means user doesn't exist
	if debug {
		c.Logger.Debug("viosadmin user does not exist")
	}
	return false, nil
}

// EnsureVIOSAdminUser checks if viosadmin user exists, creates it if not
// Returns the username, password, and whether the user was created (true) or already existed (false)
func (c *RestClient) EnsureVIOSAdminUser(ctx context.Context, hmcUsername, hmcPassword string, debug bool) (username, password string, created bool, err error) {
	username, password = c.GetVIOSAdminCredentials()

	// Check if user already exists
	exists, err := c.CheckVIOSAdminUser(ctx, hmcUsername, hmcPassword, debug)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to check viosadmin user: %w", err)
	}

	if exists {
		if debug {
			c.Logger.Info("viosadmin user already exists, using existing credentials")
		}
		return username, password, false, nil
	}

	// User doesn't exist, create it with proper VIOS admin privileges
	c.Logger.Info("Creating viosadmin user on HMC with VIOS admin role")

	// Step 1: Create custom VIOS_Admin role with ViosAdminOp permission
	// This role is cloned from hmcsuperadmin but includes ViosAdminOp permission
	roleCmd := `mkaccfg -t taskrole -i 'name=VIOS_Admin,parent=hmcsuperadmin,"resources=lpar:ViosAdminOp"'`

	if debug {
		c.Logger.Debug("Creating VIOS_Admin role")
	}

	_, err = CliRunnerViaSSH(c.hmcIP, hmcUsername, hmcPassword, roleCmd, debug)
	if err != nil {
		// Role might already exist, log warning but continue
		if debug {
			c.Logger.Debug("VIOS_Admin role creation failed (may already exist)", "error", err)
		}
	}

	// Step 2: Create viosadmin user with VIOS_Admin role
	createCmd := fmt.Sprintf(`mkhmcusr -u %s -a VIOS_Admin --passwd %s`, username, password)

	if debug {
		c.Logger.Debug("Creating viosadmin user with VIOS_Admin role", "username", username)
	}

	_, err = CliRunnerViaSSH(c.hmcIP, hmcUsername, hmcPassword, createCmd, debug)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to create viosadmin user: %w", err)
	}

	c.Logger.Info("✓ Successfully created viosadmin user on HMC with VIOS admin privileges", "username", username)

	return username, password, true, nil
}

// RunVIOSCommandAsAdmin executes a viosvrcmd with --admin flag using viosadmin credentials
// Uses SSH to authenticate as viosadmin user
func (c *RestClient) RunVIOSCommandAsAdmin(ctx context.Context, systemName, viosName, command string, debug bool) (string, error) {
	viosAdminUser, viosAdminPass := c.GetVIOSAdminCredentials()

	// Build the full viosvrcmd command with --admin flag
	viosCmd := fmt.Sprintf(`viosvrcmd -m %s -p %s -c "%s" --admin`,
		systemName, viosName, command)

	// Use sshpass to execute command as viosadmin
	sshCmd := fmt.Sprintf(`sshpass -p '%s' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null %s@%s '%s'`,
		viosAdminPass, viosAdminUser, c.hmcIP, viosCmd)

	if debug {
		c.Logger.Debug("Executing viosvrcmd as viosadmin",
			"user", viosAdminUser,
			"system", systemName,
			"vios", viosName,
			"command", command)
	}

	// Execute via CliRunner (which will run the SSH command)
	return c.CliRunner(ctx, sshCmd, debug)
}

// GetSRIOVAdapters fetches the detailed Managed System configuration and extracts all SR-IOV adapters.
// It bypasses the IBM Atom <entry> wrapper and unmarshals natively into ManagedSystemDetailed.
func (c *RestClient) GetSRIOVAdapters(ctx context.Context, sysUUID string, debug bool) ([]SRIOVAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, sysUUID)

	if debug {
		c.Logger.Debug("Fetching Managed System to extract SR-IOV Adapters", "sysUUID", sysUUID)
	}

	// 1. Fetch and strip namespaces into an etree Document
	doc, err := c.fetchAndParseHMCXML(ctx, url, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Managed System configuration: %v", err)
	}

	// 2. ✨ THE FIX: Bypass the <entry><content> wrapper and target the actual payload
	sysElem := doc.FindElement("//ManagedSystem")
	if sysElem == nil {
		return nil, fmt.Errorf("ManagedSystem element not found in HMC response")
	}

	// 3. Convert only the ManagedSystem block back to bytes
	tempDoc := etree.NewDocument()
	tempDoc.SetRoot(sysElem.Copy())
	sysBytes, _ := tempDoc.WriteToBytes()

	// 4. Safely Unmarshal into your comprehensive struct
	var sysDetailed ManagedSystemDetailed
	if err := xml.Unmarshal(sysBytes, &sysDetailed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Managed System XML: %v", err)
	}

	if debug {
		c.Logger.Debug("Successfully extracted SR-IOV Adapters", "sysUUID", sysUUID, "count", len(sysDetailed.SRIOVAdapters))
	}

	if debug {
		c.Logger.Debug("Successfully extracted SR-IOV Adapters", "sysUUID", sysUUID, "count", len(sysDetailed.IOConfig.SRIOVAdapters))
	}

	// 4. Return from the nested IOConfig!
	return sysDetailed.IOConfig.SRIOVAdapters, nil
}
