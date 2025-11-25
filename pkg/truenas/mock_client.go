package truenas

import (
	"context"
	"fmt"
)

// MockClient is a mock implementation of ClientInterface for testing.
type MockClient struct {
	// Mock data
	Datasets       map[string]*Dataset
	Snapshots      map[string]*Snapshot
	NFSShares      map[int]*NFSShare
	ISCSITargets   map[int]*ISCSITarget
	ISCSIExtents   map[int]*ISCSIExtent
	TargetExtents  map[int]*ISCSITargetExtent
	NVMeSubsystems map[int]*NVMeoFSubsystem
	NVMeNamespaces map[int]*NVMeoFNamespace
	PoolAvailable  int64

	// Error injection
	InjectError error
}

// NewMockClient creates a new MockClient.
func NewMockClient() *MockClient {
	return &MockClient{
		Datasets:       make(map[string]*Dataset),
		Snapshots:      make(map[string]*Snapshot),
		NFSShares:      make(map[int]*NFSShare),
		ISCSITargets:   make(map[int]*ISCSITarget),
		ISCSIExtents:   make(map[int]*ISCSIExtent),
		TargetExtents:  make(map[int]*ISCSITargetExtent),
		NVMeSubsystems: make(map[int]*NVMeoFSubsystem),
		NVMeNamespaces: make(map[int]*NVMeoFNamespace),
		PoolAvailable:  100 * 1024 * 1024 * 1024, // 100 GiB default
	}
}

// Core methods
func (m *MockClient) Close() error                                                   { return nil }
func (m *MockClient) IsConnected() bool                                              { return true }
func (m *MockClient) Call(method string, params ...interface{}) (interface{}, error) { return nil, nil }
func (m *MockClient) CallWithContext(ctx context.Context, method string, params ...interface{}) (interface{}, error) {
	return nil, nil
}

// Dataset methods
func (m *MockClient) DatasetCreate(params *DatasetCreateParams) (*Dataset, error) {
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	if _, exists := m.Datasets[params.Name]; exists {
		// Simulate "already exists" behavior if needed, or return error
		// For now, let's just overwrite or return existing
		return m.Datasets[params.Name], nil
	}

	ds := &Dataset{
		ID:             params.Name,
		Name:           params.Name,
		Type:           params.Type,
		UserProperties: make(map[string]UserProperty),
		Volsize:        DatasetProperty{Parsed: float64(params.Volsize)},
		Refquota:       DatasetProperty{Parsed: float64(params.Refquota)},
	}
	m.Datasets[params.Name] = ds
	return ds, nil
}

func (m *MockClient) DatasetDelete(name string, recursive bool, force bool) error {
	if m.InjectError != nil {
		return m.InjectError
	}
	delete(m.Datasets, name)
	return nil
}

func (m *MockClient) DatasetGet(name string) (*Dataset, error) {
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	if ds, ok := m.Datasets[name]; ok {
		return ds, nil
	}
	return nil, &APIError{Code: -1, Message: "dataset not found"}
}

func (m *MockClient) DatasetUpdate(name string, params *DatasetUpdateParams) (*Dataset, error) {
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	ds, ok := m.Datasets[name]
	if !ok {
		return nil, &APIError{Code: -1, Message: "dataset not found"}
	}

	if params.Volsize > 0 {
		ds.Volsize = DatasetProperty{Parsed: float64(params.Volsize)}
	}
	// Handle other updates as needed
	return ds, nil
}

func (m *MockClient) DatasetList(parentName string) ([]*Dataset, error) {
	var list []*Dataset
	for _, ds := range m.Datasets {
		list = append(list, ds)
	}
	return list, nil
}

func (m *MockClient) DatasetSetUserProperty(name string, key string, value string) error {
	if m.InjectError != nil {
		return m.InjectError
	}
	ds, ok := m.Datasets[name]
	if !ok {
		return &APIError{Code: -1, Message: "dataset not found"}
	}
	ds.UserProperties[key] = UserProperty{Value: value}
	return nil
}

func (m *MockClient) DatasetGetUserProperty(name string, key string) (string, error) {
	ds, ok := m.Datasets[name]
	if !ok {
		return "", &APIError{Code: -1, Message: "dataset not found"}
	}
	if prop, ok := ds.UserProperties[key]; ok {
		return prop.Value, nil
	}
	return "", nil
}

