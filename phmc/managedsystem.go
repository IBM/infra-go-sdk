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


	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Ensure we get the raw JSON object
	req = req.WithContext(ctx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}


	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HMC error %d: %s", resp.StatusCode, string(body))
	}

	var system ManagedSystemQuick
	if err := json.Unmarshal(body, &system); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exhaustive JSON: %v", err)
	}


	return &system, nil
}

// GetManagedSystemByName fetches the managed system UUID and comprehensive details by its friendly name.
func (c *RestClient) GetManagedSystemByName(ctx context.Context, systemName string, debug bool) (string, *ManagedSystemDetailed, error) {
	if systemName == "" {
		return "", nil, fmt.Errorf("systemName cannot be empty")
	}

	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/search/(SystemName=='%s')", c.hmcIP, systemName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	// Using atom+xml to ensure we get the proper feed/entry wrapper that the search endpoint returns
	req.Header.Set("Accept", "application/atom+xml")
	req = req.WithContext(ctx)


	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()


	if resp.StatusCode == 204 {
		return "", nil, nil // No content found
	}
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %v", err)
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


	return uuid, &detailedSystem, nil
}

// GetMaximumPartitions retrieves the MaximumPartitions for a system by UUID
func (c *RestClient) GetMaximumPartitions(ctx context.Context, systemUUID string, debug bool) (string, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)


	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.uom+xml; type=ManagedSystem")
	req.Header.Set("X-API-Session", c.session)

	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


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

// GetManagedSystems retrieves the list of managed systems as an XML document
func (c *RestClient) GetManagedSystems(ctx context.Context, debug bool) (*etree.Element, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem", c.hmcIP)

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


	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()


	// Handle 204 No Content
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
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



	// Parse XML response
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	managedSystems := doc.FindElement("//ManagedSystem")
	if managedSystems == nil {
		return nil, fmt.Errorf("ManagedSystem element not found in response")
	}


	return managedSystems, nil
}

// GetManagedSystemQuickAll fetches all systems using the high-performance JSON endpoint.
func (c *RestClient) GetManagedSystemQuickAll(ctx context.Context, debug bool) ([]ManagedSystemQuick, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/quick/All", c.hmcIP)


	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/json") // Request JSON explicitly
	req = req.WithContext(ctx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HMC error (%s): %s", resp.Status, string(body))
	}

	// We decode directly since we don't need to read body string for logging unless requested,
	// but to keep wire logging consistent we should read it
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}


	var systems []ManagedSystemQuick
	if err := json.Unmarshal(body, &systems); err != nil {
		return nil, fmt.Errorf("failed to decode Quick/All JSON: %v", err)
	}


	return systems, nil
}

// GetManagedSystem retrieves the comprehensive, deeply parsed XML details of a Managed System.
func (c *RestClient) GetManagedSystem(ctx context.Context, systemUUID string, debug bool) (*ManagedSystemDetailed, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, systemUUID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)


	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
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


	return &detailedSystem, nil
}

// CliRunner executes an OS-level command by tunneling it through the HMC CLIRunner job.
// This can be used to run HMC CLI commands or viosvrcmd commands to execute commands on VIOS partitions.
// It returns the stdout of the command as a string, and an error if the job fails.
func (c *RestClient) CliRunner(ctx context.Context, cmdString string, debug bool) (string, error) {
	// 1. Fetch the Management Console UUID
	mcURL := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole", c.hmcIP)


	req, err := http.NewRequest("GET", mcURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/atom+xml")

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()


	resp, err := c.client.Do(req.WithContext(timeoutCtx))
	if err != nil {
		return "", fmt.Errorf("failed to fetch Management Console: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}


	if resp.StatusCode != http.StatusOK {
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
		return "", fmt.Errorf("could not resolve Management Console UUID from response")
	}


	// 2. Target the Management Console's CLIRunner Job endpoint
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagementConsole/%s/do/CLIRunner", c.hmcIP, mcUUID)


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


	resp2, err := c.client.Do(req2.WithContext(timeoutCtx2))
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read CLIRunner response: %v", err)
	}


	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusAccepted && resp2.StatusCode != http.StatusCreated {
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
				}
			}
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


	output, err := CliRunnerViaSSH(c.hmcIP, hmcUsername, hmcPassword, cmd, debug)
	if err != nil {
		// If lshmcusr fails, return error
		return false, fmt.Errorf("lshmcusr command failed: %w", err)
	}

	// If output contains "name=viosadmin", user exists
	if strings.Contains(output, "name=viosadmin") {
		return true, nil
	}

	// Empty output means user doesn't exist
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
		return username, password, false, nil
	}

	// User doesn't exist, create it with proper VIOS admin privileges

	// Step 1: Create custom VIOS_Admin role with ViosAdminOp permission
	// This role is cloned from hmcsuperadmin but includes ViosAdminOp permission
	roleCmd := `mkaccfg -t taskrole -i 'name=VIOS_Admin,parent=hmcsuperadmin,"resources=lpar:ViosAdminOp"'`


	_, err = CliRunnerViaSSH(c.hmcIP, hmcUsername, hmcPassword, roleCmd, debug)
	if err != nil {
		// Role might already exist, log warning but continue
	}

	// Step 2: Create viosadmin user with VIOS_Admin role
	createCmd := fmt.Sprintf(`mkhmcusr -u %s -a VIOS_Admin --passwd %s`, username, password)


	_, err = CliRunnerViaSSH(c.hmcIP, hmcUsername, hmcPassword, createCmd, debug)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to create viosadmin user: %w", err)
	}


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


	// Execute via CliRunner (which will run the SSH command)
	return c.CliRunner(ctx, sshCmd, debug)
}

// GetSRIOVAdapters fetches the detailed Managed System configuration and extracts all SR-IOV adapters.
// It bypasses the IBM Atom <entry> wrapper and unmarshals natively into ManagedSystemDetailed.
func (c *RestClient) GetSRIOVAdapters(ctx context.Context, sysUUID string, debug bool) ([]SRIOVAdapter, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/ManagedSystem/%s", c.hmcIP, sysUUID)


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



	// 4. Return from the nested IOConfig!
	return sysDetailed.IOConfig.SRIOVAdapters, nil
}
