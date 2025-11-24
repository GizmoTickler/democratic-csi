const { sleep, stringify } = require("../../../utils/general");
const { Zetabyte } = require("../../../utils/zfs");

// Registry namespace for cached objects
const __REGISTRY_NS__ = "TrueNASWebSocketApi";

/**
 * TrueNAS SCALE 25.04+ JSON-RPC API Wrapper
 *
 * This class provides a clean interface to TrueNAS SCALE 25.04+ API
 * using WebSocket JSON-RPC 2.0 protocol only.
 *
 * All legacy support for FreeNAS and old TrueNAS versions has been removed.
 *
 * API Documentation: https://api.truenas.com/v25.04.2/
 */
class Api {
  constructor(client, cache, options = {}) {
    this.client = client;
    this.cache = cache;
    this.options = options;
    this.ctx = options.ctx;
  }

  async getHttpClient() {
    return this.client;
  }

  /**
   * Get ZFS helper utility (for local operations only)
   */
  async getZetabyte() {
    return this.ctx.registry.get(`${__REGISTRY_NS__}:zb`, () => {
      return new Zetabyte({
        executor: {
          spawn: function () {
            throw new Error(
              "Cannot use ZFS executor directly - must use WebSocket API"
            );
          },
        },
      });
    });
  }

  /**
   * Call a JSON-RPC method on the TrueNAS API
   */
  async call(method, params = []) {
    const client = await this.getHttpClient();
    return await client.call(method, params);
  }

  /**
   * Query resources with optional filters
   * @param {string} method - The query method (e.g., "pool.dataset.query")
   * @param {array} filters - Query filters
   * @param {object} options - Query options (limit, offset, etc.)
   */
  async query(method, filters = [], options = {}) {
    return await this.call(method, [filters, options]);
  }

  /**
   * Find a single resource by properties
   * @param {string} method - The query method
   * @param {object|function} match - Properties to match or matcher function
   */
  async findResourceByProperties(method, match) {
    if (!match) {
      return null;
    }

    if (typeof match === "object" && Object.keys(match).length < 1) {
      return null;
    }

    const results = await this.query(method);

    if (!Array.isArray(results)) {
      return null;
    }

    return results.find((item) => {
      if (typeof match === "function") {
        return match(item);
      }

      for (let property in match) {
        if (match[property] !== item[property]) {
          return false;
        }
      }
      return true;
    });
  }

  // ============================================================================
  // SYSTEM VERSION & INFO
  // ============================================================================

  /**
   * Get TrueNAS system version info
   * This is cached to avoid repeated calls
   */
  async getSystemVersion() {
    const cacheKey = "truenas:system_version";
    let cached = this.cache.get(cacheKey);

    if (cached) {
      return cached;
    }

    const version = await this.call("system.version");
    this.cache.set(cacheKey, version, 300); // Cache for 5 minutes
    return version;
  }

  /**
   * Get system info (hostname, version, etc.)
   */
  async getSystemInfo() {
    return await this.call("system.info");
  }

  // ============================================================================
  // POOL & DATASET OPERATIONS
  // ============================================================================

  /**
   * Create a new dataset
   * @param {string} datasetName - Full dataset name (e.g., "pool/dataset")
   * @param {object} data - Dataset properties
   */
  async DatasetCreate(datasetName, data = {}) {
    try {
      const params = {
        name: datasetName,
        ...data,
      };

      await this.call("pool.dataset.create", [params]);
    } catch (error) {
      // Ignore "already exists" errors
      if (error.message && error.message.includes("already exists")) {
        return;
      }
      throw error;
    }
  }

  /**
   * Delete a dataset
   * @param {string} datasetName - Full dataset name
   * @param {object} data - Delete options (recursive, force, etc.)
   */
  async DatasetDelete(datasetName, data = {}) {
    try {
      await this.call("pool.dataset.delete", [datasetName, data]);
    } catch (error) {
      // Ignore "does not exist" errors
      if (error.message && error.message.includes("does not exist")) {
        return;
      }
      throw error;
    }
  }

