# Status Report

## Current State
The current `freenas` driver in `democratic-csi` relies on the TrueNAS REST API (v1 and v2).
*   **HTTP Client**: Uses `axios` via `src/utils/general.js`.
*   **API Implementation**: `src/driver/freenas/http/api.js` implements methods like `DatasetCreate`, `SnapshotCreate`, etc., making HTTP requests.
*   **Driver Logic**: `src/driver/freenas/api.js` uses the HTTP API client to perform CSI operations.

## Target State
The goal is to migrate to the TrueNAS SCALE 25.04 API, which uses **JSON-RPC 2.0 over WebSocket**.
*   **Protocol**: WebSocket (ws/wss).
*   **Format**: JSON-RPC 2.0.
*   **Authentication**: Auth token or username/password via WebSocket auth method.
*   **Legacy Support**: Explicitly dropping support for legacy REST APIs.

## Pull Request Analysis
The following open PRs were reviewed for integration:

| PR # | Title | Scope | Status/Plan |
| :--- | :--- | :--- | :--- |
| **#523** | fix(docs): update talos docs | Documentation | Integrate (Low complexity). |
| **#517** | Allowing ZFS datasetParentName to be defined in the k8s storageclass | Feature (ZFS) | Integrate. Useful for flexibility. |
| **#501** | Disable TRIM | Feature (MKFS) | Integrate. Improves performance on some backends. |
| **#474** | Add context logs to Node, ZFS, Freenas, synology, objectivefs, common client drivers | Refactor (Logging) | **High Priority**. Essential for debugging the new driver. Integrate first. |
| **#471** | Change driver interface: add call context arg | Refactor (Core) | **High Priority**. Dependency for #474. Integrate first. |
| **#469** | Add explicit driver cleanup | Refactor (Core) | **High Priority**. Critical for WebSocket connection management (closing sockets). Integrate first. |
| **#466** | Add topology support to proxy driver | Feature (Proxy) | Advanced. Dependency on #433. Postpone until core migration is done. |
| **#464** | Add iSCSI targets/LUNs through Pacemaker clusters | Feature (iSCSI) | Advanced. Complex changes to ZFS generic driver. Postpone or integrate carefully. |
| **#458** | docs: updating docs and example with synology chap authentication | Documentation | Integrate (Low complexity). |
| **#433** | Add proxy driver | Feature (Core) | Major feature. Postpone until core migration is done. |
| **#218** | ZFS and sudo permissions related improvements | Feature (ZFS) | Integrate. Improves non-root usage. |

## Migration Risks & Challenges
1.  **WebSocket State Management**: Unlike stateless REST calls, WebSockets require managing connection state, reconnections, and request matching (id correlation).
2.  **Event Loop Blocking**: Extensive JSON parsing or synchronous operations on the WebSocket stream could block the Node.js event loop.
3.  **Error Handling**: Mapping JSON-RPC errors to gRPC errors needs careful implementation.
4.  **Testing**: Verifying the new driver requires a running TrueNAS SCALE 25.04 instance or a mock WebSocket server.
