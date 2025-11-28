package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/GizmoTickler/truenas-scale-csi/pkg/truenas"
)

// ISCSIAuditResult holds the results of the iSCSI audit
type ISCSIAuditResult struct {
	TargetsWithoutExtents         []TargetInfo
	ExtentsWithoutTargets         []ExtentInfo
	TargetsWithoutConnection      []TargetConnectionInfo
	OrphanedExtents               []ExtentInfo // Extents pointing to non-existent zvols
	OrphanedTargetsWithDataset    []TargetInfo // Target exists, no extent, but dataset exists
	OrphanedTargetsWithoutDataset []TargetInfo // Target exists, no extent, no dataset (safe to delete)
}

type TargetInfo struct {
	ID          int
	Name        string
	HasDataset  bool
	DatasetPath string
}

type ExtentInfo struct {
	ID   int
	Name string
	Disk string
}

type TargetConnectionInfo struct {
	Target           TargetInfo
	HasExtent        bool
	ExtentInfo       *ExtentInfo
	ActiveSessions   int
	PossibleOrphaned bool
}

var (
	parentDataset = flag.String("parent-dataset", "flashstor/k8s-csi", "Parent dataset path for PVCs")
	cleanup       = flag.Bool("cleanup", false, "Clean up orphaned targets without datasets")
	dryRun        = flag.Bool("dry-run", true, "Dry run mode (default true, set to false to actually delete)")
	debugSessions = flag.Bool("debug-sessions", false, "Debug: dump raw session data")
)

func main() {
	flag.Parse()

	apiKey := os.Getenv("TRUENAS_API_KEY")
	if apiKey == "" {
		fmt.Println("TRUENAS_API_KEY not set")
		os.Exit(1)
	}

	host := os.Getenv("TRUENAS_HOST")
	if host == "" {
		host = "nas01.achva.casa"
	}

	client, err := truenas.NewClient(&truenas.ClientConfig{
		Host:              host,
		Port:              443,
		Protocol:          "https",
		APIKey:            apiKey,
		AllowInsecure:     true,
		Timeout:           60 * time.Second,
		MaxConcurrentReqs: 1,
		MaxConnections:    1,
	})
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Debug mode: dump raw session data
	if *debugSessions {
		dumpRawSessions(ctx, client)
		return
	}

	fmt.Println("=== TrueNAS iSCSI Audit Tool ===")
	fmt.Printf("Parent dataset: %s\n", *parentDataset)
	fmt.Println()

	result, err := auditISCSI(ctx, client, *parentDataset)
	if err != nil {
		fmt.Printf("Audit failed: %v\n", err)
		os.Exit(1)
	}

	printAuditResults(result)

	// Cleanup if requested
	if *cleanup && len(result.OrphanedTargetsWithoutDataset) > 0 {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("         CLEANUP ORPHANED TARGETS")
		fmt.Println("========================================")
		if *dryRun {
			fmt.Println("DRY RUN MODE - no changes will be made")
			fmt.Println("Run with -dry-run=false to actually delete")
		}
		fmt.Println()

		cleanupOrphanedTargets(ctx, client, result.OrphanedTargetsWithoutDataset, *dryRun)
	}
}

