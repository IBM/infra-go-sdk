package hmc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// FetchJobResponse retrieves the full job response and returns it as a structured JobResponseDetail
func (c *HmcRestClient) FetchJobResponse(jobID string, verbose bool) (*JobResponseDetail, error) {
	url := fmt.Sprintf("https://%s/rest/api/uom/jobs/%s", c.hmcIP, jobID)
	if verbose {
		hmcLogger.Printf("Fetching job response for JobID: %s, URL: %s", jobID, url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("X-API-Session", c.session)
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml; type=JobResponse")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if verbose {
		hmcLogger.Printf("Job response status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	if verbose {
		hmcLogger.Printf("Job response body:\n%s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	// Parse the XML document into JobResponseDetail struct
	jobResp := &JobResponseDetail{
		JobID:   jobID,
		Results: make(map[string]string),
	}

	// Extract Status
	if statusElem := doc.FindElement("//Status"); statusElem != nil {
		jobResp.Status = statusElem.Text()
	}

	// Extract PercentComplete
	if percentElem := doc.FindElement("//PercentComplete"); percentElem != nil {
		if percent, err := strconv.Atoi(percentElem.Text()); err == nil {
			jobResp.PercentComplete = percent
		}
	}

	// Extract TimeStarted
	if timeStartedElem := doc.FindElement("//TimeStarted"); timeStartedElem != nil {
		jobResp.TimeStarted = timeStartedElem.Text()
	}

	// Extract TimeCompleted
	if timeCompletedElem := doc.FindElement("//TimeCompleted"); timeCompletedElem != nil {
		jobResp.TimeCompleted = timeCompletedElem.Text()
	}

	// Extract Results (JobParameters)
	for _, param := range doc.FindElements("//Results/JobParameter") {
		nameElem := param.FindElement("ParameterName")
		valueElem := param.FindElement("ParameterValue")
		if nameElem != nil && valueElem != nil {
			paramName := strings.TrimSpace(nameElem.Text())
			paramValue := strings.TrimSpace(valueElem.Text())
			jobResp.Results[paramName] = paramValue
		}
	}

	// Extract error message if present
	if errMsgElem := doc.FindElement("//ResponseException//Message"); errMsgElem != nil {
		jobResp.ErrorMessage = errMsgElem.Text()
	}

	if verbose {
		hmcLogger.Printf("Parsed job response: Status=%s, PercentComplete=%d%%", jobResp.Status, jobResp.PercentComplete)
	}

	return jobResp, nil
}

// FetchJobStatus fetches the job status and response, waiting for completion or error
// Returns a structured JobResponseDetail instead of raw XML
func (c *HmcRestClient) FetchJobStatus(jobID string, template bool, timeoutInMin int, verbose bool) (*JobResponseDetail, error) {
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
	var jobResp *JobResponseDetail
	
	for i := 0; i < maxChecks; i++ {
		if i > 0 {
			time.Sleep(checkInterval)
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

		// Build JobResponseDetail struct
		jobResp = &JobResponseDetail{
			JobID:   jobID,
			Status:  jobStatus,
			Results: make(map[string]string),
		}

		// Extract PercentComplete
		if percentElem := doc.FindElement("//PercentComplete"); percentElem != nil {
			if percent, err := strconv.Atoi(percentElem.Text()); err == nil {
				jobResp.PercentComplete = percent
			}
		}

		// Extract TimeStarted
		if timeStartedElem := doc.FindElement("//TimeStarted"); timeStartedElem != nil {
			jobResp.TimeStarted = timeStartedElem.Text()
		}

		// Extract TimeCompleted
		if timeCompletedElem := doc.FindElement("//TimeCompleted"); timeCompletedElem != nil {
			jobResp.TimeCompleted = timeCompletedElem.Text()
		}

		// Extract Results (JobParameters)
		for _, param := range doc.FindElements("//Results/JobParameter") {
			nameElem := param.FindElement("ParameterName")
			valueElem := param.FindElement("ParameterValue")
			if nameElem != nil && valueElem != nil {
				paramName := strings.TrimSpace(nameElem.Text())
				paramValue := strings.TrimSpace(valueElem.Text())
				jobResp.Results[paramName] = paramValue
			}
		}

		// Log status if verbose
		if verbose {
			hmcLogger.Printf("Job status: %s (PercentComplete: %d%%)", jobStatus, jobResp.PercentComplete)
		}

		// Handle different job statuses
		switch jobStatus {
		case "COMPLETED_OK":
			if verbose {
				hmcLogger.Printf("Job completed successfully")
			}
			return jobResp, nil

		case "COMPLETED_WITH_ERROR":
			if verbose {
				hmcLogger.Printf("Job completed with error")
			}
			// Look for the 'result' parameter specifically
			var errMsg string
			if resultMsg, ok := jobResp.Results["result"]; ok {
				errMsg = resultMsg
			} else if len(jobResp.Results) > 0 {
				// Fallback to first result if 'result' not found
				for _, v := range jobResp.Results {
					errMsg = v
					break
				}
			}
			
			if errMsg != "" {
				jobResp.ErrorMessage = errMsg
				if verbose {
					hmcLogger.Printf("Error message: %s", errMsg)
				}
				return nil, fmt.Errorf("job completed with error: %s", errMsg)
			}
			return nil, fmt.Errorf("job completed with error, but no result message found")

		default:
			if jobStatus != "RUNNING" {
				if verbose {
					hmcLogger.Printf("Job failed with status: %s", jobStatus)
				}
				errMsgElem := doc.FindElement("//ResponseException//Message")
				if errMsgElem == nil {
					errMsgElem = doc.FindElement("//Results/JobParameter/ParameterValue")
				}
				if errMsgElem != nil {
					errMsg := errMsgElem.Text()
					jobResp.ErrorMessage = errMsg
					return nil, fmt.Errorf("job failed: %s", errMsg)
				}
				return nil, fmt.Errorf("job failed with status %s", jobStatus)
			}
			// If status is "RUNNING", continue looping
		}
	}

	// Timeout reached
	operationNameElem := doc.FindElement("//OperationName")
	if operationNameElem != nil {
		operationName := operationNameElem.Text()
		if verbose {
			hmcLogger.Printf("%s job stuck in %s state. Timed out!!", operationName, jobStatus)
		}
		return nil, fmt.Errorf("job %s timed out in state %s", operationName, jobStatus)
	}
	return nil, fmt.Errorf("job timed out")
}
