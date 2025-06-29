package hmc

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Hmc represents the HMC resource manager
type Hmc struct {
	sshClient *ssh.Client
	cmdStack  *HmcCommandStack
}

// NewHmc creates a new Hmc instance with an SSH client
func NewHmc(sshClient *ssh.Client) *Hmc {
	return &Hmc{
		sshClient: sshClient,
		cmdStack:  NewHmcCommandStack(),
	}
}

// execute runs a command via SSH and returns the output
func (h *Hmc) execute(cmd string, verbose bool) (string, error) {
	if verbose {
		hmcLogger.Printf("Executing SSH command: %s", cmd)
	}
	session, err := h.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to run command '%s': %v", cmd, err)
	}
	if verbose {
		hmcLogger.Printf("Command output: %s", string(output))
	}
	return string(output), nil
}

// ListHMCVersion lists the HMC version details
func (h *Hmc) ListHMCVersion(verbose bool) (map[string]string, error) {
	// Check if HMC_CMD["LSHMC"] exists
	if _, ok := h.cmdStack.HMC_CMD["LSHMC"]; !ok {
		return nil, fmt.Errorf("command LSHMC not found in HMC_CMD")
	}
	// Check if HMC_CMD_OPT["LSHMC"] exists
	if _, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdLshmc]; !ok {
		return nil, fmt.Errorf("options for LSHMC not found in HMC_CMD_OPT")
	}
	// Check if HMC_CMD_OPT["LSHMC"]["-V"] exists and is a string
	vOpt, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdLshmc]["-V"]
	if !ok {
		return nil, fmt.Errorf("option -V not found for LSHMC in HMC_CMD_OPT")
	}
	vStr, ok := vOpt.(string)
	if !ok {
		return nil, fmt.Errorf("expected string for -V option, got %T", vOpt)
	}

	cmd := h.cmdStack.HMC_CMD["LSHMC"] + vStr
	result, err := h.execute(cmd, verbose)
	if err != nil {
		return nil, err
	}

	versionDict := make(map[string]string)
	fixPacks := []string{}
	for _, line := range strings.Split(result, "\n") {
		switch {
		case strings.Contains(line, "Version:"):
			if len(strings.Split(line, ":")) > 1 {
				versionDict["VERSION"] = strings.TrimSpace(strings.Split(line, ":")[1])
			}
		case strings.Contains(line, "Release:"):
			if len(strings.Split(line, ":")) > 1 {
				versionDict["RELEASE"] = strings.TrimSpace(strings.Split(line, ":")[1])
			}
		case strings.Contains(line, "Service Pack:"):
			if len(strings.Split(line, ":")) > 1 {
				versionDict["SERVICEPACK"] = strings.TrimSpace(strings.Split(line, ":")[1])
			}
		case strings.Contains(line, "HMC Build level"):
			if len(strings.Split(line, "l ")) > 1 {
				versionDict["HMCBUILDLEVEL"] = strings.TrimSpace(strings.Split(line, "l ")[1])
			}
		case strings.Contains(line, "-"):
			fixPacks = append(fixPacks, line)
		case strings.Contains(line, "base_version"):
			if len(strings.Split(line, "=")) > 1 {
				versionDict["BASEVERSION"] = strings.TrimSpace(strings.Split(line, "=")[1])
			}
		}
	}
	if len(fixPacks) > 0 {
		versionDict["FIXPACKS"] = strings.Join(fixPacks, ",")
	}
	return versionDict, nil
}

// PingTest pings a host and returns the result
func (h *Hmc) PingTest(host string, verbose bool) (string, error) {
	if verbose {
		hmcLogger.Printf("Pinging host: %s", host)
	}
	cmd := exec.Command("ping", "-c", "2", host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if verbose {
			hmcLogger.Printf("Ping failed: %v", err)
		}
		return "No response", nil
	}
	if verbose {
		hmcLogger.Printf("Ping output: %s", string(output))
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "packets transmitted") {
			if strings.Contains(line, "0 received") {
				return "No response", nil
			} else if strings.Contains(line, "1 received") {
				return "Partial Response", nil
			} else if strings.Contains(line, "2 received") {
				return "Alive", nil
			}
		}
	}
	return "No response", nil
}

// CheckHmcUpandRunning checks if HMC is up and running
func (h *Hmc) CheckHmcUpandRunning(rebootStarted bool, timeoutInMin int, verbose bool) (bool, error) {
	pollInterval := 30 * time.Second
	waitUntil := time.Duration(timeoutInMin) * time.Minute

	var wg sync.WaitGroup
	wg.Add(1)
	pingSuccess := false

	go func() {
		defer wg.Done()
		start := time.Now()
		for time.Since(start) < waitUntil {
			pingState, _ := h.PingTest(h.sshClient.RemoteAddr().String(), verbose)
			if pingState == "Alive" && rebootStarted {
				pingSuccess = true
				return
			}
			if pingState == "No response" {
				rebootStarted = true
			}
			time.Sleep(pollInterval)
		}
	}()

	wg.Wait()
	return pingSuccess, nil
}