func auditISCSI(ctx context.Context, client *truenas.Client, parentDataset string) (*ISCSIAuditResult, error) {
	result := &ISCSIAuditResult{}

	// 1. Get all targets
	fmt.Println("Fetching iSCSI targets...")
	targets, err := getAllTargets(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}
	fmt.Printf("  Found %d targets\n", len(targets))

	// 2. Get all extents
	fmt.Println("Fetching iSCSI extents...")
	extents, err := getAllExtents(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get extents: %w", err)
	}
	fmt.Printf("  Found %d extents\n", len(extents))

	// 3. Get all target-extent associations
	fmt.Println("Fetching target-extent associations...")
	associations, err := getAllTargetExtents(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get associations: %w", err)
	}
	fmt.Printf("  Found %d associations\n", len(associations))

	// Build lookup maps (needed for session lookup)
	targetMap := make(map[int]*truenas.ISCSITarget)
	targetNameToID := make(map[string]int)
	for _, t := range targets {
		targetMap[t.ID] = t
		targetNameToID[t.Name] = t.ID
	}

	extentMap := make(map[int]*truenas.ISCSIExtent)
	for _, e := range extents {
		extentMap[e.ID] = e
	}

	// 4. Get active iSCSI sessions
	fmt.Println("Fetching active iSCSI sessions...")
	sessions, err := getActiveSessions(ctx, client, targetNameToID)
	if err != nil {
		fmt.Printf("  Warning: Failed to get sessions: %v\n", err)
		sessions = make(map[int]int) // empty map to continue
	} else {
		fmt.Printf("  Found sessions for %d targets\n", len(sessions))
	}

	// Track which targets have extents and which extents have targets
	targetsWithExtents := make(map[int]bool)
	extentsWithTargets := make(map[int]bool)
	targetToExtent := make(map[int]*truenas.ISCSIExtent)

	for _, assoc := range associations {
		targetsWithExtents[assoc.Target] = true
		extentsWithTargets[assoc.Extent] = true
		if e, ok := extentMap[assoc.Extent]; ok {
			targetToExtent[assoc.Target] = e
		}
	}

	// 5. Find targets without extents and check for corresponding datasets
	fmt.Println("Analyzing targets without extents...")
	fmt.Println("  Checking for corresponding datasets...")
	for _, t := range targets {
		if !targetsWithExtents[t.ID] {
			// Check if dataset exists for this target
			hasDataset, datasetPath, err := checkDatasetExists(ctx, client, t.Name, parentDataset)
			if err != nil {
				fmt.Printf("  Warning: Could not check dataset for %s: %v\n", t.Name, err)
			}

			info := TargetInfo{
				ID:          t.ID,
				Name:        t.Name,
				HasDataset:  hasDataset,
				DatasetPath: datasetPath,
			}

			result.TargetsWithoutExtents = append(result.TargetsWithoutExtents, info)

			if hasDataset {
				result.OrphanedTargetsWithDataset = append(result.OrphanedTargetsWithDataset, info)
			} else {
				result.OrphanedTargetsWithoutDataset = append(result.OrphanedTargetsWithoutDataset, info)
			}
		}
	}

	// 6. Find extents without targets
	fmt.Println("Analyzing extents without targets...")
	for _, e := range extents {
		if !extentsWithTargets[e.ID] {
			result.ExtentsWithoutTargets = append(result.ExtentsWithoutTargets, ExtentInfo{
				ID:   e.ID,
				Name: e.Name,
				Disk: e.Disk,
			})
		}
	}

	// 7. Find targets without active connections (potentially orphaned)
	fmt.Println("Analyzing targets without active connections...")
	for _, t := range targets {
		sessionCount := sessions[t.ID]
		hasExtent := targetsWithExtents[t.ID]

		info := TargetConnectionInfo{
			Target: TargetInfo{
				ID:   t.ID,
				Name: t.Name,
			},
			HasExtent:      hasExtent,
			ActiveSessions: sessionCount,
		}

		if e, ok := targetToExtent[t.ID]; ok {
			info.ExtentInfo = &ExtentInfo{
				ID:   e.ID,
				Name: e.Name,
				Disk: e.Disk,
			}
		}

		// A target is possibly orphaned if it has no sessions AND either:
		// - Has no extent (incomplete setup)
		// - Has extent but no connections (unused)
		if sessionCount == 0 {
			info.PossibleOrphaned = true
			result.TargetsWithoutConnection = append(result.TargetsWithoutConnection, info)
		}
	}

	// 8. Check for extents pointing to non-existent zvols
	fmt.Println("Checking for extents with missing zvols...")
	for _, e := range extents {
		if e.Type == "DISK" && strings.HasPrefix(e.Disk, "zvol/") {
			// Extract dataset name from zvol path
			datasetName := strings.TrimPrefix(e.Disk, "zvol/")
			exists, err := checkZvolExists(ctx, client, datasetName)
			if err != nil {
				fmt.Printf("  Warning: Could not check zvol %s: %v\n", datasetName, err)
				continue
			}
			if !exists {
				result.OrphanedExtents = append(result.OrphanedExtents, ExtentInfo{
					ID:   e.ID,
					Name: e.Name,
					Disk: e.Disk,
				})
			}
		}
	}

	fmt.Println()
	return result, nil
}

