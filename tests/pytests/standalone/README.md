# Standalone Validation Scripts

This directory contains standalone validation scripts for AMD network components that run directly on host systems without requiring Kubernetes.

## validate_nicctl.py

Validates AMD NIC firmware, nicctl tool, and metrics exporter functionality on a host system.

### Purpose

This script performs comprehensive validation of:
- Firmware version consistency
- nicctl command-line tool functionality
- NIC metrics exporter operation
- RDMA queue pair metrics accuracy

### Prerequisites

- AMD NIC hardware installed
- `nicctl` command-line tool installed and in PATH
- `amd-nic-metrics-exporter` service running on specified metrics port
- `curl` command available
- Metrics exporter log at `/var/log/amd-nic-metrics-exporter.log`

### Usage

```bash
python3 validate_nicctl.py <fw_version> <nic_exporter_version> <metrics_port>
```

**Arguments:**
- `fw_version`: Expected firmware version (e.g., "1.117.5-a-57")
- `nic_exporter_version`: Expected NIC exporter version string (e.g., "nic-v1.0.0-14")
- `metrics_port`: Metrics exporter port number (e.g., "5002")

**Example:**
```bash
python3 validate_nicctl.py 1.117.5-a-57 nic-v1.0.0-14 5002
```

### Validation Checks

1. **Firmware Version Check**
   - Verifies `nicctl show version firmware` returns expected Firmware-A version
   - Validates firmware matches expected version

2. **Host Software Version Check**
   - Verifies `nicctl show version host-software` returns expected nicctl version
   - Validates nicctl tool version matches firmware version

3. **Basic Command Functionality**
   - Tests `nicctl show lif` command executes successfully
   - Tests `nicctl show card` command executes successfully

4. **NIC Exporter Version Check**
   - Parses `/var/log/amd-nic-metrics-exporter.log` for version string
   - Validates exporter version matches expected version

5. **RDMA Queue Pair Validation**
   - Queries `nicctl show rdma queue-pair --summary`
   - Calculates total queue pairs across all interfaces
   - Determines expected queue pair ID (total QPs + 2)

6. **Metrics Endpoint Validation**
   - Fetches metrics from `http://localhost:<metrics_port>/metrics`
   - Counts occurrences of `amd_qp_rq_rsp_rx_num_packet` metric
   - Validates metric count matches expected queue pair ID

7. **Metric Value Validation**
   - Sums metric values for the expected queue pair ID
   - Validates sum exceeds 1000 (indicates active traffic)

### Exit Codes

- `0`: All validations passed
- `1`: One or more validations failed

### Output Format

Each validation prints a status line:
```
Firmware-A : 1.5.0 PASS
nicctl     : 1.5.0 PASS
nicctl show lif : PASS
nicctl show card : PASS
NIC exporter : v0.0.1-139 PASS
Total queue pairs : 4000
Expected qp_id   : 4002
amd_qp_rq_rsp_rx_num_packet count : 4002
Metric count validation : PASS
Metric value sum for qp_id 4002 : 15234
Metric value validation : PASS
```

### Troubleshooting

**"nicctl: command not found"**
- Ensure nicctl is installed and in your PATH
- Verify AMD NIC drivers are properly installed

**"NIC exporter : FAIL"**
- Check if amd-nic-metrics-exporter service is running
- Verify log file exists at `/var/log/amd-nic-metrics-exporter.log`
- Ensure metrics endpoint is accessible on the specified port

**"Metric count validation : FAIL"**
- RDMA queue pairs may not be properly configured
- Metrics exporter may not be collecting all queue pair data
- Check nicctl RDMA configuration

**"Metric value validation : FAIL (sum <= 1000)"**
- Insufficient traffic through queue pairs
- Run RDMA traffic tests before validation
- Ensure workloads are actively using the NICs

### Integration

This script is designed for:
- Post-installation validation
- CI/CD pipeline integration
- Regression testing after firmware/driver updates
- Pre-deployment health checks

### Related Documentation

- [AMD Network Operator Overview](../../../docs/overview.md)
- [Metrics Exporter Documentation](../../../docs/metrics/)
- [Device Plugin Documentation](../../../docs/device_plugin/)
