package hmc

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beevik/etree"
)

// FetchJobResponse retrieves the full job response and returns it as a structured JobResponse
func (c *RestClient) FetchJobResponse(ctx context.Context, jobID string) (*JobResponse, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/jobs/%s", c.hmcIP, jobID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml; type=JobResponse")

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	// Strip namespaces for easier parsing
	doc, err := xmlStripNamespace(body)
	if err != nil {
		return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
	}

	// Find the JobResponse element
	jobRespElem := doc.FindElement("//JobResponse")
	if jobRespElem == nil {
		return nil, fmt.Errorf("JobResponse element not found in response")
	}

	// Use XML unmarshaling to populate the struct
	var jobResp JobResponse

	// Create a new document with the JobResponse element as root
	jobRespDoc := etree.NewDocument()
	jobRespDoc.SetRoot(jobRespElem.Copy())

	jobRespBytes, err := jobRespDoc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize JobResponse element: %v", err)
	}

	if err := xml.Unmarshal(jobRespBytes, &jobResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JobResponse: %v", err)
	}

	return &jobResp, nil
}

// FetchJobStatus fetches the job status and response, waiting for completion or error.
// Returns a structured JobResponseDetail instead of raw XML.
//
// Supported Job Statuses:
//   - COMPLETED_OK: Job completed successfully (returns JobResponseDetail, nil)
//   - COMPLETED_WITH_WARNINGS: Job completed with warnings (returns JobResponseDetail, nil)
//   - COMPLETED_WITH_ERROR: Job completed with errors (returns nil, error)
//   - CANCELED_BEFORE_START: Job was canceled before starting (returns nil, error)
//   - CANCELED_WHILE_RUNNING: Job was canceled during execution (returns nil, error)
//   - FAILED_TO_START: Job failed to start (returns nil, error)
//   - FAILED_BEFORE_COMPLETION: Job failed during execution (returns nil, error)
//   - FAILED_BEFORE_COMPLETION_RETRY: Job failed but HMC will retry (continues polling)
//   - NOT_STARTED: Job queued but not started yet (continues polling)
//   - RUNNING: Job is executing (continues polling)
//
// Parameters:
//   - ctx: Context for cancellation support
//   - jobID: The job identifier to monitor
//   - template: If true, uses template API endpoint; if false, uses UOM endpoint
//   - timeoutInMin: Maximum time to wait for job completion (in minutes)
//   - debug: If true, logs detailed status information (unused, kept for API compatibility)
//
// Returns:
//   - *JobResponseDetail: Job response with status, results, and metadata (on success)
//   - error: Error if job fails, is canceled, or times out
//
// Reference: https://www.ibm.com/docs/en/power10/7063-CR1?topic=apis-job-status
func (c *RestClient) FetchJobStatus(ctx context.Context, jobID string, template bool, timeoutInMin int) (*JobResponse, error) {
	// Construct URL based on template flag
	var url string
	if template {
		url = fmt.Sprintf("https://%s/rest/api/templates/jobs/%s", c.hmcIP, jobID)
	} else {
		url = fmt.Sprintf("https://%s/rest/api/uom/jobs/%s", c.hmcIP, jobID)
	}

	// Set up headers
	headers := map[string]string{
		"X-API-Session": c.session,
		"Accept":        "application/atom+xml",
	}

	// Set up timeout mechanism
	maxChecks := timeoutInMin * 60 // Check every 1 second
	checkInterval := 1 * time.Second
	var jobStatus string // To use in timeout error message
	var doc *etree.Document
	var jobResp *JobResponse

	for i := 0; i < maxChecks; i++ {
		if i > 0 {
			// Use select to allow context cancellation during sleep
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("job monitoring canceled: %w", ctx.Err())
			case <-time.After(checkInterval):
				// Continue to next iteration
			}
		}

		// Check for context cancellation before making request
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("job monitoring canceled: %w", ctx.Err())
		default:
			// Continue with request
		}
		// Create and configure HTTP request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Execute request
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %v", err)
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close() // Close immediately after reading, not deferred
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}

		// Parse XML and strip namespaces
		doc, err = xmlStripNamespace(body)
		if err != nil {
			return nil, fmt.Errorf("failed to strip namespaces from XML: %v", err)
		}

		// Extract job status
		statusElem := doc.FindElement("//Status")
		if statusElem == nil {
			return nil, fmt.Errorf("Status element not found in response")
		}
		jobStatus = statusElem.Text()

		// Find the JobResponse element and unmarshal it
		jobRespElem := doc.FindElement("//JobResponse")
		if jobRespElem == nil {
			return nil, fmt.Errorf("JobResponse element not found in response")
		}

		// Use XML unmarshaling to populate the struct
		var jr JobResponse

		// Create a new document with the JobResponse element as root
		jobRespDoc := etree.NewDocument()
		jobRespDoc.SetRoot(jobRespElem.Copy())

		jobRespBytes, err := jobRespDoc.WriteToBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize JobResponse element: %v", err)
		}

		if err := xml.Unmarshal(jobRespBytes, &jr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JobResponse: %v", err)
		}

		jobResp = &jr
		jobStatus = jobResp.Status

		// Handle different job statuses according to IBM PowerVM HMC REST API documentation
		// Reference: https://www.ibm.com/docs/en/power10/7063-CR1?topic=apis-job-status
		switch jobStatus {
		case "COMPLETED_OK":
			return jobResp, nil

		case "COMPLETED_WITH_WARNINGS":
			// Return success - warnings are informational, not errors
			return jobResp, nil

		case "COMPLETED_WITH_ERROR":
			// Look for the 'result' parameter specifically
			var errMsg string
			for _, param := range jobResp.Results.Parameters {
				if param.ParameterName == "result" {
					errMsg = param.ParameterValue
					break
				}
			}
			// Fallback to first parameter if 'result' not found
			if errMsg == "" && len(jobResp.Results.Parameters) > 0 {
				errMsg = jobResp.Results.Parameters[0].ParameterValue
			}
			if errMsg != "" {
				return nil, fmt.Errorf("job completed with error: %s", errMsg)
			}
			return nil, fmt.Errorf("job completed with error, but no result message found")

		case "CANCELED_BEFORE_START":
			return nil, fmt.Errorf("job was canceled before it could start")

		case "CANCELED_WHILE_RUNNING":
			return nil, fmt.Errorf("job was canceled during execution")

		case "FAILED_TO_START":
			errMsgElem := doc.FindElement("//ResponseException//Message")
			if errMsgElem == nil {
				errMsgElem = doc.FindElement("//Results/JobParameter/ParameterValue")
			}
			if errMsgElem != nil {
				return nil, fmt.Errorf("job failed to start: %s", errMsgElem.Text())
			}
			return nil, fmt.Errorf("job failed to start")

		case "FAILED_BEFORE_COMPLETION":
			errMsgElem := doc.FindElement("//ResponseException//Message")
			if errMsgElem == nil {
				errMsgElem = doc.FindElement("//Results/JobParameter/ParameterValue")
			}
			if errMsgElem != nil {
				return nil, fmt.Errorf("job failed during execution: %s", errMsgElem.Text())
			}
			return nil, fmt.Errorf("job failed during execution")

		case "FAILED_BEFORE_COMPLETION_RETRY":
			// Job failed but HMC will retry — continue polling
			continue

		case "NOT_STARTED":
			// Job is queued but not running yet — continue polling
			continue

		case "RUNNING":
			// Job is currently executing — continue polling
			continue

		default:
			errMsgElem := doc.FindElement("//ResponseException//Message")
			if errMsgElem == nil {
				errMsgElem = doc.FindElement("//Results/JobParameter/ParameterValue")
			}
			if errMsgElem != nil {
				return nil, fmt.Errorf("job encountered unknown status '%s': %s", jobStatus, errMsgElem.Text())
			}
			return nil, fmt.Errorf("job encountered unknown status: %s", jobStatus)
		}
	}

	// Timeout reached
	operationNameElem := doc.FindElement("//OperationName")
	if operationNameElem != nil {
		return nil, fmt.Errorf("job %s timed out in state %s", operationNameElem.Text(), jobStatus)
	}
	return nil, fmt.Errorf("job timed out")
}

