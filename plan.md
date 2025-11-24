# Implementation Plan

## Phase 1: Prerequisites & Core Refactoring
This phase prepares the codebase for the migration and integrates essential architectural improvements from open PRs.

1.  **Update Dependencies**: Ensure `ws` and `reconnecting-websocket` are up to date.
2.  **Integrate Core PRs**:
    *   **#469 (Explicit Cleanup)**: Essential for closing WebSocket connections gracefully.
    *   **#471 (Call Context)**: Adds context to driver calls, useful for request tracing.
    *   **#474 (Context Logging)**: Improves logging, critical for debugging async WebSocket interactions.
3.  **Integrate Functional PRs**:
    *   **#517 (StorageClass `datasetParentName`)**
    *   **#501 (Disable TRIM)**
    *   **#218 (ZFS Sudo)**

## Phase 2: TrueNAS SCALE 25.04 Driver (JSON-RPC)
This phase replaces the HTTP-based driver with the new WebSocket-based driver.

### Step 1: WebSocket Client Implementation
Create `src/driver/freenas/websocket/client.js`:
*   **Connection Management**: Use `reconnecting-websocket`.
*   **Authentication**: Implement `auth.login` or `auth.login_with_api_key` upon connection.
*   **Request/Response Correlation**: Use a Map to store pending requests (by ID) and resolve promises when responses arrive.
*   **Timeout Handling**: Reject promises if no response is received within a timeout.

### Step 2: API Wrapper Implementation
Create `src/driver/freenas/websocket/api.js`:
*   Replace `src/driver/freenas/http/api.js`.
*   Map high-level methods to JSON-RPC calls:
    *   `DatasetCreate` -> `pool.dataset.create`
    *   `DatasetDelete` -> `pool.dataset.delete`
    *   `SnapshotCreate` -> `zfs.snapshot.create` (or `pool.dataset.create_snapshot` depending on API)
    *   `SharingNfsCreate` -> `sharing.nfs.create`
    *   `SharingSmbCreate` -> `sharing.smb.create`
    *   `IscsiTargetCreate` -> `iscsi.target.create`
    *   And so on for all used methods.

### Step 3: Driver Update
Update `src/driver/freenas/api.js`:
*   Import the new `WebSocketClient` and `WebSocketApi`.
*   Initialize the WebSocket connection in the constructor or `Probe` method.
*   Update all calls to use the new API wrapper.
*   Remove legacy REST API code (v1/v2 specific logic).

## Phase 3: Advanced Features (Optional/Later)
*   **#433 (Proxy Driver)**: Implement if time permits or as a follow-up.
*   **#464 (HA iSCSI)**: Implement if time permits.

## Verification Plan
1.  **Unit Tests**: Mock the WebSocket server to verify request/response handling and error mapping.
2.  **Integration Tests**: If possible, run against a TrueNAS SCALE 25.04 dev instance (manual verification or CI if available).
