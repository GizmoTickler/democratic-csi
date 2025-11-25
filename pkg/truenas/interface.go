package truenas

import (
	"context"
)

// ClientInterface defines the interface for the TrueNAS API client.
// This allows for mocking the client in unit tests.
type ClientInterface interface {
	// Core methods
	Close() error
	IsConnected() bool
	Call(method string, params ...interface{}) (interface{}, error)
	CallWithContext(ctx context.Context, method string, params ...interface{}) (interface{}, error)

	// Dataset methods
	DatasetCreate(params *DatasetCreateParams) (*Dataset, error)
	DatasetDelete(name string, recursive bool, force bool) error
	DatasetGet(name string) (*Dataset, error)
	DatasetUpdate(name string, params *DatasetUpdateParams) (*Dataset, error)
	DatasetList(parentName string) ([]*Dataset, error)
	DatasetSetUserProperty(name string, key string, value string) error
	DatasetGetUserProperty(name string, key string) (string, error)
	DatasetExpand(name string, newSize int64) error
	GetPoolAvailable(poolName string) (int64, error)

	// Snapshot methods
	SnapshotCreate(dataset string, name string) (*Snapshot, error)
	SnapshotDelete(snapshotID string, defer_ bool, recursive bool) error
	SnapshotGet(snapshotID string) (*Snapshot, error)
	SnapshotList(dataset string) ([]*Snapshot, error)
	SnapshotListAll(parentDataset string) ([]*Snapshot, error)
	SnapshotSetUserProperty(snapshotID string, key string, value string) error
	SnapshotClone(snapshotID string, newDatasetName string) error
	SnapshotRollback(snapshotID string, force bool, recursive bool, recursiveClones bool) error

	// NFS methods
	NFSShareCreate(params *NFSShareCreateParams) (*NFSShare, error)
	NFSShareDelete(id int) error
	NFSShareGet(id int) (*NFSShare, error)
	NFSShareFindByPath(path string) (*NFSShare, error)
	NFSShareList() ([]*NFSShare, error)
	NFSShareUpdate(id int, params map[string]interface{}) (*NFSShare, error)

	// iSCSI methods
	ISCSITargetCreate(name string, alias string, mode string, groups []ISCSITargetGroup) (*ISCSITarget, error)
	ISCSITargetDelete(id int, force bool) error
	ISCSITargetGet(id int) (*ISCSITarget, error)
	ISCSITargetFindByName(name string) (*ISCSITarget, error)
	ISCSIExtentCreate(name string, diskPath string, comment string, blocksize int, rpm string) (*ISCSIExtent, error)
	ISCSIExtentDelete(id int, remove bool, force bool) error
	ISCSIExtentGet(id int) (*ISCSIExtent, error)
	ISCSIExtentFindByName(name string) (*ISCSIExtent, error)
	ISCSITargetExtentCreate(targetID int, extentID int, lunID int) (*ISCSITargetExtent, error)
	ISCSITargetExtentDelete(id int, force bool) error
	ISCSITargetExtentFind(targetID int, extentID int) (*ISCSITargetExtent, error)
	ISCSIGlobalConfigGet() (*ISCSIGlobalConfig, error)

	// NVMe-oF methods
	NVMeoFSubsystemCreate(nqn string, serial string, allowAnyHost bool, hosts []string) (*NVMeoFSubsystem, error)
	NVMeoFSubsystemDelete(id int) error
	NVMeoFSubsystemGet(id int) (*NVMeoFSubsystem, error)
	NVMeoFSubsystemFindByNQN(nqn string) (*NVMeoFSubsystem, error)
	NVMeoFNamespaceCreate(subsystemID int, devicePath string) (*NVMeoFNamespace, error)
	NVMeoFNamespaceDelete(id int) error
	NVMeoFNamespaceGet(id int) (*NVMeoFNamespace, error)
	NVMeoFNamespaceFindByDevice(subsystemID int, devicePath string) (*NVMeoFNamespace, error)
	NVMeoFPortList() ([]*NVMeoFPort, error)
	NVMeoFGetTransportAddresses(transport string) ([]string, error)
}