func (m *MockClient) DatasetExpand(name string, newSize int64) error {
	if m.InjectError != nil {
		return m.InjectError
	}
	ds, ok := m.Datasets[name]
	if !ok {
		return &APIError{Code: -1, Message: "dataset not found"}
	}
	ds.Volsize = DatasetProperty{Parsed: float64(newSize)}
	return nil
}

func (m *MockClient) GetPoolAvailable(poolName string) (int64, error) {
	return m.PoolAvailable, nil
}

// Snapshot methods
func (m *MockClient) SnapshotCreate(dataset string, name string) (*Snapshot, error) {
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	id := fmt.Sprintf("%s@%s", dataset, name)
	snap := &Snapshot{
		ID:             id,
		Name:           name,
		Dataset:        dataset,
		UserProperties: make(map[string]UserProperty),
	}
	m.Snapshots[id] = snap
	return snap, nil
}

func (m *MockClient) SnapshotDelete(snapshotID string, defer_ bool, recursive bool) error {
	if m.InjectError != nil {
		return m.InjectError
	}
	delete(m.Snapshots, snapshotID)
	return nil
}

func (m *MockClient) SnapshotGet(snapshotID string) (*Snapshot, error) {
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	if snap, ok := m.Snapshots[snapshotID]; ok {
		return snap, nil
	}
	return nil, &APIError{Code: -1, Message: "snapshot not found"}
}

func (m *MockClient) SnapshotList(dataset string) ([]*Snapshot, error) {
	var list []*Snapshot
	for _, snap := range m.Snapshots {
		if snap.Dataset == dataset {
			list = append(list, snap)
		}
	}
	return list, nil
}

func (m *MockClient) SnapshotListAll(parentDataset string) ([]*Snapshot, error) {
	var list []*Snapshot
	for _, snap := range m.Snapshots {
		list = append(list, snap)
	}
	return list, nil
}

func (m *MockClient) SnapshotSetUserProperty(snapshotID string, key string, value string) error {
	if m.InjectError != nil {
		return m.InjectError
	}
	snap, ok := m.Snapshots[snapshotID]
	if !ok {
		return &APIError{Code: -1, Message: "snapshot not found"}
	}
	snap.UserProperties[key] = UserProperty{Value: value}
	return nil
}

func (m *MockClient) SnapshotClone(snapshotID string, newDatasetName string) error {
	if m.InjectError != nil {
		return m.InjectError
	}
	// Create a new dataset as a clone
	m.Datasets[newDatasetName] = &Dataset{
		ID:             newDatasetName,
		Name:           newDatasetName,
		UserProperties: make(map[string]UserProperty),
	}
	return nil
}

func (m *MockClient) SnapshotRollback(snapshotID string, force bool, recursive bool, recursiveClones bool) error {
	return nil
}

// NFS methods
func (m *MockClient) NFSShareCreate(params *NFSShareCreateParams) (*NFSShare, error) {
	if m.InjectError != nil {
		return nil, m.InjectError
	}
	id := len(m.NFSShares) + 1
	share := &NFSShare{
		ID:   id,
		Path: params.Path,
	}
	m.NFSShares[id] = share
	return share, nil
}

func (m *MockClient) NFSShareDelete(id int) error {
	delete(m.NFSShares, id)
	return nil
}

func (m *MockClient) NFSShareGet(id int) (*NFSShare, error) {
	if share, ok := m.NFSShares[id]; ok {
		return share, nil
	}
	return nil, fmt.Errorf("share not found")
}

func (m *MockClient) NFSShareFindByPath(path string) (*NFSShare, error) {
	for _, share := range m.NFSShares {
		if share.Path == path {
			return share, nil
		}
	}
	return nil, nil
}

func (m *MockClient) NFSShareList() ([]*NFSShare, error) {
	var list []*NFSShare
	for _, share := range m.NFSShares {
		list = append(list, share)
	}
	return list, nil
}

func (m *MockClient) NFSShareUpdate(id int, params map[string]interface{}) (*NFSShare, error) {
	return m.NFSShares[id], nil
}

