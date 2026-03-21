# API Improvement Recommendations

## Functions Returning `*etree.Document` Instead of Go Structures

The following public SDK functions currently return `*etree.Document`, which forces users to manually parse XML instead of working with proper Go structures:

### 1. Job Management Functions (`jobs.go`)

#### `FetchJobResponse(jobID string, verbose bool) (*etree.Document, error)`
**Current Issue:** Returns raw XML document
**Recommendation:** Create a `JobResponse` struct and return it instead

```go
type JobResponse struct {
    JobID          string
    Status         string
    PercentComplete int
    Results        map[string]string
    ErrorMessage   string
    // Add other relevant fields
}

func (c *HmcRestClient) FetchJobResponse(jobID string, verbose bool) (*JobResponse, error)
```

#### `FetchJobStatus(jobID string, template bool, timeoutInMin int, verbose bool) (*etree.Document, error)`
**Current Issue:** Returns raw XML document
**Recommendation:** Return `JobResponse` struct

```go
func (c *HmcRestClient) FetchJobStatus(jobID string, template bool, timeoutInMin int, verbose bool) (*JobResponse, error)
```

**Impact:** Users currently have to do this:
```go
doc, err := client.FetchJobStatus(jobID, false, 10, true)
statusElem := doc.FindElement("//Status")
status := statusElem.Text()
```

**Should be:**
```go
jobResp, err := client.FetchJobStatus(jobID, false, 10, true)
status := jobResp.Status
```

---

### 2. Partition Template Functions (`partitiontemplate.go`)

#### `TransformPartitionTemplate(draftUUID, cecUUID string, verbose bool) (*etree.Document, error)`
**Current Issue:** Returns raw XML job response
**Recommendation:** Return `JobResponse` struct or a specific `TransformResult` struct

```go
type TransformResult struct {
    JobID           string
    Status          string
    TransformedUUID string
    ErrorMessage    string
}

func (c *HmcRestClient) TransformPartitionTemplate(draftUUID, cecUUID string, verbose bool) (*TransformResult, error)
```

#### `CheckPartitionTemplate(templateName, cecUUID string, verbose bool) (*etree.Document, error)`
**Current Issue:** Returns raw XML validation response
**Recommendation:** Create a `TemplateValidationResult` struct

```go
type TemplateValidationResult struct {
    IsValid      bool
    Errors       []string
    Warnings     []string
    JobID        string
    Status       string
}

func (c *HmcRestClient) CheckPartitionTemplate(templateName, cecUUID string, verbose bool) (*TemplateValidationResult, error)
```

---

## Implementation Priority

### High Priority (User-Facing APIs)
1. `FetchJobStatus` - Most commonly used in examples
2. `TransformPartitionTemplate` - Used in partition creation workflows
3. `CheckPartitionTemplate` - Used in validation workflows

### Medium Priority
4. `FetchJobResponse` - Lower-level function, but still public

---

## Migration Strategy

To maintain backward compatibility:

1. **Create new functions with proper return types:**
   ```go
   // New function
   func (c *HmcRestClient) FetchJobStatusV2(jobID string, template bool, timeoutInMin int, verbose bool) (*JobResponse, error)
   
   // Keep old function but mark as deprecated
   // Deprecated: Use FetchJobStatusV2 instead. This function will be removed in v2.0.0
   func (c *HmcRestClient) FetchJobStatus(jobID string, template bool, timeoutInMin int, verbose bool) (*etree.Document, error)
   ```

2. **Or use a major version bump (v2.0.0)** and break compatibility:
   - Update all functions to return proper structs
   - Update all examples
   - Provide migration guide

---

## Benefits of This Change

1. **Type Safety:** Users get compile-time type checking instead of runtime XML parsing errors
2. **Better IDE Support:** Auto-completion and documentation for struct fields
3. **Easier Testing:** Mock structs are easier than mocking XML documents
4. **Cleaner Code:** No need for XPath queries in user code
5. **Better Error Handling:** Structured error information instead of parsing XML error messages
6. **API Consistency:** Most other functions already return proper Go structures

---

## Example: Current vs Proposed Usage

### Current (Bad)
```go
doc, err := client.FetchJobStatus(jobID, false, 10, true)
if err != nil {
    return err
}

// User must know XML structure and XPath
statusElem := doc.FindElement("//Status")
if statusElem == nil {
    return fmt.Errorf("status not found")
}
status := statusElem.Text()

percentElem := doc.FindElement("//PercentComplete")
percent, _ := strconv.Atoi(percentElem.Text())
```

### Proposed (Good)
```go
jobResp, err := client.FetchJobStatus(jobID, false, 10, true)
if err != nil {
    return err
}

// Clean, type-safe access
status := jobResp.Status
percent := jobResp.PercentComplete
```

---

## Internal Helper Functions (OK to Keep)

These functions are fine to keep as-is since they're internal:
- `xmlStripNamespace(xmlData []byte) (*etree.Document, error)` - Internal helper
- `fetchAndParseHMCXML(url string, verbose bool) (*etree.Document, error)` - Private helper

---

## Recommendation Summary

**Action Required:** Refactor the 4 public functions listed above to return proper Go structures instead of `*etree.Document`. This will significantly improve the developer experience and make the SDK more idiomatic Go code.

**Timeline Suggestion:**
- v1.x: Add new V2 functions, deprecate old ones
- v2.0: Remove deprecated functions, make new ones the default