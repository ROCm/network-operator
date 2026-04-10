# upgrade-pollara Examples

Comprehensive examples for the Pollara firmware upgrade skill.

## Basic Usage

### 1. Firmware Upgrade Only
```bash
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt
```
- Upgrades firmware on all hosts in `.hosts.txt`
- Skips hosts already at v1.117.5-a-57 (smart skip)
- Does NOT configure card profile
- Does NOT reboot

### 2. Firmware + Default Profile
```bash
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile default
```
- Upgrades firmware
- Configures `default` card profile
- Auto-reboots if profile update requires it
- Brings all LIFs admin up

### 3. Firmware + pf1_vf1 Profile (SR-IOV)
```bash
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1
```
- Upgrades firmware
- Configures `pf1_vf1` profile
- Creates 1 VF per PF (SR-IOV)
- Brings all LIFs admin up (PFs + VFs)

**Result**: Each NIC will have PF + VF interfaces:
- NIC 1: `enp68s0` (PF) + `enp68s0v0` (VF)
- NIC 2: `enp132s0` (PF) + `enp132s0v0` (VF)

## Advanced Usage

### 4. Full Workflow with Reboot
```bash
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1 --reboot
```
- Upgrades firmware
- Configures card profile (may auto-reboot once)
- Forces additional reboot at the end
- Re-configures profile after final reboot
- Validates everything post-reboot

**Use when**: You want guaranteed clean state after upgrade

### 5. Custom SSH User
```bash
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --ssh-user core --card-profile default
```
- Uses `core` user instead of default `root`
- User must have sudo privileges

**Use when**: Your hosts use non-root default users (e.g., CoreOS, Flatcar)

### 6. Custom Build Server
```bash
upgrade-pollara --version 1.120.0 --hosts-file .hosts.txt --build-server jenkins@build.company.com
```
- Fetches firmware from custom build server
- Useful for CI/CD pipelines or alternative build systems

### 7. Environment Variable for Build Server
```bash
export BUILD_SERVER=jenkins@ci.company.com
upgrade-pollara --version 1.120.0 --hosts-file .hosts.txt
```
- Sets build server via environment variable
- Useful in scripts/automation

## Troubleshooting Workflows

### 8. Check What Failed
```bash
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1

# Check failures
cat ./logs/failed_hosts.txt

# Check detailed logs for a specific host
cat ./logs/10.9.11.246-install.log
```

### 9. Re-run After Partial Failure
```bash
# First run - some hosts might fail
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1

# Fix issues on failed hosts (check network, SSH, permissions, etc.)

# Re-run - smart skip will skip successful hosts, only retry failures
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1
```

**Smart skip**: Already-upgraded hosts are automatically skipped, so re-running is safe and efficient.

### 10. Verify After Upgrade
```bash
# After upgrade, verify on each host
for host in $(cat .hosts.txt); do
  echo "=== $host ==="
  ssh root@$host "nicctl show version firmware"
  ssh root@$host "nicctl show card profile"
  ssh root@$host "nicctl show lif"
done
```

## CI/CD Integration

### 11. Jenkins/GitLab Pipeline
```bash
#!/bin/bash
# upgrade-firmware.sh

set -e

VERSION="${FIRMWARE_VERSION:-1.117.5-a-57}"
HOSTS_FILE="${CLUSTER_HOSTS:-./hosts.txt}"
BUILD_SERVER="${BUILD_SERVER:-jenkins@build.company.com}"
CARD_PROFILE="${CARD_PROFILE:-pf1_vf1}"

echo "Upgrading firmware to $VERSION on cluster..."

upgrade-pollara \
  --version "$VERSION" \
  --hosts-file "$HOSTS_FILE" \
  --build-server "$BUILD_SERVER" \
  --card-profile "$CARD_PROFILE"

# Check results
if [ -f ./logs/failed_hosts.txt ]; then
  echo "ERROR: Some hosts failed upgrade:"
  cat ./logs/failed_hosts.txt
  exit 1
fi

echo "SUCCESS: All hosts upgraded to $VERSION"
```

## Production Workflows

### 12. Rolling Upgrade (Manual)
```bash
# Split hosts into batches
head -5 hosts.txt > batch1.txt
tail -n +6 hosts.txt > batch2.txt

# Upgrade batch 1
upgrade-pollara --version 1.117.5-a-57 --hosts-file batch1.txt --card-profile pf1_vf1 --reboot

# Validate batch 1
for host in $(cat batch1.txt); do
  ssh root@$host "nicctl show version firmware"
done

# If batch 1 OK, proceed with batch 2
upgrade-pollara --version 1.117.5-a-57 --hosts-file batch2.txt --card-profile pf1_vf1 --reboot
```

### 13. Dry Run (Check Versions Only)
```bash
# See which hosts need upgrade
for host in $(cat .hosts.txt); do
  version=$(ssh root@$host "nicctl show version firmware 2>/dev/null | grep -E 'Firmware-(A|B)' | head -n1 | awk '{print \$NF}'")
  echo "$host: $version"
done

# Then run actual upgrade
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1
```

## Expected Outputs

### Successful Upgrade
```
Total: 2 | Succeeded: 2 | Skipped: 0 | Failed: 0
```

### Smart Skip (All Already Upgraded)
```
All hosts at 1.117.5-a-57
```

### Partial Failure
```
Total: 5 | Succeeded: 3 | Skipped: 1 | Failed: 1
Failed:
10.9.11.247
```

## Common Issues

### SSH Connection Failures
```bash
# Verify SSH access
for host in $(cat .hosts.txt); do
  ssh -o ConnectTimeout=10 root@$host "echo $host: OK"
done

# Setup passwordless SSH if needed
ssh-copy-id root@<host>
```

### Bundle Not Found
```bash
# Verify build server access
ssh <user>@sw-dev3.pensando.io "ls /vol/builds/hourly/1.117.5-a-57/rudra-bundle/release-artifacts/pulsar/salina/"

# Or use custom build server
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --build-server myuser@alt-server.com
```

### Profile Configuration Failed
```bash
# Check if profile update output required reboot
cat ./logs/<host>-install.log | grep -i "reboot"

# If needed, add --reboot flag
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1 --reboot
```
