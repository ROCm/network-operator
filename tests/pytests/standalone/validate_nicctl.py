import subprocess
import sys

def run(cmd):
    r = subprocess.run(cmd, shell=True, stdout=subprocess.PIPE,
                       stderr=subprocess.PIPE, text=True)
    return r.returncode, r.stdout.strip(), r.stderr.strip()

def get_version(output, key):
    for line in output.splitlines():
        if line.strip().startswith(key):
            return line.split(":", 1)[1].strip()
    return None

if len(sys.argv) != 4:
    print("Usage: python validate_nicctl.py <fw_version> <nic_exporter_version> <metrics_port>")
    sys.exit(1)

fw_expected = sys.argv[1]
exporter_expected = sys.argv[2]
metrics_port = sys.argv[3]
fail = False

# -------------------------------------------------
# Firmware & nicctl versions
# -------------------------------------------------
_, fw_out, _ = run("nicctl show version firmware")
_, host_out, _ = run("nicctl show version host-software")

fw_version = get_version(fw_out, "Firmware-A")
host_version = get_version(host_out, "nicctl")

print("Firmware-A :", fw_version, "PASS" if fw_version == fw_expected else "FAIL")
print("nicctl     :", host_version, "PASS" if host_version == fw_expected else "FAIL")

if fw_version != fw_expected or host_version != fw_expected:
    fail = True

# -------------------------------------------------
# Basic command checks
# -------------------------------------------------
for cmd in ["nicctl show lif", "nicctl show card"]:
    rc, _, err = run(cmd)
    if rc == 0:
        print(f"{cmd} : PASS")
    else:
        print(f"{cmd} : FAIL")
        print(err)
        fail = True

# -------------------------------------------------
# NIC exporter version
# -------------------------------------------------
rc, log_out, _ = run("grep Version /var/log/amd-nic-metrics-exporter.log")
if rc == 0 and exporter_expected in log_out:
    print("NIC exporter :", exporter_expected, "PASS")
else:
    print("NIC exporter : FAIL")
    fail = True

# -------------------------------------------------
# RDMA queue-pair sum
# -------------------------------------------------
_, rdma_out, _ = run("nicctl show rdma queue-pair --summary")

qp_sum = 0
for line in rdma_out.splitlines():
    if "Number of queue pairs" in line:
        qp_sum += int(line.split(":")[1].strip())

expected_qp_id = qp_sum + 2

print("Total queue pairs :", qp_sum)
print("Expected qp_id   :", expected_qp_id)

# -------------------------------------------------
# Fetch metrics
# -------------------------------------------------
run(f"curl -s http://localhost:{metrics_port}/metrics > /tmp/test")

# -------------------------------------------------
# EXACT metric count check
# cat test | grep -c 'amd_qp_rq_rsp_rx_num_packet'
# -------------------------------------------------
_, count_out, _ = run("grep -c amd_qp_rq_rsp_rx_num_packet /tmp/test")
metric_count = int(count_out)

print("amd_qp_rq_rsp_rx_num_packet count :", metric_count)

if metric_count == expected_qp_id:
    print("Metric count validation : PASS")
else:
    print(f"Metric count validation : FAIL (expected {expected_qp_id})")
    fail = True

# -------------------------------------------------
# EXACT qp_id value sum
# cat test | grep num_packet | grep 4002
# -------------------------------------------------
cmd = f"grep num_packet /tmp/test | grep {expected_qp_id}"
_, qp_lines, _ = run(cmd)

value_sum = 0
for line in qp_lines.splitlines():
    value_sum += int(line.split()[-1])

print(f"Metric value sum for qp_id {expected_qp_id} :", value_sum)

if value_sum > 1000:
    print("Metric value validation : PASS")
else:
    print("Metric value validation : FAIL (sum <= 1000)")
    fail = True

# -------------------------------------------------
# Exit
# -------------------------------------------------
sys.exit(1 if fail else 0)
