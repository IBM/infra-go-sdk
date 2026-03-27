# URL and File Path Validation in ocp-upi-deployer

## Overview
The ocp-upi-deployer includes comprehensive validation for URLs and file paths to ensure all required resources are properly formatted and accessible before deployment begins. This prevents deployment failures due to invalid URLs or missing files.

## Validated URLs

### 1. RHCOS Image URLs
These URLs are required for PXE boot setup by the Ansible playbook:

- **Kernel URL** (`rhcos_images.kernel_url`)
  - Example: `https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-4.21.0-ppc64le-live-kernel-ppc64le`
  - Used for: Network boot kernel image

- **Initramfs URL** (`rhcos_images.initramfs_url`)
  - Example: `https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-4.21.0-ppc64le-live-initramfs.ppc64le.img`
  - Used for: Initial RAM filesystem for boot

- **Rootfs URL** (`rhcos_images.rootfs_url`)
  - Example: `https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-4.21.0-ppc64le-live-rootfs.ppc64le.img`
  - Used for: Root filesystem image

### 2. OpenShift Client/Installer URLs
These URLs are required for downloading OpenShift tools:

- **Base URL** (`ocp_client_config.ocp_base_url`)
  - Example: `https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients`
  - Used for: Base path for OCP clients

- **Client Tarball** (`ocp_client_config.ocp_client`)
  - Example: `https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/latest-4.21/openshift-client-linux.tar.gz`
  - Used for: `oc` and `kubectl` CLI tools

- **Installer Tarball** (`ocp_client_config.ocp_installer`)
  - Example: `https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/latest-4.21/openshift-install-linux.tar.gz`
  - Used for: `openshift-install` tool

## File Path Validation

### Validated File Paths
These file paths are required and validated before deployment:

- **Pull Secret File** (`openshift.pull_secret_file`)
  - Example: `~/pull-secret.txt` or `/path/to/pull-secret.json`
  - Used for: Authenticating with Red Hat registries to pull container images

- **SSH Public Key File** (`openshift.ssh_public_key_file`)
  - Example: `~/.ssh/id_rsa.pub`
  - Used for: SSH access to cluster nodes

- **Helper Node SSH Key File** (`helper_node.ssh_key_file`)
  - Example: `~/.ssh/id_rsa`
  - Used for: SSH access to helper node for Ansible playbook execution

### File Path Validation Checks
1. **Path Expansion**: Supports `~` expansion to home directory
2. **File Existence**: Verifies file exists at specified path
3. **File Type**: Ensures path points to a file (not a directory)
4. **Readability**: Checks if file can be opened and read
5. **Error Handling**: Provides clear error messages for issues

## Validation Levels

### Level 1: Format Validation (Always Performed)
Validates URL format and structure:

```go
// Checks performed:
- URL scheme is http or https
- URL has a valid host
- URL can be parsed successfully
```

**When it runs**: Automatically during configuration validation

**Example errors**:
```
❌ invalid RHCOS kernel URL: htp://invalid-url
❌ invalid OCP client URL: not-a-url
❌ RHCOS initramfs URL is required
```

### Level 2: Accessibility Validation (Optional)
Checks if URLs are actually accessible:

```go
// Checks performed:
- HTTP HEAD request to each URL
- Validates HTTP status code (200, 301, 302)
- 10-second timeout per URL
```

**When it runs**: Optional, can be enabled for thorough validation

**Example warnings**:
```
⚠️  RHCOS Kernel URL may not be accessible: https://example.com/kernel (error: connection timeout)
⚠️  OCP Client URL returned status 404: https://example.com/client.tar.gz
```

## Configuration Example

### Valid Configuration
```yaml
openshift:
  version: "4.21"
  
  # RHCOS Images for PXE boot
  rhcos_images:
    kernel_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-4.21.0-ppc64le-live-kernel-ppc64le"
    initramfs_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-4.21.0-ppc64le-live-initramfs.ppc64le.img"
    rootfs_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/dependencies/rhcos/4.21/4.21.0/rhcos-4.21.0-ppc64le-live-rootfs.ppc64le.img"
  
  # OpenShift Client/Installer Configuration
  ocp_client_config:
    ocp_client_arch: "ppc64le"
    ocp_base_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients"
    ocp_client_base: "ocp"
    ocp_client_tag: "latest-4.21"
    ocp_client: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/latest-4.21/openshift-client-linux.tar.gz"
    ocp_installer: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/latest-4.21/openshift-install-linux.tar.gz"
```

### Invalid Configuration Examples

#### Missing URLs
```yaml
openshift:
  rhcos_images:
    kernel_url: ""  # ❌ Error: RHCOS kernel URL is required
    initramfs_url: "https://example.com/initramfs.img"
    rootfs_url: "https://example.com/rootfs.img"
```

#### Invalid URL Format
```yaml
openshift:
  rhcos_images:
    kernel_url: "not-a-url"  # ❌ Error: invalid RHCOS kernel URL
    initramfs_url: "ftp://example.com/file"  # ❌ Error: invalid scheme (must be http/https)
    rootfs_url: "https://"  # ❌ Error: missing host
```

## Implementation Details

### Validator Methods

#### `validateRHCOSURLs()`
```go
func (v *Validator) validateRHCOSURLs() {
    // Validates all RHCOS image URLs
    // - Checks if URLs are provided
    // - Validates URL format
    // - Adds errors for invalid URLs
}
```