func getAllTargets(ctx context.Context, client *truenas.Client) ([]*truenas.ISCSITarget, error) {
	result, err := client.Call(ctx, "iscsi.target.query", []interface{}{}, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	items, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	var targets []*truenas.ISCSITarget
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		t := &truenas.ISCSITarget{}
		if v, ok := m["id"].(float64); ok {
			t.ID = int(v)
		}
		if v, ok := m["name"].(string); ok {
			t.Name = v
		}
		targets = append(targets, t)
	}

	return targets, nil
}

func getAllExtents(ctx context.Context, client *truenas.Client) ([]*truenas.ISCSIExtent, error) {
	result, err := client.Call(ctx, "iscsi.extent.query", []interface{}{}, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	items, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	var extents []*truenas.ISCSIExtent
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		e := &truenas.ISCSIExtent{}
		if v, ok := m["id"].(float64); ok {
			e.ID = int(v)
		}
		if v, ok := m["name"].(string); ok {
			e.Name = v
		}
		if v, ok := m["disk"].(string); ok {
			e.Disk = v
		}
		if v, ok := m["type"].(string); ok {
			e.Type = v
		}
		extents = append(extents, e)
	}

	return extents, nil
}

func getAllTargetExtents(ctx context.Context, client *truenas.Client) ([]*truenas.ISCSITargetExtent, error) {
	result, err := client.Call(ctx, "iscsi.targetextent.query", []interface{}{}, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	items, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	var associations []*truenas.ISCSITargetExtent
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		a := &truenas.ISCSITargetExtent{}
		if v, ok := m["id"].(float64); ok {
			a.ID = int(v)
		}
		if v, ok := m["target"].(float64); ok {
			a.Target = int(v)
		}
		if v, ok := m["extent"].(float64); ok {
			a.Extent = int(v)
		}
		associations = append(associations, a)
	}

	return associations, nil
}

func dumpRawSessions(ctx context.Context, client *truenas.Client) {
	fmt.Println("=== Raw iSCSI Session Data ===")
	fmt.Println()

	result, err := client.Call(ctx, "iscsi.global.sessions")
	if err != nil {
		fmt.Printf("Error fetching sessions: %v\n", err)
		return
	}

	items, ok := result.([]interface{})
	if !ok {
		fmt.Printf("Unexpected result type: %T\n", result)
		fmt.Printf("Raw result: %+v\n", result)
		return
	}

	fmt.Printf("Found %d sessions:\n\n", len(items))

	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			fmt.Printf("Session %d: unexpected type %T\n", i, item)
			continue
		}

		fmt.Printf("Session %d:\n", i)
		// Print all keys in the session object
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %v\n", k, m[k])
		}
		fmt.Println()
	}
}

func getActiveSessions(ctx context.Context, client *truenas.Client, targetNameToID map[string]int) (map[int]int, error) {
	// Returns map of target_id -> session_count
	result, err := client.Call(ctx, "iscsi.global.sessions")
	if err != nil {
		return nil, err
	}

	sessions := make(map[int]int)

	items, ok := result.([]interface{})
	if !ok {
		return sessions, nil
	}

	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Try to get target ID directly
		if targetID, ok := m["target"].(float64); ok {
			sessions[int(targetID)]++
			continue
		}

		// Try target_alias (target name) and look up ID
		if targetName, ok := m["target_alias"].(string); ok && targetName != "" {
			if id, found := targetNameToID[targetName]; found {
				sessions[id]++
				continue
			}
		}

		// Try target_name field
		if targetName, ok := m["target_name"].(string); ok && targetName != "" {
			if id, found := targetNameToID[targetName]; found {
				sessions[id]++
				continue
			}
		}

		// Try extracting from IQN format (iqn.xxx:targetname)
		if iqn, ok := m["target"].(string); ok && iqn != "" {
			// IQN format: iqn.2005-10.org.freenas.ctl:pvc-xxx
			parts := strings.Split(iqn, ":")
			if len(parts) >= 2 {
				targetName := parts[len(parts)-1]
				if id, found := targetNameToID[targetName]; found {
					sessions[id]++
				}
			}
		}
	}

	return sessions, nil
}