  /**
   * Update dataset properties
   * @param {string} datasetName - Full dataset name
   * @param {object} properties - Properties to update
   */
  async DatasetSet(datasetName, properties) {
    const params = {
      ...this.getSystemProperties(properties),
      user_properties_update: this.getPropertiesKeyValueArray(
        this.getUserProperties(properties)
      ),
    };

    await this.call("pool.dataset.update", [datasetName, params]);
  }

  /**
   * Inherit a dataset property from parent
   * @param {string} datasetName - Full dataset name
   * @param {string} property - Property name to inherit
   */
  async DatasetInherit(datasetName, property) {
    const isUserProperty = this.getIsUserProperty(property);
    let params = {};

    if (isUserProperty) {
      params.user_properties_update = [{ key: property, remove: true }];
    } else {
      params[property] = { source: "INHERIT" };
    }

    await this.call("pool.dataset.update", [datasetName, params]);
  }

  /**
   * Get dataset properties
   * @param {string} datasetName - Full dataset name
   * @param {array} properties - Specific properties to retrieve (optional)
   */
  async DatasetGet(datasetName, properties = []) {
    const filters = [["id", "=", datasetName]];
    const options = {};

    if (properties && properties.length > 0) {
      options.select = properties;
    }

    const results = await this.query("pool.dataset.query", filters, options);

    if (!results || results.length === 0) {
      throw new Error(`Dataset not found: ${datasetName}`);
    }

    return results[0];
  }

  /**
   * Destroy snapshots matching criteria
   * @param {string} datasetName - Dataset name
   * @param {object} data - Snapshot destruction criteria
   */
  async DatasetDestroySnapshots(datasetName, data = {}) {
    await this.call("pool.dataset.destroy_snapshots", [datasetName, data]);
  }

  // ============================================================================
  // SNAPSHOT OPERATIONS
  // ============================================================================

  /**
   * Create a snapshot
   * @param {string} snapshotName - Full snapshot name (dataset@snapshot)
   * @param {object} data - Snapshot options
   */
  async SnapshotCreate(snapshotName, data = {}) {
    const parts = snapshotName.split("@");
    if (parts.length !== 2) {
      throw new Error(`Invalid snapshot name: ${snapshotName}`);
    }

    const params = {
      dataset: parts[0],
      name: parts[1],
      ...data,
    };

    await this.call("zfs.snapshot.create", [params]);
  }

  /**
   * Delete a snapshot
   * @param {string} snapshotName - Full snapshot name (dataset@snapshot)
   * @param {object} data - Delete options
   */
  async SnapshotDelete(snapshotName, data = {}) {
    try {
      await this.call("zfs.snapshot.delete", [snapshotName, data]);
    } catch (error) {
      // Ignore "does not exist" errors
      if (error.message && error.message.includes("does not exist")) {
        return;
      }
      throw error;
    }
  }

  /**
   * Update snapshot properties
   * @param {string} snapshotName - Full snapshot name
   * @param {object} properties - Properties to update
   */
  async SnapshotSet(snapshotName, properties) {
    const params = {
      ...this.getSystemProperties(properties),
      user_properties_update: this.getPropertiesKeyValueArray(
        this.getUserProperties(properties)
      ),
    };

    await this.call("zfs.snapshot.update", [snapshotName, params]);
  }

  /**
   * Get snapshot properties
   * @param {string} snapshotName - Full snapshot name
   * @param {array} properties - Specific properties to retrieve (optional)
   */
  async SnapshotGet(snapshotName, properties = []) {
    const filters = [["id", "=", snapshotName]];
    const options = {};

    if (properties && properties.length > 0) {
      options.select = properties;
    }

    const results = await this.query("zfs.snapshot.query", filters, options);

    if (!results || results.length === 0) {
      throw new Error(`Snapshot not found: ${snapshotName}`);
    }

    return results[0];
  }