// iSCSI methods
func (m *MockClient) ISCSITargetCreate(name string, alias string, mode string, groups []ISCSITargetGroup) (*ISCSITarget, error) {
	id := len(m.ISCSITargets) + 1
	target := &ISCSITarget{ID: id, Name: name, Alias: alias, Mode: mode, Groups: groups}
	m.ISCSITargets[id] = target
	return target, nil
}
func (m *MockClient) ISCSITargetDelete(id int, force bool) error {
	delete(m.ISCSITargets, id)
	return nil
}
func (m *MockClient) ISCSITargetGet(id int) (*ISCSITarget, error) {
	if t, ok := m.ISCSITargets[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *MockClient) ISCSITargetFindByName(name string) (*ISCSITarget, error) {
	for _, t := range m.ISCSITargets {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, nil
}
func (m *MockClient) ISCSIExtentCreate(name string, diskPath string, comment string, blocksize int, rpm string) (*ISCSIExtent, error) {
	id := len(m.ISCSIExtents) + 1
	ext := &ISCSIExtent{ID: id, Name: name, Disk: diskPath}
	m.ISCSIExtents[id] = ext
	return ext, nil
}
func (m *MockClient) ISCSIExtentDelete(id int, remove bool, force bool) error {
	delete(m.ISCSIExtents, id)
	return nil
}
func (m *MockClient) ISCSIExtentGet(id int) (*ISCSIExtent, error) {
	if e, ok := m.ISCSIExtents[id]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *MockClient) ISCSIExtentFindByName(name string) (*ISCSIExtent, error) {
	for _, e := range m.ISCSIExtents {
		if e.Name == name {
			return e, nil
		}
	}
	return nil, nil
}
func (m *MockClient) ISCSITargetExtentCreate(targetID int, extentID int, lunID int) (*ISCSITargetExtent, error) {
	id := len(m.TargetExtents) + 1
	te := &ISCSITargetExtent{ID: id, Target: targetID, Extent: extentID, LunID: lunID}
	m.TargetExtents[id] = te
	return te, nil
}
func (m *MockClient) ISCSITargetExtentDelete(id int, force bool) error {
	delete(m.TargetExtents, id)
	return nil
}
func (m *MockClient) ISCSITargetExtentFind(targetID int, extentID int) (*ISCSITargetExtent, error) {
	for _, te := range m.TargetExtents {
		if te.Target == targetID && te.Extent == extentID {
			return te, nil
		}
	}
	return nil, nil
}
func (m *MockClient) ISCSIGlobalConfigGet() (*ISCSIGlobalConfig, error) {
	return &ISCSIGlobalConfig{Basename: "iqn.2005-10.org.freenas.ctl"}, nil
}

// NVMe-oF methods
func (m *MockClient) NVMeoFSubsystemCreate(nqn string, serial string, allowAnyHost bool, hosts []string) (*NVMeoFSubsystem, error) {
	id := len(m.NVMeSubsystems) + 1
	sub := &NVMeoFSubsystem{ID: id, NQN: nqn}
	m.NVMeSubsystems[id] = sub
	return sub, nil
}
func (m *MockClient) NVMeoFSubsystemDelete(id int) error {
	delete(m.NVMeSubsystems, id)
	return nil
}
func (m *MockClient) NVMeoFSubsystemGet(id int) (*NVMeoFSubsystem, error) {
	if s, ok := m.NVMeSubsystems[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *MockClient) NVMeoFSubsystemFindByNQN(nqn string) (*NVMeoFSubsystem, error) {
	for _, s := range m.NVMeSubsystems {
		if s.NQN == nqn {
			return s, nil
		}
	}
	return nil, nil
}
func (m *MockClient) NVMeoFNamespaceCreate(subsystemID int, devicePath string) (*NVMeoFNamespace, error) {
	id := len(m.NVMeNamespaces) + 1
	ns := &NVMeoFNamespace{ID: id, Subsystem: subsystemID, DevicePath: devicePath}
	m.NVMeNamespaces[id] = ns
	return ns, nil
}
func (m *MockClient) NVMeoFNamespaceDelete(id int) error {
	delete(m.NVMeNamespaces, id)
	return nil
}
func (m *MockClient) NVMeoFNamespaceGet(id int) (*NVMeoFNamespace, error) {
	if n, ok := m.NVMeNamespaces[id]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *MockClient) NVMeoFNamespaceFindByDevice(subsystemID int, devicePath string) (*NVMeoFNamespace, error) {
	for _, n := range m.NVMeNamespaces {
		if n.Subsystem == subsystemID && n.DevicePath == devicePath {
			return n, nil
		}
	}
	return nil, nil
}
func (m *MockClient) NVMeoFPortList() ([]*NVMeoFPort, error) {
	return []*NVMeoFPort{{ID: 1, Transport: "tcp", Address: "0.0.0.0", Port: 4420}}, nil
}
func (m *MockClient) NVMeoFGetTransportAddresses(transport string) ([]string, error) {
	return []string{"0.0.0.0"}, nil
}