func checkZvolExists(ctx context.Context, client *truenas.Client, datasetName string) (bool, error) {
	filters := [][]interface{}{{"id", "=", datasetName}}
	result, err := client.Call(ctx, "pool.dataset.query", filters, map[string]interface{}{})
	if err != nil {
		return false, err
	}

	items, ok := result.([]interface{})
	if !ok {
		return false, nil
	}

	return len(items) > 0, nil
}

// checkDatasetExists checks if a dataset exists for a given target name
func checkDatasetExists(ctx context.Context, client *truenas.Client, targetName string, parentDataset string) (bool, string, error) {
	datasetPath := parentDataset + "/" + targetName
	filters := [][]interface{}{{"id", "=", datasetPath}}
	result, err := client.Call(ctx, "pool.dataset.query", filters, map[string]interface{}{})
	if err != nil {
		return false, datasetPath, err
	}

	items, ok := result.([]interface{})
	if !ok {
		return false, datasetPath, nil
	}

	return len(items) > 0, datasetPath, nil
}

// cleanupOrphanedTargets deletes orphaned targets that have no datasets
func cleanupOrphanedTargets(ctx context.Context, client *truenas.Client, targets []TargetInfo, dryRun bool) {
	deleted := 0
	for _, t := range targets {
		if dryRun {
			fmt.Printf("  [DRY RUN] Would delete target ID %d: %s\n", t.ID, t.Name)
			deleted++
		} else {
			fmt.Printf("  Deleting target ID %d: %s...", t.ID, t.Name)
			err := client.ISCSITargetDelete(ctx, t.ID, true)
			if err != nil {
				fmt.Printf(" FAILED: %v\n", err)
			} else {
				fmt.Printf(" OK\n")
				deleted++
			}
		}
	}

	if dryRun {
		fmt.Printf("\nWould delete %d orphaned targets\n", deleted)
	} else {
		fmt.Printf("\nDeleted %d orphaned targets\n", deleted)
	}
}