// DeleteJob deletes a job from the HMC.
// According to IBM documentation, after a job is completed, you must delete the job.
//
// Parameters:
//   - jobID: The job identifier to delete
//   - template: If true, uses template API endpoint; if false, uses UOM endpoint
//   - debug: Unused, kept for API compatibility
//
// Returns:
//   - error: Error if the deletion fails, nil on success
//
// Reference: https://www.ibm.com/docs/en/power10/7063-CR1?topic=apis-jobs
func (c *RestClient) DeleteJob(ctx context.Context, jobID string, template bool) error {
	// Construct URL based on template flag
	var url string
	if template {
		url = fmt.Sprintf("https://%s/rest/api/templates/jobs/%s", c.hmcIP, jobID)
	} else {
		url = fmt.Sprintf("https://%s/rest/api/uom/jobs/%s", c.hmcIP, jobID)
	}

	// Create DELETE request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %v", err)
	}

	// Set headers
	req.Header.Set("X-API-Session", c.session)

	// Set up timeout
	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req = req.WithContext(reqCtx)

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response body for error details if needed
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// DELETE typically returns 204 No Content on success, but may also return 200 OK
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if len(body) > 0 {
			return fmt.Errorf("failed to delete job (status %s)", resp.Status)
		}
		return fmt.Errorf("failed to delete job with status: %s", resp.Status)
	}

	return nil
}