// CheckIfHMCFullyBootedUp checks if HMC is fully booted up
func CheckIfHMCFullyBootedUp(hmcIP, user, password string, verbose bool) (bool, map[string]string, error) {
	pollInterval := 30 * time.Second
	waitUntil := 20 * time.Minute

	if verbose {
		hmcLogger.Printf("Checking if HMC %s is fully booted up", hmcIP)
	}
	time.Sleep(3 * time.Minute) // Initial wait
	start := time.Now()
	for time.Since(start) < waitUntil {
		sshClient, err := ssh.Dial("tcp", hmcIP+":22", &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if err == nil {
			defer sshClient.Close()
			hmcObj := NewHmc(sshClient)
			versionDict, err := hmcObj.ListHMCVersion(verbose)
			if err == nil && versionDict["RELEASE"] != "" {
				return true, versionDict, nil
			}
		}
		if verbose {
			hmcLogger.Printf("HMC not fully booted, retrying after %v", pollInterval)
		}
		time.Sleep(pollInterval)
	}
	return false, nil, nil
}

// HmcShutdown shuts down the HMC
func (h *Hmc) HmcShutdown(numOfMin string, reboot bool, verbose bool) error {
	if _, ok := h.cmdStack.HMC_CMD["HMCSHUTDOWN"]; !ok {
		return fmt.Errorf("command HMCSHUTDOWN not found in HMC_CMD")
	}
	if _, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdHmcshutdown]; !ok {
		return fmt.Errorf("options for HMCSHUTDOWN not found in HMC_CMD_OPT")
	}
	tOpt, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdHmcshutdown]["-T"]
	if !ok {
		return fmt.Errorf("option -T not found for HMCSHUTDOWN in HMC_CMD_OPT")
	}
	tStr, ok := tOpt.(string)
	if !ok {
		return fmt.Errorf("expected string for -T option, got %T", tOpt)
	}

	cmd := h.cmdStack.HMC_CMD["HMCSHUTDOWN"] + tStr + numOfMin
	if reboot {
		rOpt, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdHmcshutdown]["-R"]
		if !ok {
			return fmt.Errorf("option -R not found for HMCSHUTDOWN in HMC_CMD_OPT")
		}
		rStr, ok := rOpt.(string)
		if !ok {
			return fmt.Errorf("expected string for -R option, got %T", rOpt)
		}
		cmd += rStr
	}
	_, err := h.execute(cmd, verbose)
	if err != nil {
		return err
	}
	if numOfMin != "now" {
		minutes, err := time.ParseDuration(numOfMin + "m")
		if err != nil {
			return fmt.Errorf("invalid numOfMin value: %v", err)
		}
		time.Sleep(minutes)
	}
	return nil
}

// GetNextPartitionID retrieves the next available partition ID for a given CEC
func (h *Hmc) GetNextPartitionID(cecName string, maxSuppLpars int, verbose bool) (int, error) {
	// Validate inputs
	if cecName == "" {
		return 0, fmt.Errorf("cecName cannot be empty")
	}
	if maxSuppLpars <= 0 {
		return 0, fmt.Errorf("maxSuppLpars must be positive")
	}

	// Construct the lssyscfg command
	if _, ok := h.cmdStack.HMC_CMD["LSSYSCFG"]; !ok {
		return 0, fmt.Errorf("command LSSYSCFG not found in HMC_CMD")
	}
	if _, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdLssyscfg]; !ok {
		return 0, fmt.Errorf("options for LSSYSCFG not found in HMC_CMD_OPT")
	}
	rOpt, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdLssyscfg]["-R"]
	if !ok {
		return 0, fmt.Errorf("option -R not found for LSSYSCFG in HMC_CMD_OPT")
	}
	rLparMap, ok := rOpt.(map[string]string)
	if !ok {
		return 0, fmt.Errorf("expected map[string]string for -R option, got %T", rOpt)
	}
	rLpar, ok := rLparMap["LPAR"]
	if !ok {
		return 0, fmt.Errorf("option -R LPAR not found for LSSYSCFG")
	}
	mOpt, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdLssyscfg]["-M"]
	if !ok {
		return 0, fmt.Errorf("option -M not found for LSSYSCFG in HMC_CMD_OPT")
	}
	mStr, ok := mOpt.(string)
	if !ok {
		return 0, fmt.Errorf("expected string for -M option, got %T", mOpt)
	}
	fOpt, ok := h.cmdStack.HMC_CMD_OPT[HmcCmdLssyscfg]["-F"]
	if !ok {
		return 0, fmt.Errorf("option -F not found for LSSYSCFG in HMC_CMD_OPT")
	}
	fStr, ok := fOpt.(string)
	if !ok {
		return 0, fmt.Errorf("expected string for -F option, got %T", fOpt)
	}

	cmd := h.cmdStack.HMC_CMD["LSSYSCFG"] + rLpar + mStr + cecName + fStr + "lpar_id"
	result, err := h.execute(cmd, verbose)
	if err != nil {
		return 0, fmt.Errorf("failed to execute lssyscfg command: %v", err)
	}

	result = strings.TrimSpace(result)
	// Check if no partitions exist
	if result == "No results were found." {
		return 1, nil
	}

	// Parse existing partition IDs into a slice of integers
	existingLparList := []int{}
	for _, idStr := range strings.Split(result, "\n") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return 0, fmt.Errorf("failed to parse partition ID '%s': %v", idStr, err)
		}
		existingLparList = append(existingLparList, id)
	}

	// Generate the list of supported IDs (1 to maxSuppLpars)
	suppIDList := make([]int, maxSuppLpars)
	for i := 0; i < maxSuppLpars; i++ {
		suppIDList[i] = i + 1
	}

	// Find available IDs (IDs in suppIDList but not in existingLparList)
	availList := []int{}
	for _, id := range suppIDList {
		if !contains(existingLparList, id) {
			availList = append(availList, id)
		}
	}

	// If no available IDs, return an error
	if len(availList) == 0 {
		return 0, fmt.Errorf("no available partition IDs for CEC %s", cecName)
	}

	// Sort available IDs and return the smallest one
	sort.Ints(availList)
	return availList[0], nil
}

// contains checks if a slice contains a specific integer
func contains(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