  // ============================================================================
  // CLONE OPERATIONS
  // ============================================================================

  /**
   * Clone a snapshot to create a new dataset
   * @param {string} snapshotName - Source snapshot name
   * @param {string} datasetName - Target dataset name
   * @param {object} data - Clone options
   */
  async CloneCreate(snapshotName, datasetName, data = {}) {
    const params = {
      snapshot: snapshotName,
      dataset_dst: datasetName,
      ...data,
    };

    await this.call("zfs.snapshot.clone", [params]);
  }

  // ============================================================================
  // REPLICATION
  // ============================================================================

  /**
   * Run a one-time replication task
   * @param {object} data - Replication configuration
   */
  async ReplicationRunOnetime(data) {
    const jobId = await this.call("replication.run_onetime", [data]);
    return jobId;
  }

  // ============================================================================
  // JOB MANAGEMENT
  // ============================================================================

  /**
   * Wait for a job to complete
   * @param {number} jobId - Job ID to wait for
   * @param {number} timeout - Timeout in seconds (0 = no timeout)
   * @param {number} checkInterval - Interval between checks in milliseconds
   */
  async CoreWaitForJob(jobId, timeout = 0, checkInterval = 3000) {
    const startTime = Date.now();

    while (true) {
      const job = await this.call("core.get_jobs", [[["id", "=", jobId]]]);

      if (!job || job.length === 0) {
        throw new Error(`Job ${jobId} not found`);
      }

      const jobInfo = job[0];

      // Check job state
      if (jobInfo.state === "SUCCESS") {
        return jobInfo;
      } else if (jobInfo.state === "FAILED") {
        throw new Error(`Job ${jobId} failed: ${jobInfo.error || "Unknown error"}`);
      } else if (jobInfo.state === "ABORTED") {
        throw new Error(`Job ${jobId} was aborted`);
      }

      // Check timeout
      if (timeout > 0) {
        const elapsed = (Date.now() - startTime) / 1000;
        if (elapsed >= timeout) {
          throw new Error(`Job ${jobId} timed out after ${timeout} seconds`);
        }
      }

      // Wait before checking again
      await sleep(checkInterval);
    }
  }

  /**
   * Get jobs matching filters
   * @param {array} filters - Job filters
   */
  async CoreGetJobs(filters = []) {
    return await this.call("core.get_jobs", [filters]);
  }

  // ============================================================================
  // FILESYSTEM PERMISSIONS
  // ============================================================================

  /**
   * Set filesystem permissions
   * @param {object} data - Permission data (path, mode, uid, gid, options)
   */
  async FilesystemSetperm(data) {
    const jobId = await this.call("filesystem.setperm", [data]);
    return jobId;
  }

  /**
   * Change filesystem ownership
   * @param {object} data - Ownership data (path, uid, gid, options)
   */
  async FilesystemChown(data) {
    await this.call("filesystem.chown", [data]);
  }

  // ============================================================================
  // PROPERTY HELPERS
  // ============================================================================

  /**
   * Check if a property is a user property
   */
  getIsUserProperty(property) {
    return property.includes(":");
  }

  /**
   * Split properties into system and user properties
   */
  getSystemProperties(properties) {
    const systemProps = {};
    for (const [key, value] of Object.entries(properties)) {
      if (!this.getIsUserProperty(key)) {
        systemProps[key] = value;
      }
    }
    return systemProps;
  }

  /**
   * Get only user properties
   */
  getUserProperties(properties) {
    const userProps = {};
    for (const [key, value] of Object.entries(properties)) {
      if (this.getIsUserProperty(key)) {
        userProps[key] = value;
      }
    }
    return userProps;
  }

  /**
   * Convert properties object to key-value array format
   */
  getPropertiesKeyValueArray(properties) {
    return Object.entries(properties).map(([key, value]) => ({
      key,
      value: String(value),
    }));
  }
}

module.exports.Api = Api;
