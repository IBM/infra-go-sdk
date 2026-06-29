package svc

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

var (
	svcIP   = flag.String("svc-ip", "", "SVC IP address (required for integration tests)")
	svcUser = flag.String("svc-user", "", "SVC username (required for integration tests)")
	svcPass = flag.String("svc-pass", "", "SVC password (required for integration tests)")
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	return &Client{
		Host:        "test.example.com",
		Port:        7443,
		Username:    "user",
		Password:    "pass",
		Token:       "test-token",
		TokenExpiry: time.Now().Add(10 * time.Minute),
		HTTPClient:  httpClient,
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDecodeIBMError_JSONBody(t *testing.T) {
	err := errors.New(`{"code":"CMMVC0001E","description":"bad request"}`)

	got := decodeIBMError(err)
	if got == nil {
		t.Fatal("expected decoded error, got nil")
	}

	want := "error CMMVC0001E: bad request"
	if got.Error() != want {
		t.Fatalf("unexpected decoded error: got %q want %q", got.Error(), want)
	}
}

func TestDecodeIBMError_NonJSONBody(t *testing.T) {
	err := errors.New("plain-text-error")

	got := decodeIBMError(err)
	if got == nil {
		t.Fatal("expected original error, got nil")
	}
	if got.Error() != "plain-text-error" {
		t.Fatalf("unexpected error passthrough: got %q", got.Error())
	}
}

func TestWithTLSInsecureSetsStateAndTransport(t *testing.T) {
	client := NewClient("test.example.com", "user", "pass")
	client.WithTLSInsecure()

	if !client.InsecureTLS {
		t.Fatal("expected InsecureTLS to be true")
	}

	transport, ok := client.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected HTTP transport to be *http.Transport")
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected TLS transport to skip verify")
	}
}

func TestMkhostValidation(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected HTTP call")
		return nil, nil
	}))

	if err := client.Mkhost(context.Background(), Host{}); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected missing name validation error, got %v", err)
	}

	if err := client.Mkhost(context.Background(), Host{Name: "host1"}); err == nil || !strings.Contains(err.Error(), "missing fcwwpn") {
		t.Fatalf("expected missing fcwwpn validation error, got %v", err)
	}
}

func TestMkhostPayloadFormatting(t *testing.T) {
	var body []byte
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/rest/mkhost" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if req.Header.Get("X-Auth-Token") != "test-token" {
			t.Fatalf("missing auth token header")
		}

		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}

		return jsonResponse(http.StatusOK, `{}`), nil
	}))

	err := client.Mkhost(context.Background(), Host{
		Name:     "host1",
		Fcwwpn:   []string{"10000000AAAA0001", "10000000AAAA0002"},
		Type:     "generic",
		Protocol: "scsi",
		Force:    true,
	})
	if err != nil {
		t.Fatalf("Mkhost returned error: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"name":"host1"`) {
		t.Fatalf("expected host name in payload: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"fcwwpn":"10000000AAAA0001:10000000AAAA0002"`) {
		t.Fatalf("expected joined fcwwpn in payload: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"force":true`) {
		t.Fatalf("expected force=true in payload: %s", bodyStr)
	}
}

func TestLsVdiskByNameParsesSingleItemArray(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/rest/lsvdisk/vol1" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `[{"id":"1","name":"vol1"}]`), nil
	}))

	vdisk, err := client.LsVdiskByName(context.Background(), "vol1")
	if err != nil {
		t.Fatalf("LsVdiskByName returned error: %v", err)
	}
	if vdisk == nil {
		t.Fatal("expected vdisk, got nil")
	}
	if vdisk.ID != "1" || vdisk.Name != "vol1" {
		t.Fatalf("unexpected vdisk parsed: %+v", vdisk)
	}
}