func printAuditResults(result *ISCSIAuditResult) {
	fmt.Println("========================================")
	fmt.Println("         iSCSI AUDIT RESULTS")
	fmt.Println("========================================")
	fmt.Println()

	// Sort results by ID for consistent output
	sort.Slice(result.TargetsWithoutExtents, func(i, j int) bool {
		return result.TargetsWithoutExtents[i].ID < result.TargetsWithoutExtents[j].ID
	})
	sort.Slice(result.ExtentsWithoutTargets, func(i, j int) bool {
		return result.ExtentsWithoutTargets[i].ID < result.ExtentsWithoutTargets[j].ID
	})
	sort.Slice(result.TargetsWithoutConnection, func(i, j int) bool {
		return result.TargetsWithoutConnection[i].Target.ID < result.TargetsWithoutConnection[j].Target.ID
	})

	// 1. Targets without extents - categorized
	fmt.Println("🔴 TARGETS WITHOUT EXTENTS (incomplete setup):")
	fmt.Println("   These targets have no associated storage extent.")
	fmt.Println()

	if len(result.TargetsWithoutExtents) == 0 {
		fmt.Println("   ✅ None found")
	} else {
		// Show targets WITH datasets (need investigation - extent creation failed)
		if len(result.OrphanedTargetsWithDataset) > 0 {
			fmt.Printf("   WITH DATASET (%d) - extent creation may have failed:\n", len(result.OrphanedTargetsWithDataset))
			fmt.Println("   (These may be recoverable - the zvol exists but extent wasn't created)")
			for _, t := range result.OrphanedTargetsWithDataset {
				fmt.Printf("     - ID: %d, Name: %s\n", t.ID, t.Name)
				fmt.Printf("       Dataset: %s (EXISTS)\n", t.DatasetPath)
			}
			fmt.Println()
		}

		// Show targets WITHOUT datasets (safe to delete)
		if len(result.OrphanedTargetsWithoutDataset) > 0 {
			fmt.Printf("   WITHOUT DATASET (%d) - SAFE TO DELETE:\n", len(result.OrphanedTargetsWithoutDataset))
			fmt.Println("   (These are completely orphaned - no data associated)")
			for _, t := range result.OrphanedTargetsWithoutDataset {
				fmt.Printf("     - ID: %d, Name: %s\n", t.ID, t.Name)
			}
			fmt.Println()
			fmt.Println("   Run with -cleanup -dry-run=false to delete these orphaned targets")
		}
	}
	fmt.Println()

	// 2. Extents without targets
	fmt.Println("🟠 EXTENTS WITHOUT TARGETS (orphaned extents):")
	fmt.Println("   These extents are not associated with any target.")
	fmt.Println("   They consume storage but are inaccessible via iSCSI.")
	fmt.Println()
	if len(result.ExtentsWithoutTargets) == 0 {
		fmt.Println("   ✅ None found")
	} else {
		for _, e := range result.ExtentsWithoutTargets {
			fmt.Printf("   - ID: %d, Name: %s, Disk: %s\n", e.ID, e.Name, e.Disk)
		}
	}
	fmt.Println()

	// 3. Orphaned extents (extent -> missing zvol)
	fmt.Println("🔴 EXTENTS WITH MISSING ZVOLS (broken extents):")
	fmt.Println("   These extents reference zvols that no longer exist.")
	fmt.Println("   They should be cleaned up immediately.")
	fmt.Println()
	if len(result.OrphanedExtents) == 0 {
		fmt.Println("   ✅ None found")
	} else {
		for _, e := range result.OrphanedExtents {
			fmt.Printf("   - ID: %d, Name: %s, Missing zvol: %s\n", e.ID, e.Name, e.Disk)
		}
	}
	fmt.Println()

	// 4. Targets without connections
	fmt.Println("🟡 TARGETS WITHOUT ACTIVE CONNECTIONS:")
	fmt.Println("   These targets have no active iSCSI sessions.")
	fmt.Println("   This may be normal (unused volumes) or indicate orphaned resources.")
	fmt.Println()
	if len(result.TargetsWithoutConnection) == 0 {
		fmt.Println("   ✅ None found (all targets have active connections)")
	} else {
		withExtent := 0
		withoutExtent := 0
		for _, t := range result.TargetsWithoutConnection {
			if t.HasExtent {
				withExtent++
			} else {
				withoutExtent++
			}
		}

		if withoutExtent > 0 {
			fmt.Println("   Without extent (definitely orphaned):")
			for _, t := range result.TargetsWithoutConnection {
				if !t.HasExtent {
					fmt.Printf("     - ID: %d, Name: %s\n", t.Target.ID, t.Target.Name)
				}
			}
			fmt.Println()
		}

		if withExtent > 0 {
			fmt.Println("   With extent but no sessions (may be unused):")
			for _, t := range result.TargetsWithoutConnection {
				if t.HasExtent && t.ExtentInfo != nil {
					fmt.Printf("     - ID: %d, Name: %s\n", t.Target.ID, t.Target.Name)
					fmt.Printf("       Extent: %s -> %s\n", t.ExtentInfo.Name, t.ExtentInfo.Disk)
				}
			}
		}
	}
	fmt.Println()

	// Summary
	fmt.Println("========================================")
	fmt.Println("              SUMMARY")
	fmt.Println("========================================")
	fmt.Printf("Targets without extents:        %d\n", len(result.TargetsWithoutExtents))
	fmt.Printf("  - With dataset (investigate):  %d\n", len(result.OrphanedTargetsWithDataset))
	fmt.Printf("  - Without dataset (delete):    %d\n", len(result.OrphanedTargetsWithoutDataset))
	fmt.Printf("Extents without targets:        %d\n", len(result.ExtentsWithoutTargets))
	fmt.Printf("Extents with missing zvols:     %d\n", len(result.OrphanedExtents))
	fmt.Printf("Targets without connections:    %d\n", len(result.TargetsWithoutConnection))

	totalIssues := len(result.TargetsWithoutExtents) + len(result.ExtentsWithoutTargets) + len(result.OrphanedExtents)
	if totalIssues > 0 {
		fmt.Println()
		fmt.Println("⚠️  Found orphaned iSCSI resources that should be investigated!")
		if len(result.OrphanedTargetsWithoutDataset) > 0 {
			fmt.Printf("\nTo cleanup %d orphaned targets (without datasets), run:\n", len(result.OrphanedTargetsWithoutDataset))
			fmt.Println("  go run ./cmd/debug-api -cleanup -dry-run=false")
		}
	} else {
		fmt.Println()
		fmt.Println("✅ No critical orphaned resources found")
	}
}
