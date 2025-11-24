![Image](https://img.shields.io/docker/pulls/democraticcsi/democratic-csi.svg)
![Image](https://img.shields.io/github/actions/workflow/status/democratic-csi/democratic-csi/main.yml?branch=master&style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/democratic-csi)](https://artifacthub.io/packages/search?repo=democratic-csi)

# Introduction

`democratic-csi` implements the `csi` (container storage interface) spec
providing storage for various container orchestration systems (ie: Kubernetes).

This version focuses exclusively on providing storage via iSCSI/NFS/NVMe-oF from
**TrueNAS SCALE 25.04+** using the modern WebSocket JSON-RPC 2.0 API.

The drivers implement the depth and breadth of the `csi` spec, so you
have access to resizing, snapshots, clones, etc functionality.

`democratic-csi` is 2 things:

- TrueNAS SCALE 25.04+ CSI driver implementations
  - `truenas-nfs` (manages ZFS datasets to share over NFS)
  - `truenas-iscsi` (manages ZFS zvols to share over iSCSI)
  - `truenas-nvmeof` (manages ZFS zvols to share over NVMe-oF)
- framework for developing `csi` drivers

## Key Features

- **WebSocket JSON-RPC 2.0 API**: No SSH required - all operations via WebSocket
- **Modern TrueNAS SCALE 25.04+**: Uses the latest versioned API (`/api/current`)
- **Three Storage Protocols**: NFS, iSCSI, and NVMe-oF support
- **Full CSI Spec**: Volume resizing, snapshots, clones, and more
- **Persistent Connection**: Auto-reconnecting WebSocket with authentication
- **API Key Auth**: Secure authentication via TrueNAS API keys

If you have any interest in providing a `csi` driver, simply open an issue to
discuss. The project provides an extensive framework to build from making it
relatively easy to implement new drivers.

# Installation

Predominantly 3 things are needed:

- node prep (ie: your kubernetes cluster nodes)
- server prep (ie: your storage server)
- deploy the driver into the cluster (`helm` chart provided with sample
  `values.yaml`)

## Community Guides

- https://jonathangazeley.com/2021/01/05/using-truenas-to-provide-persistent-storage-for-kubernetes/
- https://www.lisenet.com/2021/moving-to-truenas-and-democratic-csi-for-kubernetes-persistent-storage/
- https://gist.github.com/admun/4372899f20421a947b7544e5fc9f9117 (migrating
  from `nfs-client-provisioner` to `democratic-csi`)
- https://gist.github.com/deefdragon/d58a4210622ff64088bd62a5d8a4e8cc
  (migrating between storage classes using `velero`)
- https://github.com/fenio/k8s-truenas (NFS/iSCSI over API with TrueNAS Scale)

## Node Prep

You should install/configure the requirements for both nfs and iscsi.

### cifs

```bash
# RHEL / CentOS
sudo yum install -y cifs-utils

# Ubuntu / Debian
sudo apt-get install -y cifs-utils
```

### nfs

```bash
# RHEL / CentOS
sudo yum install -y nfs-utils

# Ubuntu / Debian
sudo apt-get install -y nfs-common
```

### iscsi

Note that `multipath` is supported for the `iscsi`-based drivers. Simply setup
multipath to your liking and set multiple portals in the config as appropriate.

If you are running Kubernetes with rancher/rke please see the following:

- https://github.com/rancher/rke/issues/1846

#### RHEL / CentOS

```bash
# Install the following system packages
sudo yum install -y lsscsi iscsi-initiator-utils sg3_utils device-mapper-multipath

# Enable multipathing
sudo mpathconf --enable --with_multipathd y

# Ensure that iscsid and multipathd are running
sudo systemctl enable iscsid multipathd
sudo systemctl start iscsid multipathd

# Start and enable iscsi
sudo systemctl enable iscsi
sudo systemctl start iscsi
```

#### Ubuntu / Debian

```
# Install the following system packages
sudo apt-get install -y open-iscsi lsscsi sg3-utils multipath-tools scsitools

# Enable multipathing
sudo tee /etc/multipath.conf <<-'EOF'
defaults {
    user_friendly_names yes
    find_multipaths yes
}
EOF

sudo systemctl enable multipath-tools.service
sudo service multipath-tools restart

# Ensure that open-iscsi and multipath-tools are enabled and running
sudo systemctl status multipath-tools
sudo systemctl enable open-iscsi.service
sudo service open-iscsi start
sudo systemctl status open-iscsi
```

#### [Talos](https://www.talos.dev/)

To use iscsi storage in kubernetes cluster in talos these steps are needed which are similar to the ones explained in https://www.talos.dev/v1.1/kubernetes-guides/configuration/replicated-local-storage-with-openebs-jiva/#patching-the-jiva-installation

##### Patch nodes

since talos does not have iscsi support by default, the iscsi extension is needed
create a `patch.yaml` file with

```yaml
- op: add
  path: /machine/install/extensions
  value:
    - image: ghcr.io/siderolabs/iscsi-tools:v0.1.1
```

and apply the patch across all of your nodes

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> patch mc -p @patch.yaml
```

the extension will not activate until you "upgrade" the nodes, even if there is no update, use the latest version of talos installer.
VERIFY THE TALOS VERSION IN THIS COMMAND BEFORE RUNNING IT AND READ THE [OpenEBS Jiva](https://www.talos.dev/v1.1/kubernetes-guides/configuration/replicated-local-storage-with-openebs-jiva/#patching-the-jiva-installation).
upgrade all of the nodes in the cluster to get the extension

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> upgrade --image=ghcr.io/siderolabs/installer:v1.1.1
```

in your `values.yaml` file make sure to enable these settings

```yaml
node:
  hostPID: true
  driver:
    extraEnv:
      - name: ISCSIADM_HOST_STRATEGY
        value: nsenter
      - name: ISCSIADM_HOST_PATH
        value: /usr/local/sbin/iscsiadm
    iscsiDirHostPath: /usr/local/etc/iscsi
    iscsiDirHostPathType: ""
```

and continue your democratic installation as usuall with other iscsi drivers.

#### Privileged Namespace

democratic-csi requires privileged access to the nodes, so the namespace should allow for privileged pods. One way of doing it is via [namespace labels](https://kubernetes.io/docs/tasks/configure-pod-container/enforce-standards-namespace-labels/).
Add the followin label to the democratic-csi installation namespace `pod-security.kubernetes.io/enforce=privileged`

```
kubectl label --overwrite namespace democratic-csi pod-security.kubernetes.io/enforce=privileged
```

### nvmeof

```bash
# not required but likely helpful (tools are included in the democratic images
# so not needed on the host)
apt-get install -y nvme-cli

# get the nvme fabric modules
apt-get install linux-generic

# ensure the nvmeof modules get loaded at boot
cat <<EOF > /etc/modules-load.d/nvme.conf
nvme
nvme-tcp
nvme-fc
nvme-rdma
EOF

# load the modules immediately
modprobe nvme
modprobe nvme-tcp
modprobe nvme-fc
modprobe nvme-rdma

# nvme has native multipath or can use DM multipath
# democratic-csi will gracefully handle either configuration
# RedHat recommends DM multipath (nvme_core.multipath=N)
cat /sys/module/nvme_core/parameters/multipath

# kernel arg to enable/disable native multipath
nvme_core.multipath=N
```

### zfs-local-ephemeral-inline

This `driver` provisions node-local ephemeral storage on a per-pod basis. Each
node should have an identically named zfs pool created and avaialble to the
`driver`. Note, this is _NOT_ the same thing as using the docker zfs storage
driver (although the same pool could be used). No other requirements are
necessary.

- https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/20190122-csi-inline-volumes.md
- https://kubernetes-csi.github.io/docs/ephemeral-local-volumes.html

### zfs-local-{dataset,zvol}

This `driver` provisions node-local storage. Each node should have an
identically named zfs pool created and avaialble to the `driver`. Note, this is
_NOT_ the same thing as using the docker zfs storage driver (although the same
pool could be used). Nodes should have the standard `zfs` utilities installed.

In the name of ease-of-use these drivers by default report `MULTI_NODE` support
(`ReadWriteMany` in k8s) however the volumes will implicity only work on the
node where originally provisioned. Topology contraints manage this in an
automated fashion preventing any undesirable behavior. So while you may
provision `MULTI_NODE` / `RWX` volumes, any workloads using the volume will
always land on a single node and that node will always be the node where the
volume is/was provisioned.

### local-hostpath

This `driver` provisions node-local storage. Each node should have an
identically name folder where volumes will be created.

In the name of ease-of-use these drivers by default report `MULTI_NODE` support
(`ReadWriteMany` in k8s) however the volumes will implicity only work on the
node where originally provisioned. Topology contraints manage this in an
automated fashion preventing any undesirable behavior. So while you may
provision `MULTI_NODE` / `RWX` volumes, any workloads using the volume will
always land on a single node and that node will always be the node where the
volume is/was provisioned.

The nature of this `driver` also prevents the enforcement of quotas. In short
the requested volume size is generally ignored.

### windows

Support for Windows was introduced in `v1.7.0`. Currently support is limited
to kubernetes nodes capabale of running `HostProcess` containers. Support was
tested against `Windows Server 2019` using `rke2-v1.24`. Currently any of the
`-smb` and `-iscsi` drivers will work. Support for `ntfs` was added to the
linux nodes as well (using the `ntfs3` driver) so volumes created can be
utilized by nodes with either operating system (in the case of `cifs` by both
simultaneously).

If using any `-iscsi` driver be sure your iqns are always fully lower-case by
default (https://github.com/PowerShell/PowerShell/issues/17306).

Due to current limits in the kubernetes tooling it is not possible to use the
`local-hostpath` driver but support is implemented in this project and will
work as soon as kubernetes support is available.

```powershell
# ensure all updates are installed

# enable the container feature
Enable-WindowsOptionalFeature -Online -FeatureName Containers –All

# install a HostProcess compatible kubernetes

# smb support
# If using with Windows based machines you may need to enable guest access
# (even if you are connecting with credentials)
Set-ItemProperty HKLM:\SYSTEM\CurrentControlSet\Services\LanmanWorkstation\Parameters AllowInsecureGuestAuth -Value 1
Restart-Service LanmanWorkstation -Force

# iscsi
# enable iscsi service and mpio as appropriate
Get-Service -Name MSiSCSI
Set-Service -Name MSiSCSI -StartupType Automatic
Start-Service -Name MSiSCSI
Get-Service -Name MSiSCSI

# mpio
Get-WindowsFeature -Name 'Multipath-IO'
Add-WindowsFeature -Name 'Multipath-IO'

Enable-MSDSMAutomaticClaim -BusType "iSCSI"
Disable-MSDSMAutomaticClaim -BusType "iSCSI"

Get-MSDSMGlobalDefaultLoadBalancePolicy
Set-MSDSMGlobalLoadBalancePolicy -Policy RR
```

- https://kubernetes.io/blog/2021/08/16/windows-hostprocess-containers/
- https://kubernetes.io/docs/tasks/configure-pod-container/create-hostprocess-pod/

## Server Prep

### TrueNAS SCALE 25.04+ (truenas-nfs, truenas-iscsi, truenas-nvmeof)

**Required**: TrueNAS SCALE 25.04 or later

These drivers use the WebSocket JSON-RPC 2.0 API exclusively - **no SSH required**.
All operations are performed via a persistent WebSocket connection to the
TrueNAS API endpoint (`wss://host/api/current`).

#### TrueNAS Configuration

1. **Enable API Access**
   - Navigate to **Settings → API Keys**
   - Click **Add** to create a new API key
   - Copy the API key (format: `1-xxxxxxxxxxxxxxxxxxxxx`)
   - Store securely - this is used for authentication

2. **Configure Storage Pools**
   - Ensure you have a ZFS pool created (e.g., `tank`)
   - Create parent datasets for volumes and snapshots:
     ```bash
     # Example: Create parent datasets
     zfs create tank/k8s
     zfs create tank/k8s/volumes
     zfs create tank/k8s/snapshots
     ```
   - **Important**: Volume and snapshot datasets should be siblings, not nested

3. **Configure Services**
   Ensure the appropriate services are enabled and running:

   **For NFS (`truenas-nfs`)**:
   - Navigate to **Sharing → NFS**
   - Ensure NFS service is enabled (will be started automatically when shares are created)
   - No pre-configuration needed - shares are created dynamically by the CSI driver

   **For iSCSI (`truenas-iscsi`)**:
   - Navigate to **Sharing → iSCSI**
   - Create Portal (default port 3260)
   - Create Initiator Group (allow appropriate initiators or leave empty for all)
   - Optionally configure CHAP authentication
   - Note the Portal Group ID and Initiator Group ID for configuration
   - Use TrueNAS UI or API to get IDs:
     ```bash
     # Get Portal IDs
     curl -H "Authorization: Bearer YOUR_API_KEY" \
       https://truenas.example.com/api/v2.0/iscsi/portal

     # Get Initiator Group IDs
     curl -H "Authorization: Bearer YOUR_API_KEY" \
       https://truenas.example.com/api/v2.0/iscsi/initiator
     ```
   - Targets and extents are created dynamically by the CSI driver

   **For NVMe-oF (`truenas-nvmeof`)**:
   - Navigate to **Sharing → NVMe-oF**
   - Ensure NVMe-oF service is configured
   - Subsystems and namespaces are created dynamically by the CSI driver
   - Configure transport (TCP recommended, port 4420)

4. **Network Configuration**
   - Ensure the TrueNAS system is reachable from your Kubernetes cluster
   - Open required ports in firewall:
     - **WebSocket API**: 443 (HTTPS) or 80 (HTTP)
     - **NFS**: 2049, 111, 20048
     - **iSCSI**: 3260 (default)
     - **NVMe-oF**: 4420 (TCP default)

5. **TLS/SSL Configuration**
   - For production use, configure a valid TLS certificate
   - For testing, you can use self-signed certificates with `allowInsecure: true`
   - Navigate to **Settings → Certificates** to manage certificates

#### Example Configuration

See the `examples/` directory for complete configuration examples:
- `examples/truenas-nfs.yaml` - NFS driver configuration
- `examples/truenas-iscsi.yaml` - iSCSI driver configuration
- `examples/truenas-nvmeof.yaml` - NVMe-oF driver configuration

Each example includes detailed comments explaining all configuration options.

## Helm Installation

```bash
helm repo add democratic-csi https://democratic-csi.github.io/charts/
helm repo update
# helm v2
helm search democratic-csi/

# helm v3
helm search repo democratic-csi/

# copy proper values file from https://github.com/democratic-csi/charts/tree/master/stable/democratic-csi/examples
# edit as appropriate
# examples are from helm v2, alter as appropriate for v3

# add --create-namespace for helm v3
helm upgrade \
--install \
--values freenas-iscsi.yaml \
--namespace democratic-csi \
zfs-iscsi democratic-csi/democratic-csi

helm upgrade \
--install \
--values freenas-nfs.yaml \
--namespace democratic-csi \
zfs-nfs democratic-csi/democratic-csi
```

### A note on non standard kubelet paths

Some distrobutions, such as `minikube` and `microk8s` use a non-standard
kubelet path. In such cases it is necessary to provide a new kubelet host path,
microk8s example below:

```bash
microk8s helm upgrade \
  --install \
  --values freenas-nfs.yaml \
  --set node.kubeletHostPath="/var/snap/microk8s/common/var/lib/kubelet"  \
  --namespace democratic-csi \
  zfs-nfs democratic-csi/democratic-csi
```

- microk8s - `/var/snap/microk8s/common/var/lib/kubelet`
- pivotal - `/var/vcap/data/kubelet`
- k0s - `/var/lib/k0s/kubelet`

### openshift

`democratic-csi` generally works fine with openshift. Some special parameters
need to be set with helm (support added in chart version `0.6.1`):

```bash
# for sure required
--set node.rbac.openshift.privileged=true
--set node.driver.localtimeHostPath=false

# unlikely, but in special circumstances may be required
--set controller.rbac.openshift.privileged=true
```

### Nomad

`democratic-csi` works with Nomad in a functioning but limted capacity. See the
[Nomad docs](docs/nomad.md) for details.

### Docker Swarm

- https://github.com/moby/moby/blob/master/docs/cluster_volumes.md
- https://github.com/olljanat/csi-plugins-for-docker-swarm

## Multiple Deployments

You may install multiple deployments of each/any driver. It requires the
following:

- Use a new helm release name for each deployment
- Make sure you have a unique `csiDriver.name` in the values file (within the
  same cluster)
- Use unqiue names for your storage classes (per cluster)
- Use a unique parent dataset (ie: don't try to use the same parent across
  deployments or clusters)
- For `iscsi` and `smb` be aware that the names of assets/shares are _global_
  and so collisions are possible/probable. Appropriate use of the respective
  `nameTemplate`, `namePrefix`, and `nameSuffix` configuration options will
  mitigate the issue [#210](https://github.com/democratic-csi/democratic-csi/issues/210).

# Snapshot Support

Install snapshot controller (once per cluster):

- https://github.com/democratic-csi/charts/tree/master/stable/snapshot-controller

OR

- https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd
- https://github.com/kubernetes-csi/external-snapshotter/tree/master/deploy/kubernetes/snapshot-controller

Install `democratic-csi` as usual with `volumeSnapshotClasses` defined as appropriate.

- https://kubernetes.io/docs/concepts/storage/volume-snapshots/
- https://github.com/kubernetes-csi/external-snapshotter#usage
- https://github.com/democratic-csi/democratic-csi/issues/129#issuecomment-961489810

# Migrating from freenas-provisioner and freenas-iscsi-provisioner

It is possible to migrate all volumes from the non-csi freenas provisioners
to `democratic-csi`.

Copy the `contrib/freenas-provisioner-to-democratic-csi.sh` script from the
project to your workstation, read the script in detail, and edit the variables
to your needs to start migrating!

# Related

- https://github.com/nmaupu/freenas-provisioner
- https://github.com/travisghansen/freenas-iscsi-provisioner
- https://datamattsson.tumblr.com/post/624751011659202560/welcome-truenas-core-container-storage-provider
- https://github.com/dravanet/truenas-csi
- https://github.com/SynologyOpenSource/synology-csi
- https://github.com/openebs/zfs-localpv
