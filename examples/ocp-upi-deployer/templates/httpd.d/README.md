# Apache/httpd Configuration Structure for Multiple OpenShift Clusters

This directory contains modular Apache configuration files organized by cluster using a flat structure.

## Directory Structure

```
/etc/httpd/conf.d/
├── <cluster1>-vhost.conf        # Virtual host for cluster1
├── <cluster2>-vhost.conf        # Virtual host for cluster2
└── README.md                     # This file
```

## Naming Convention

Each cluster's configuration file follows this pattern:
- `<cluster-name>-vhost.conf` - Virtual host configuration with DocumentRoot

## Example: Managing Two Clusters

```
/etc/httpd/conf.d/
├── ocp4-prod-vhost.conf         # Production cluster vhost
└── ocp4-dev-vhost.conf          # Development cluster vhost
```

## Directory Structure

Each cluster gets its own directory under the main DocumentRoot:

```
/var/www/html/
├── ocp4-prod/
│   ├── ignition/
│   │   ├── bootstrap.ign
│   │   ├── master.ign
│   │   └── worker.ign
│   └── images/
│       ├── rhcos-live-kernel-ppc64le
│       ├── rhcos-live-initramfs.ppc64le.img
│       └── rhcos-live-rootfs.ppc64le.img
└── ocp4-dev/
    ├── ignition/
    └── images/
```

## How It Works

1. **Apache automatically loads** all `.conf` files from `/etc/httpd/conf.d/`
2. **Each cluster** has its own virtual host configuration
3. **URL structure**: `http://helper-ip:port/<cluster-name>/ignition/bootstrap.ign`
4. **Isolation**: Each cluster's files are in separate directories

## Benefits

- ✅ **Multi-cluster support**: Serve files for multiple OpenShift clusters
- ✅ **Isolation**: Each cluster's files are in separate directories
- ✅ **Easy updates**: Modify one cluster without affecting others
- ✅ **Easy cleanup**: Remove cluster by deleting its vhost config and directory
- ✅ **Simple URLs**: Clear URL structure with cluster name prefix

## Removing a Cluster

```bash
CLUSTER_NAME="ocp4-prod"
rm -f /etc/httpd/conf.d/${CLUSTER_NAME}-vhost.conf
rm -rf /var/www/html/${CLUSTER_NAME}
systemctl reload httpd
```

## Listing Configured Clusters

```bash
ls -1 /etc/httpd/conf.d/*-vhost.conf | sed 's|.*/||; s|-vhost.conf||'