func TestLsVdiskByNameEmptyArray(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `[]`), nil
	}))

	_, err := client.LsVdiskByName(context.Background(), "missing-vol")
	if err == nil {
		t.Fatal("expected error for empty response array")
	}
	if !strings.Contains(err.Error(), "empty response array") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostReturnsRawIBMErrorForDecoding(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, `{"code":"CMMVC1234E","description":"bad input"}`), nil
	}))

	_, err := client.post(context.Background(), "failing-endpoint", map[string]interface{}{"x": "y"})
	if err == nil {
		t.Fatal("expected post error")
	}

	decoded := decodeIBMError(err)
	if decoded == nil {
		t.Fatal("expected decoded error")
	}
	if decoded.Error() != "error CMMVC1234E: bad input" {
		t.Fatalf("unexpected decoded error: %v", decoded)
	}
}

func TestPostSendsJSONPayload(t *testing.T) {
	var body bytes.Buffer
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", req.Method)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content-type: %s", req.Header.Get("Content-Type"))
		}
		if _, err := io.Copy(&body, req.Body); err != nil {
			t.Fatalf("failed reading body: %v", err)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}))

	_, err := client.post(context.Background(), "payload-check", map[string]interface{}{"name": "demo", "size": 10})
	if err != nil {
		t.Fatalf("post returned error: %v", err)
	}

	bodyStr := body.String()
	if !strings.Contains(bodyStr, `"name":"demo"`) || !strings.Contains(bodyStr, `"size":10`) {
		t.Fatalf("unexpected payload body: %s", bodyStr)
	}
}

func TestNewClientDefaults(t *testing.T) {
	client := NewClient("host", "user", "pass")

	if client.Port != defaultPort {
		t.Fatalf("unexpected default port: got %d want %d", client.Port, defaultPort)
	}
	if client.HTTPClient == nil {
		t.Fatal("expected HTTP client to be initialized")
	}
	if client.HTTPClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("unexpected default timeout: got %v want %v", client.HTTPClient.Timeout, defaultHTTPTimeout)
	}
}

func TestWithHTTPTimeout(t *testing.T) {
	client := NewClient("host", "user", "pass")
	client.WithHTTPTimeout(42 * time.Second)

	if client.HTTPClient.Timeout != 42*time.Second {
		t.Fatalf("unexpected timeout: got %v", client.HTTPClient.Timeout)
	}
}

func TestEnsureTokenValidRefreshesExpiredToken(t *testing.T) {
	client := NewClient("test.example.com", "user", "pass")
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/rest/auth" {
				t.Fatalf("unexpected auth path: %s", req.URL.Path)
			}
			return jsonResponse(http.StatusOK, `{"token":"new-token"}`), nil
		}),
		Timeout: 5 * time.Second,
	}
	client.Token = "old-token"
	client.TokenExpiry = time.Now().Add(-1 * time.Minute)

	token, err := client.ensureTokenValid(context.Background(), client.HTTPClient)
	if err != nil {
		t.Fatalf("ensureTokenValid returned error: %v", err)
	}
	if token != "new-token" {
		t.Fatalf("expected token 'new-token', got %q", token)
	}
	if client.Token != "new-token" {
		t.Fatalf("expected refreshed token, got %q", client.Token)
	}
}