#### `validateOCPClientConfig()`
```go
func (v *Validator) validateOCPClientConfig() {
    // Validates OCP client/installer configuration
    // - Checks required fields (arch, base_url, etc.)
    // - Validates URL formats
    // - Adds errors for invalid URLs
}
```

#### `isValidURL()`
```go
func (v *Validator) isValidURL(urlStr string) bool {
    // Validates URL format
    // - Parses URL
    // - Checks scheme (http/https)
    // - Checks host presence
    // Returns: true if valid, false otherwise
}
```

#### `ValidateURLAccessibility()` (Optional)
```go
func (v *Validator) ValidateURLAccessibility() {
    // Performs HTTP HEAD requests to check accessibility
    // - 10-second timeout per URL
    // - Follows redirects
    // - Adds warnings for inaccessible URLs
}
```

## Usage

### Basic Validation (Format Only)
```go
validator := NewValidator(config)
if err := validator.Validate(); err != nil {
    log.Fatalf("Configuration validation failed: %v", err)
}
```

### Thorough Validation (Format + Accessibility)
```go
validator := NewValidator(config)
if err := validator.Validate(); err != nil {
    log.Fatalf("Configuration validation failed: %v", err)
}

// Optional: Check URL accessibility
validator.ValidateURLAccessibility()
```

## Error Handling

### Validation Errors (Blocking)
These errors prevent deployment from starting:
- Missing required URLs
- Invalid URL format
- Invalid URL scheme (not http/https)
- Missing URL host

### Validation Warnings (Non-blocking)
These warnings are informational but don't block deployment:
- URL accessibility issues (timeout, 404, etc.)
- Slow response times

## Best Practices

### 1. Use Official Mirrors
```yaml
# ✅ Recommended: Use official OpenShift mirrors
ocp_base_url: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients"

# ⚠️  Not recommended: Use custom mirrors only if necessary
ocp_base_url: "https://my-custom-mirror.com/ocp"
```

### 2. Verify URLs Before Deployment
```bash
# Test URLs manually before deployment
curl -I https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/latest-4.21/openshift-client-linux.tar.gz
```

### 3. Keep URLs Updated
```yaml
# Update URLs when upgrading OpenShift versions
version: "4.21"  # Update this
rhcos_images:
  kernel_url: "...4.21/4.21.0/..."  # And these
ocp_client_config:
  ocp_client_tag: "latest-4.21"  # And this
```

### 4. Use Version-Specific URLs
```yaml
# ✅ Good: Version-specific URLs
ocp_client: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/4.21.0/openshift-client-linux.tar.gz"

# ⚠️  Risky: Using 'latest' may cause version mismatches
ocp_client: "https://mirror.openshift.com/pub/openshift-v4/ppc64le/clients/ocp/latest/openshift-client-linux.tar.gz"
```

## Troubleshooting

### Problem: "invalid RHCOS kernel URL"
**Solution**: Check URL format, ensure it starts with http:// or https://

### Problem: "RHCOS Kernel URL may not be accessible"
**Possible causes**:
1. Network connectivity issues
2. URL is incorrect or file moved
3. Mirror is temporarily down
4. Firewall blocking access

**Solution**: 
```bash
# Test URL manually
curl -I <url>

# Check network connectivity
ping mirror.openshift.com

# Try alternative mirror
```

### Problem: "OCP client URL returned status 404"
**Solution**: 
1. Verify the OpenShift version exists
2. Check if the file path is correct
3. Browse the mirror directory to find the correct URL

## Integration with Deployment

The URL validation is integrated into the deployment workflow:

```
1. Load Configuration
   └── Parse config-sno.yaml

2. Validate Configuration ← URL VALIDATION HAPPENS HERE
   ├── Validate HMC, systems, network
   ├── Validate RHCOS URLs (format)
   ├── Validate OCP client URLs (format)
   └── Optional: Check URL accessibility

3. Start Deployment
   └── URLs are guaranteed to be valid
```

## Summary

URL and file path validation provides:
- ✅ Early error detection (before deployment starts)
- ✅ Clear error messages for invalid URLs and missing files
- ✅ Optional accessibility checks for URLs
- ✅ File existence and readability verification
- ✅ Support for `~` home directory expansion
- ✅ Prevention of deployment failures due to bad URLs or missing files
- ✅ Better user experience with immediate feedback

All URLs and file paths are validated automatically during configuration validation, ensuring a smooth deployment process.

## File Path Validation Examples

### Valid Configuration
```yaml
openshift:
  pull_secret_file: "~/pull-secret.txt"
  ssh_public_key_file: "~/.ssh/id_rsa.pub"

helper_node:
  ssh_key_file: "~/.ssh/id_rsa"
```

### Common File Path Errors

#### File Not Found
```
❌ pull secret file not found: ~/pull-secret.txt
```
**Solution**: Verify the file exists at the specified path

#### Path is a Directory
```
❌ SSH public key path is a directory, not a file: ~/.ssh
```
**Solution**: Specify the full path to the file, not just the directory

#### File Not Readable
```
❌ pull secret file is not readable: ~/pull-secret.txt (error: permission denied)
```
**Solution**: Check file permissions with `ls -l` and adjust with `chmod`

#### Home Directory Expansion Failed
```
❌ pull secret: unable to expand home directory in path '~/pull-secret.txt': error
```
**Solution**: Use absolute path instead of `~`