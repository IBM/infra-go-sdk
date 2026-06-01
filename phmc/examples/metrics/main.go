package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Mapped to package path
)

// =========================================================================
// UNIVERSAL LOCAL XML STRUCTS FOR FEED PARSING
// =========================================================================

// LocalAtomFeed represents an Atom feed containing multiple entries
type LocalAtomFeed struct {
	XMLName xml.Name         `xml:"feed"`
	Entries []LocalAtomEntry `xml:"entry"`
}

// LocalAtomEntry represents a single entry in an Atom feed with publication metadata
type LocalAtomEntry struct {
	Published string            `xml:"published"`
	Updated   string            `xml:"updated"`
	Category  LocalAtomCategory `xml:"category"`
	Links     []LocalAtomLink   `xml:"link"`
}

// LocalAtomCategory represents the category information of an Atom entry
type LocalAtomCategory struct {
	Term string `xml:"term,attr"`
}

// LocalAtomLink represents a link element in an Atom entry
type LocalAtomLink struct {
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

func main() {
	// =========================================================================
	// 1. COMMAND LINE FLAGS & INPUT VALIDATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name (Required)")
	lparName := flag.String("lpar-name", "", "Target LPAR Name (Optional)")
	viosName := flag.String("vios-name", "", "Target VIOS Name (Optional)")
	timeRange := flag.String("range", "1hr", "Time range horizon: 1hr, 1week, 1month, 1year")
	verbose := flag.Bool("verbose", false, "Enable verbose debug logs")
	flag.Parse()

	if *password == "" || *sysName == "" {
		log.Fatal("❌ Error: --hmc-pass and --system-name are mandatory parameter requirements.")
	}

	// Determine operational target scope
	scope := "SYSTEM"
	if *lparName != "" {
		scope = "LPAR"
	} else if *viosName != "" {
		scope = "VIOS"
	}

	// =========================================================================
	// 2. CONNECT AND AUTHENTICATE TO HMC NETWORK
	// =========================================================================
	restClient := hmc.NewHmcRestClient(*hmcIP)
	if err := restClient.Login(context.Background(), *username, *password, *verbose); err != nil {
		log.Fatalf("❌ Logon transaction rejected: %v", err)
	}
	defer restClient.Logoff(context.Background())

	// Resolve the base managed system physical host hardware ID
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatalf("❌ Physical system configuration target '%s' not found.", *sysName)
	}

	var lparUUID string
	if scope == "LPAR" {
		_, resolvedUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, *lparName, *verbose)
		if err != nil || resolvedUUID == "" {
			log.Fatalf("❌ Logical partition target '%s' not found on system '%s'.", *lparName, *sysName)
		}
		lparUUID = resolvedUUID
	}

	// Keep original casing for the endpoint paths
	sysUUIDForPath := sysUUID
	lparUUIDForPath := lparUUID

	// =========================================================================
	// 3. DYNAMIC METRIC INTERVAL AND HORIZON ROUTING MATRIX
	// =========================================================================
	var startTS time.Time
	var endTS time.Time = time.Now()
	var endpointType string
	var expectedFreq string

	// Shift the window back slightly to ensure we only query committed database files
	now := time.Now().Add(-5 * time.Minute)

	switch strings.ToLower(*timeRange) {
	case "1hr":
		startTS = now.Add(-1 * time.Hour)
		endpointType = "ProcessedMetrics"
		expectedFreq = "30"
	case "1week":
		startTS = now.Add(-7 * 24 * time.Hour)
		endpointType = "AggregatedMetrics"
		expectedFreq = "7200" // Shift to 2-hour rollups to bypass the 24-hr purge
	case "1month":
		startTS = now.Add(-30 * 24 * time.Hour)
		endpointType = "AggregatedMetrics"
		expectedFreq = "86400" // Daily rollups
	case "1year":
		startTS = now.Add(-365 * 24 * time.Hour)
		endpointType = "AggregatedMetrics"
		expectedFreq = "86400" // Daily rollups
	default:
		log.Fatalf("❌ Invalid window parameters: '%s'. Choose from 1hr, 1week, 1month, 1year.", *timeRange)
	}

	// =========================================================================
	// 4. VERIFY PCM DAEMON IS ACTIVE
	// =========================================================================
	fmt.Printf("\n⚙️  Verifying Long Term Monitor (LTM) engine status for '%s'...\n", *sysName)
	prefs, err := restClient.GetManagedSystemPcmPreferences(context.Background(), sysUUIDForPath)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to retrieve PCM preferences: %v", err)
	} else {
		needsUpdate := false
		if !prefs.LongTermMonitorEnabled {
			prefs.LongTermMonitorEnabled = true
			needsUpdate = true
		}
		if endpointType == "AggregatedMetrics" && !prefs.AggregationEnabled {
			prefs.AggregationEnabled = true
			needsUpdate = true
		}
		if needsUpdate {
			fmt.Println("   📡 LTM/Aggregation engine is asleep. Waking it up natively...")
			err = restClient.SetManagedSystemPcmPreferences(context.Background(), sysUUIDForPath, prefs)
			if err != nil {
				log.Fatalf("❌ Failed to enable PCM metrics: %v", err)
			}
			fmt.Println("   ✅ PCM Engine enabled. (⚠️ Note: It may take 30+ minutes for the HMC to generate the first batch of Processed JSON files!)")
		} else {
			fmt.Println("   ✅ PCM Engine is active.")
		}
	}

	// =========================================================================
	// 5. RESOLVE THE TARGET FEED ENDPOINT URL
	// =========================================================================
	var targetFeedURL string
	if scope == "LPAR" {
		targetFeedURL = fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/LogicalPartition/%s/%s", *hmcIP, sysUUIDForPath, lparUUIDForPath, endpointType)
	} else {
		targetFeedURL = fmt.Sprintf("https://%s/rest/api/pcm/ManagedSystem/%s/%s", *hmcIP, sysUUIDForPath, endpointType)
	}

	timeFormat := "2006-01-02T15:04:05Z"
	queryString := fmt.Sprintf("StartTS=%s&EndTS=%s", startTS.UTC().Format(timeFormat), endTS.UTC().Format(timeFormat))
	targetFeedURL = targetFeedURL + "?" + queryString

	if *verbose {
		fmt.Printf("\n[DEBUG] Targeting Engine Feed: %s\n", targetFeedURL)
	}

	insecureClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}

	req, _ := http.NewRequest("GET", targetFeedURL, nil)
	req.Header.Set("X-API-Session", restClient.Session())
	req.Header.Set("Accept", "application/atom+xml") // Demanding specific XML feed wrapper

	resp, err := insecureClient.Do(req)
	if err != nil {
		log.Fatalf("❌ Feed transaction dropped: %v", err)
	}
	defer resp.Body.Close()

	feedBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		log.Fatalf("⚠️  HMC returned HTTP %d: No performance metrics recorded inside the selected time envelope. (If you just enabled LTM, wait 30 minutes and try again).", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("❌ HMC API Request failed (HTTP %d). The server returned:\n%s", resp.StatusCode, string(feedBytes))
	}

	var feed LocalAtomFeed
	if err := xml.Unmarshal(feedBytes, &feed); err != nil {
		log.Fatalf("❌ Failed parsing XML framework stream: %v", err)
	}

	var actionableJSONLinks []string
	for _, entry := range feed.Entries {
		for _, link := range entry.Links {
			if strings.Contains(link.Type, "json") || strings.Contains(strings.ToLower(link.Href), ".json") {
				if endpointType == "AggregatedMetrics" && !strings.Contains(link.Href, "_"+expectedFreq+".json") {
					continue
				}
				actionableJSONLinks = append(actionableJSONLinks, link.Href)
			}
		}
	}

	if len(actionableJSONLinks) == 0 {
		fmt.Println("⚠️  No payload links discovered matching validation constraints.")
		return
	}

	fmt.Printf("📊 Target Context: [%s] | Time Horizon: [%s] | Analyzing %d Data Files...\n\n", scope, *timeRange, len(actionableJSONLinks))

	// =========================================================================
	// 6. INGESTION LOOP & TRACKING METRIC REDUCERS
	// =========================================================================
	var (
		sampleCount     int
		firstSampleTime string
		lastSampleTime  string

		// System Data Accumulators
		sysTotalCores, sysSumCores, sysMaxCores float64
		sysTotalMem, sysSumAvailMem             float64
		sysMinAvailMem                          float64 = -1.0

		// LPAR Data Accumulators
		lparEntitled               float64
		lparSumCPU, lparMaxCPU     float64
		lparMaxMem, lparDesiredMem float64

		// VIOS Data Accumulators
		viosSumMem, viosMaxMem       float64
		viosSumNetBuf, viosMaxNetBuf float64
	)

	for _, jsonLink := range actionableJSONLinks {
		jReq, _ := http.NewRequest("GET", jsonLink, nil)
		jReq.Header.Set("X-API-Session", restClient.Session())
		jReq.Header.Set("Accept", "*/*")

		jResp, err := insecureClient.Do(jReq)
		if err != nil || jResp.StatusCode != http.StatusOK {
			continue
		}
		payloadBytes, _ := io.ReadAll(jResp.Body)
		jResp.Body.Close()

		if scope == "SYSTEM" || scope == "VIOS" {
			var payload hmc.SysProcessedMetricsPayload
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				continue
			}

			for _, sample := range payload.SystemUtil.UtilSamples {
				if firstSampleTime == "" {
					firstSampleTime = sample.SampleInfo.TimeStamp
				}
				lastSampleTime = sample.SampleInfo.TimeStamp
				sampleCount++

				if scope == "SYSTEM" {
					if len(sample.ServerUtil.Processor.TotalProcUnits) > 0 {
						sysTotalCores = sample.ServerUtil.Processor.TotalProcUnits[0]
					}
					if len(sample.ServerUtil.Processor.UtilizedProcUnits) > 0 {
						used := sample.ServerUtil.Processor.UtilizedProcUnits[0]
						sysSumCores += used
						if used > sysMaxCores {
							sysMaxCores = used
						}
					}
					if len(sample.ServerUtil.Memory.TotalMem) > 0 {
						sysTotalMem = sample.ServerUtil.Memory.TotalMem[0] / 1024.0
					}
					if len(sample.ServerUtil.Memory.AvailableMem) > 0 {
						avail := sample.ServerUtil.Memory.AvailableMem[0] / 1024.0
						sysSumAvailMem += avail
						if sysMinAvailMem < 0 || avail < sysMinAvailMem {
							sysMinAvailMem = avail
						}
					}
				} else if scope == "VIOS" {
					for _, vios := range sample.ViosUtil {
						if strings.EqualFold(vios.Name, *viosName) {
							if len(vios.Memory.UtilizedMem) > 0 {
								vMem := vios.Memory.UtilizedMem[0]
								viosSumMem += vMem
								if vMem > viosMaxMem {
									viosMaxMem = vMem
								}
							}
							if len(vios.Memory.VirtualPersistentMem) > 0 {
								vBuf := vios.Memory.VirtualPersistentMem[0]
								viosSumNetBuf += vBuf
								if vBuf > viosMaxNetBuf {
									viosMaxNetBuf = vBuf
								}
							}
						}
					}
				}
			}
		} else if scope == "LPAR" {
			// DYNAMIC JSON PARSING - Immune to struct mapping failures!
			var payload map[string]interface{}
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				continue
			}

			sysUtil, ok := payload["systemUtil"].(map[string]interface{})
			if !ok {
				continue
			}

			utilSamples, ok := sysUtil["utilSamples"].([]interface{})
			if !ok {
				continue
			}

			for _, s := range utilSamples {
				sample, ok := s.(map[string]interface{})
				if !ok {
					continue
				}

				// Extract Timestamp
				if sampleInfo, ok := sample["sampleInfo"].(map[string]interface{}); ok {
					if ts, ok := sampleInfo["timeStamp"].(string); ok {
						if firstSampleTime == "" {
							firstSampleTime = ts
						}
						lastSampleTime = ts
					}
				}
				sampleCount++

				// Navigate down to lparsUtil
				lparsUtil, ok := sample["lparsUtil"].([]interface{})
				if !ok {
					continue
				}

				for _, lu := range lparsUtil {
					lparMap, ok := lu.(map[string]interface{})
					if !ok {
						continue
					}

					// Process CPU
					if proc, ok := lparMap["processor"].(map[string]interface{}); ok {
						if ent, ok := proc["entitledProcUnits"].([]interface{}); ok && len(ent) > 0 {
							if val, ok := ent[0].(float64); ok {
								lparEntitled = val
							}
						}
						// Note: util[0] seamlessly grabs the "Average" metric from the [Avg, Min, Max] array!
						if util, ok := proc["utilizedProcUnits"].([]interface{}); ok && len(util) > 0 {
							if val, ok := util[0].(float64); ok {
								lparSumCPU += val
								if val > lparMaxCPU {
									lparMaxCPU = val
								}
							}
						}
					}

					// Process Memory
					if mem, ok := lparMap["memory"].(map[string]interface{}); ok {
						if logMem, ok := mem["logicalMem"].([]interface{}); ok && len(logMem) > 0 {
							if val, ok := logMem[0].(float64); ok {
								lparDesiredMem = val / 1024.0
							}
						}
						if backed, ok := mem["backedPhysicalMem"].([]interface{}); ok && len(backed) > 0 {
							if val, ok := backed[0].(float64); ok {
								backedGB := val / 1024.0
								if backedGB > lparMaxMem {
									lparMaxMem = backedGB
								}
							}
						}
					}
				}
			}
		}
	}

	// =========================================================================
	// 7. OUTPUT CONTEXTUAL ARCHITECTURAL GRAPH SUMMARIES
	// =========================================================================
	if sampleCount == 0 {
		fmt.Println("⚠️ Ingestion complete. No valid numeric datasets located inside data snapshots.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\n=========================================================================")
	fmt.Fprintf(w, " 📊 POWERVM METRIC GRAPH SUMMARY - SCOPE [%s]\n", scope)
	fmt.Fprintln(w, "=========================================================================")
	fmt.Fprintf(w, "   Evaluation Window                  :\t%s to %s\n", firstSampleTime, lastSampleTime)
	fmt.Fprintf(w, "   Aggregated Sampling Intervals      :\t%d iterations (Freq: %ss)\n", sampleCount, expectedFreq)
	fmt.Fprintln(w, "   -------------------------------------------------------------------------")

	switch scope {
	case "SYSTEM":
		avgCPU := sysSumCores / float64(sampleCount)
		avgMemUsed := sysTotalMem - (sysSumAvailMem / float64(sampleCount))
		maxMemUsed := sysTotalMem - sysMinAvailMem

		fmt.Fprintf(w, "   Physical CPU Core Capacity         :\t%.2f Cores\n", sysTotalCores)
		fmt.Fprintf(w, "   Average Global CPU Allocation      :\t%.2f Cores (%.1f%% efficiency)\n", avgCPU, (avgCPU/sysTotalCores)*100)
		fmt.Fprintf(w, "   Absolute Peak Server Core Draw     :\t%.2f Cores (%.1f%% pool stress)\n", sysMaxCores, (sysMaxCores/sysTotalCores)*100)
		fmt.Fprintln(w, "   -------------------------------------------------------------------------")
		fmt.Fprintf(w, "   Installed Hardware Memory          :\t%.1f GB\n", sysTotalMem)
		fmt.Fprintf(w, "   Average Host RAM Consumption       :\t%.1f GB (%.1f%% allocation)\n", avgMemUsed, (avgMemUsed/sysTotalMem)*100)
		fmt.Fprintf(w, "   Absolute Hardware Peak RAM Draw    :\t%.1f GB (%.1f%% cap boundaries)\n", maxMemUsed, (maxMemUsed/sysTotalMem)*100)

	case "LPAR":
		avgLparCPU := lparSumCPU / float64(sampleCount)
		fmt.Fprintf(w, "   Partition Core Entitlement         :\t%.2f Cores\n", lparEntitled)
		fmt.Fprintf(w, "   Average Runtime Compute Footprint  :\t%.2f Cores (%.1f%% entitlement match)\n", avgLparCPU, (avgLparCPU/lparEntitled)*100)
		fmt.Fprintf(w, "   Absolute Workload Core Spike Peak  :\t%.2f Cores (%.1f%% sizing ratio)\n", lparMaxCPU, (lparMaxCPU/lparEntitled)*100)
		fmt.Fprintln(w, "   -------------------------------------------------------------------------")
		fmt.Fprintf(w, "   Profile Assigned Memory Bounds     :\t%.1f GB Desired\n", lparDesiredMem)
		fmt.Fprintf(w, "   Peak Physical RAM Allocation       :\t%.1f GB Backed Physical Pool Alloc\n", lparMaxMem)

	case "VIOS":
		avgVIOSMem := viosSumMem / float64(sampleCount)
		fmt.Fprintf(w, "   Target Virtual I/O Domain Name     :\t%s\n", *viosName)
		fmt.Fprintf(w, "   Average Shared Core Workspace RAM  :\t%.2f MB Used\n", avgVIOSMem)
		fmt.Fprintf(w, "   Peak Workspace Memory Allocation   :\t%.2f MB\n", viosMaxMem)
	}

	fmt.Fprintln(w, "=========================================================================")
	w.Flush()
}