func TestLsfcmapByNameParsesSingleObject(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/rest/lsfcmap/fcmap1" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{"id":"1","name":"fcmap1","source_vdisk_name":"src1","target_vdisk_name":"tgt1"}`), nil
	}))

	mappings, err := client.Lsfcmap(context.Background(), "fcmap1")
	if err != nil {
		t.Fatalf("Lsfcmap returned error: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
	if mappings[0].ID != "1" || mappings[0].Name != "fcmap1" {
		t.Fatalf("unexpected mapping parsed: %+v", mappings[0])
	}
}

func TestLsfcmapByNameFallsBackToArrayAndFiltersByName(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `[{"id":"1","name":"other"},{"id":"2","name":"fcmap2","source_vdisk_name":"src2","target_vdisk_name":"tgt2"}]`), nil
	}))

	mappings, err := client.Lsfcmap(context.Background(), "fcmap2")
	if err != nil {
		t.Fatalf("Lsfcmap returned error: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
	if mappings[0].ID != "2" || mappings[0].Name != "fcmap2" {
		t.Fatalf("unexpected filtered mapping parsed: %+v", mappings[0])
	}
}

func TestLsfcconsistgrpByNameParsesSingleObject(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/rest/lsfcconsistgrp/grp1" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{"id":"10","name":"grp1","status":"idle"}`), nil
	}))

	groups, err := client.Lsfcconsistgrp(context.Background(), "grp1")
	if err != nil {
		t.Fatalf("Lsfcconsistgrp returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ID != "10" || groups[0].Name != "grp1" {
		t.Fatalf("unexpected group parsed: %+v", groups[0])
	}
	if len(groups[0].Mappings) != 0 {
		t.Fatalf("expected no mappings for single-object response, got %+v", groups[0].Mappings)
	}
}

func TestLsfcconsistgrpByNameFallsBackToMixedArray(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `[{"id":"10","name":"grp1","status":"copying"},{"FC_mapping_id":"21","FC_mapping_name":"fcmapA"},{"FC_mapping_id":"22","FC_mapping_name":"fcmapB"}]`), nil
	}))

	groups, err := client.Lsfcconsistgrp(context.Background(), "grp1")
	if err != nil {
		t.Fatalf("Lsfcconsistgrp returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	group := groups[0]
	if group.ID != "10" || group.Name != "grp1" {
		t.Fatalf("unexpected group parsed: %+v", group)
	}
	if len(group.Mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(group.Mappings))
	}
	if group.Mappings[0].FCMappingID != "21" || group.Mappings[0].FCMappingName != "fcmapA" {
		t.Fatalf("unexpected first mapping: %+v", group.Mappings[0])
	}
	if group.Mappings[1].FCMappingID != "22" || group.Mappings[1].FCMappingName != "fcmapB" {
		t.Fatalf("unexpected second mapping: %+v", group.Mappings[1])
	}
}

