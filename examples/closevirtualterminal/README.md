# Close Virtual Terminal Example

## Overview

This example demonstrates how to close a virtual terminal session for an LPAR (Logical Partition) on IBM Power Systems managed by HMC (Hardware Management Console).

## Important Note: REST API Limitation

**Close terminal via REST API is not working. This is as per the HMC design!**

The HMC REST API does not provide a direct method to close virtual terminal sessions. This is a known limitation in the HMC REST API design, not a bug in the SDK.

## Workaround: SSH-Based Method

This example implements an automatic fallback mechanism that uses SSH to execute HMC CLI commands when the REST API method fails.

### How It Works

1. **Primary Method (REST API):** Attempts to close the terminal using REST API
2. **Fallback Method (SSH):** If REST API fails, automatically falls back to SSH-based CLI execution
3. **Result:** Terminal is successfully closed using the SSH method

### SSH Method Details

The SSH fallback executes the following HMC CLI command:
```bash
rmvterm -m <system-name> -p <lpar-name>
```

This command removes (closes) the virtual terminal session for the specified LPAR.

## Usage

### Basic Usage (with automatic fallback)

```bash
go run main.go \
  -hmc 192.0.2.2 \
  -user REDACTED_HMC_USER<== \
  -pass <password> \
  -system LTC09u23-p11 \
  -lpar sno-new-4 \
  -verbose
```

### REST API Only (no fallback)

```bash
go run main.go \
  -hmc 192.0.2.2 \
  -user REDACTED_HMC_USER<== \
  -pass <password> \
  -system LTC09u23-p11 \
  -lpar sno-new-4 \
  -fallback=false
```

## Command-Line Flags

| Flag | Required | Description |
|------|----------|-------------|
| `-hmc` | Yes | HMC IP address or hostname |
| `-user` | Yes | HMC username (typically `REDACTED_HMC_USER<==`) |
| `-pass` | Yes | HMC password |
| `-system` | Yes | Managed system name |
| `-lpar` | Yes | LPAR name |
| `-verbose` | No | Enable verbose output (default: false) |
| `-fallback` | No | Use SSH fallback if REST API fails (default: true) |

## Example Output

### Successful Execution (with SSH fallback)

```
Connecting to HMC at 192.0.2.2...
✓ Successfully logged in to HMC

Closing virtual terminal for LPAR 'sno-new-4' on system 'LTC09u23-p11'...
Attempting REST API method...
⚠ REST API failed: Unsupported command
Falling back to SSH method...
✓ Virtual terminal closed successfully via SSH
```

### REST API Only (expected to fail)

```
Connecting to HMC at 192.0.2.2...
✓ Successfully logged in to HMC

Closing virtual terminal for LPAR 'sno-new-4' on system 'LTC09u23-p11'...
Attempting REST API method...
Failed to close virtual terminal: Unsupported command
```

## When to Use This

Use this example when you need to:

1. **Close hanging terminal sessions** - When a virtual terminal session is stuck or unresponsive
2. **Clean up after operations** - After completing work on an LPAR console
3. **Automate terminal management** - In scripts that need to ensure terminals are closed
4. **Troubleshoot connection issues** - When you can't connect because a terminal is already open

## Common Use Cases

### 1. Before Opening a New Terminal

Close any existing terminal before opening a new one:
```bash
# Close existing terminal
go run main.go -hmc <ip> -user <user> -pass <pass> -system <sys> -lpar <lpar>

# Now open a new terminal
# (use openvirtualterminal example)
```

### 2. Cleanup After Automation

In deployment scripts:
```bash
# Do work on LPAR console
# ...

# Clean up terminal session
go run main.go -hmc <ip> -user <user> -pass <pass> -system <sys> -lpar <lpar>
```

### 3. Troubleshooting

When you get "terminal already open" errors:
```bash
# Close the stuck terminal
go run main.go -hmc <ip> -user <user> -pass <pass> -system <sys> -lpar <lpar> -verbose
```

## Technical Details

### REST API Limitation

The HMC REST API specification does not include an endpoint for closing virtual terminals. The API provides:
- ✅ `OpenVirtualTerminal` - Opens a terminal session
- ❌ `CloseVirtualTerminal` - **Not available in REST API**

### SSH Method Implementation

The SSH fallback method:
1. Establishes SSH connection to HMC
2. Executes `rmvterm -m <system> -p <lpar>` command
3. Parses command output for success/failure
4. Closes SSH connection

### Error Handling

The example handles several error scenarios:
- **Authentication failures** - Invalid credentials
- **Connection errors** - Network issues
- **Invalid system/LPAR names** - Non-existent resources
- **SSH failures** - When fallback also fails

## Prerequisites

- HMC with SSH access enabled
- Valid HMC credentials with appropriate permissions
- Network connectivity to HMC
- Go 1.16 or later

## Related Examples

- `openvirtualterminal` - Opens a virtual terminal session
- `getvirtualterminal` - Gets virtual terminal information
- `poweronpartition` - Powers on an LPAR
- `poweroffpartition` - Powers off an LPAR

## Notes

- The SSH method requires SSH access to be enabled on the HMC
- The user must have permissions to execute `rmvterm` command
- Closing a terminal does not affect the LPAR's running state
- Multiple terminal sessions can exist for the same LPAR (though not recommended)

## Troubleshooting

### "Authentication failed"
- Verify HMC credentials
- Check if account is locked
- Ensure user has appropriate permissions

### "System not found"
- Verify managed system name is correct
- Check if system is managed by this HMC
- Use `getmanagedsystems` example to list available systems

### "LPAR not found"
- Verify LPAR name is correct
- Check if LPAR exists on the specified system
- Use `getlogicalpartitions` example to list LPARs

### "SSH connection failed"
- Verify SSH is enabled on HMC
- Check firewall rules
- Ensure SSH port (22) is accessible

## License

This example is part of the powerhmc-go SDK.
