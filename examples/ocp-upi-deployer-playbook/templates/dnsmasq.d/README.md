# dnsmasq Configuration Structure for Multiple OpenShift Clusters

This directory contains modular dnsmasq configuration files organized by cluster.

## Directory Structure

Since dnsmasq's `conf-dir` directive only scans one level deep and doesn't recurse into subdirectories, we use a flat structure with cluster-prefixed filenames:

```
/etc/dnsmasq.d/
├── global.conf                    # Global settings (shared)
├── <cluster1>-dhcp.conf          # DHCP for cluster1
├── <cluster1>-dns.conf           # DNS for cluster1
├── <cluster1>-tftp.conf          # TFTP for cluster1
├── <cluster2>-dhcp.conf          # DHCP for cluster2
├── <cluster2>-dns.conf           # DNS for cluster2
├── <cluster2>-tftp.conf          # TFTP for cluster2
└── README.md                      # This file
```

## Naming Convention

Each cluster's configuration files follow this pattern:
- `<cluster-name>-dhcp.conf` - DHCP configuration
- `<cluster-name>-dns.conf` - DNS records
- `<cluster-name>-tftp.conf` - TFTP/PXE boot settings

## Example: Managing Two Clusters

```
/etc/dnsmasq.d/
├── global.conf                    # Shared settings
├── ocp4-prod-dhcp.conf           # Production DHCP
├── ocp4-prod-dns.conf            # Production DNS
├── ocp4-prod-tftp.conf           # Production TFTP
├── ocp4-dev-dhcp.conf            # Development DHCP
├── ocp4-dev-dns.conf             # Development DNS
└── ocp4-dev-tftp.conf            # Development TFTP
```

## How It Works

1. **Main dnsmasq.conf** includes: `conf-dir=/etc/dnsmasq.d,*.conf`
2. **All .conf files** in `/etc/dnsmasq.d/` are loaded (flat structure)
3. **Cluster isolation** via filename prefixes and DHCP tags
4. **Easy management**: Add/remove clusters by adding/removing their config files

## Benefits

- ✅ **Multi-cluster support**: Manage multiple OpenShift clusters from one helper node
- ✅ **Flat structure**: Works with dnsmasq's single-level conf-dir scanning
- ✅ **Easy identification**: Cluster name prefix makes files easy to find
- ✅ **Easy updates**: Modify one cluster without affecting others
- ✅ **Easy cleanup**: Remove a cluster by deleting its 3 config files
- ✅ **No conflicts**: Separate DHCP ranges, DNS zones, DHCP tags

## Deployment Process

The ocp-upi-deployer will:
1. Check if `/etc/dnsmasq.d/` exists
2. Generate cluster-specific config files with cluster name prefix
3. Deploy files to `/etc/dnsmasq.d/<cluster-name>-{dhcp,dns,tftp}.conf`
4. Reload dnsmasq to apply changes
5. Preserve existing cluster configurations

## Removing a Cluster

To remove a cluster's configuration:

```bash
CLUSTER_NAME="ocp4-prod"
rm -f /etc/dnsmasq.d/${CLUSTER_NAME}-*.conf
systemctl reload dnsmasq
```

## Listing Configured Clusters

```bash
ls -1 /etc/dnsmasq.d/*-dhcp.conf | sed 's|.*/||; s|-dhcp.conf||'