func TestMkfcmapPayloadOmitsFalseBooleansAndIncludesTrueOnes(t *testing.T) {
	var body []byte
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/rest/mkfcmap" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}))

	copyRate := 25
	err := client.Mkfcmap(context.Background(), FlashCopyMapping{
		Name:        "fcmap1",
		Source:      "src1",
		Target:      "tgt1",
		ConsistGrp:  "grp1",
		AutoDelete:  true,
		Incremental: true,
		KeepTarget:  false,
		CopyRate:    &copyRate,
	})
	if err != nil {
		t.Fatalf("Mkfcmap returned error: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"name":"fcmap1"`) {
		t.Fatalf("expected name in payload: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"source":"src1"`) || !strings.Contains(bodyStr, `"target":"tgt1"`) {
		t.Fatalf("expected source/target in payload: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"autodelete":true`) {
		t.Fatalf("expected autodelete=true in payload: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"incremental":true`) {
		t.Fatalf("expected incremental=true in payload: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"keeptarget":false`) || strings.Contains(bodyStr, `"keeptarget":true`) {
		t.Fatalf("expected keeptarget to be omitted when false: %s", bodyStr)
	}
}

func TestMkfcconsistgrpPayloadOmitsFalseAutodelete(t *testing.T) {
	var body []byte
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/rest/mkfcconsistgrp" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}))

	err := client.Mkfcconsistgrp(context.Background(), FlashCopyConsistGroup{
		Name:       "grp1",
		AutoDelete: false,
	})
	if err != nil {
		t.Fatalf("Mkfcconsistgrp returned error: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, `"name":"grp1"`) {
		t.Fatalf("expected name in payload: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"autodelete":false`) || strings.Contains(bodyStr, `"autodelete":true`) {
		t.Fatalf("expected autodelete to be omitted when false: %s", bodyStr)
	}
}

func TestAuthenticateFailure(t *testing.T) {
	client := NewClient("test.example.com", "user", "pass")
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, `unauthorized`), nil
		}),
		Timeout: 5 * time.Second,
	}

	err := client.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected authenticate error")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("unexpected auth error: %v", err)
	}
}

func TestPostPropagatesTokenRefreshFailure(t *testing.T) {
	client := NewClient("test.example.com", "user", "pass")
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
		Timeout: 5 * time.Second,
	}
	client.Token = ""
	client.TokenExpiry = time.Time{}

	_, err := client.post(context.Background(), "anything", nil)
	if err == nil {
		t.Fatal("expected post error")
	}
	if !strings.Contains(err.Error(), "token refresh failed") {
		t.Fatalf("unexpected post error: %v", err)
	}
}

func TestWithPort(t *testing.T) {
	client := NewClient("host", "user", "pass")
	client.WithPort(9443)

	if client.Port != 9443 {
		t.Fatalf("unexpected port: got %d", client.Port)
	}
}

func TestBaseURL(t *testing.T) {
	client := NewClient("example.com", "user", "pass")
	client.WithPort(7443)

	if got := client.baseURL(); got != "https://example.com:7443" {
		t.Fatalf("unexpected baseURL: %s", got)
	}
}

func TestDecodeIBMErrorNil(t *testing.T) {
	if err := decodeIBMError(nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestPostReturnsTransportError(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("transport failure")
	}))

	_, err := client.post(context.Background(), "transport-fail", nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if !strings.Contains(err.Error(), "transport failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}


// Made with Bob

func TestSVCAuthenticationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SVC integration test in short mode")
	}

	client := NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate failed against SVC %s: %v", *svcIP, err)
	}
	if client.Token == "" {
		t.Fatal("expected token after authentication")
	}
	if time.Now().After(client.TokenExpiry) {
		t.Fatalf("expected future token expiry, got %v", client.TokenExpiry)
	}
}

func TestSVCLssystemIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SVC integration test in short mode")
	}

	client := NewClient(*svcIP, *svcUser, *svcPass).WithTLSInsecure()

	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate failed against SVC %s: %v", *svcIP, err)
	}

	systemInfo, err := client.Lssystem(context.Background())
	if err != nil {
		t.Fatalf("Lssystem failed against SVC %s: %v", *svcIP, err)
	}
	if systemInfo == nil {
		t.Fatal("expected system info, got nil")
	}
	if systemInfo.Name == "" && systemInfo.ID == "" {
		t.Fatalf("expected non-empty system identity, got %+v", systemInfo)
	}
}

func TestContextCancellation(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Simulate a slow operation
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(100 * time.Millisecond):
			return jsonResponse(http.StatusOK, `[{"id":"1","name":"host1"}]`), nil
		}
	}))

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Lshost(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context cancellation error, got: %v", err)
	}
}

func TestContextTimeout(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Simulate a slow operation that takes longer than the timeout
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(200 * time.Millisecond):
			return jsonResponse(http.StatusOK, `{"id":"1","name":"system1"}`), nil
		}
	}))

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Lssystem(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected context deadline exceeded error, got: %v", err)
	}
}

func TestContextPropagation(t *testing.T) {
	var receivedContext context.Context
	client := newTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		receivedContext = req.Context()
		return jsonResponse(http.StatusOK, `[]`), nil
	}))

	// Create a context with a value
	type contextKey string
	key := contextKey("test-key")
	ctx := context.WithValue(context.Background(), key, "test-value")

	_, _ = client.LsVdisk(ctx)

	if receivedContext == nil {
		t.Fatal("expected context to be propagated to HTTP request")
	}

	// Verify the context value was propagated
	if val := receivedContext.Value(key); val != "test-value" {
		t.Fatalf("expected context value to be propagated, got: %v", val)
	}
}
