---
name: upgrade-pollara
description: Upgrade Pollara (AINIC) firmware, drivers, and optionally configure card profiles on test/dev hosts in parallel.
argumentHint: --version <VERSION> --hosts-file <FILE> [--card-profile <PROFILE>] [--ssh-user <USER>] [--reboot] [--build-server <SERVER>]
userInvocable: true
category: infrastructure
tags: [firmware, ainic, pollara, upgrade, parallel, automation, hardware]
---

# Pollara Firmware Upgrade

Automates parallel firmware/driver upgrades for Pollara (AINIC) on test/dev hosts.

**What it does:**
1. Validates SSH access and checks current versions (smart skip)
2. Fetches bundle from internal build server
3. Installs kernel modules (DKMS) + host libraries + firmware in parallel
4. Optional: Configures card profile (default/pf1_vf1) with SR-IOV
5. Optional: Reboots and validates

## Prerequisites

**Passwordless SSH required:**
```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
ssh-copy-id root@<each-host>
ssh-copy-id <user>@sw-dev3.pensando.io
```

## Parameters

**Required:**
- `--version`: Firmware version (e.g., `1.117.5-a-57`)
- `--hosts-file`: Path to hosts file (one IP/hostname per line)

**Optional:**
- `--card-profile <profile>`: Card profile to configure (`default`, `pf1_vf1`, etc.)
- `--reboot`: Reboot after installation
- `--ssh-user`: SSH user (default: `root`)
- `--build-server`: Build server (default: `<user>@sw-dev3.pensando.io`)

## Execution

### Stage 0: Setup
```bash
mkdir -p ./logs
rm -f ./logs/skipped_hosts.txt ./logs/failed_hosts.txt
TOTAL_HOSTS=$(grep -v '^#' <HOSTS_FILE> | grep -v '^$' | wc -l)
```

### Stage 1: Validate SSH
```bash
while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  (ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no <SSH_USER>@$host "echo ok" || echo "$host" >> ./logs/ssh_failed.txt) &
done < <HOSTS_FILE>
wait
```
Abort if `./logs/ssh_failed.txt` exists.

### Stage 2: Check Versions
```bash
while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  (
    current_version=$(ssh <SSH_USER>@$host "nicctl show version firmware 2>/dev/null | grep -E 'Firmware-(A|B)' | head -n1 | awk '{print \$NF}'" 2>/dev/null)
    [ "$current_version" = "<VERSION>" ] && echo "$host" >> ./logs/skipped_hosts.txt
  ) &
done < <HOSTS_FILE>
wait

SKIPPED_COUNT=$(wc -l < ./logs/skipped_hosts.txt 2>/dev/null || echo 0)
[ $SKIPPED_COUNT -eq $TOTAL_HOSTS ] && echo "All hosts at <VERSION>" && exit 0
```

### Stage 3: Fetch Bundle
```bash
VERSION=<VERSION>
BUNDLE_NAME="ainic_bundle_${VERSION}.tar.gz"
BUILD_SERVER="${BUILD_SERVER:-<user>@sw-dev3.pensando.io}"
INSTALL_DIR="/tmp/ainic-install-$(date +%s)"
mkdir -p "$INSTALL_DIR"
echo "$INSTALL_DIR" > /tmp/ainic-current-install-dir.txt

scp -o StrictHostKeyChecking=no -o ConnectTimeout=30 \
  "${BUILD_SERVER}:/vol/builds/hourly/${VERSION}/rudra-bundle/release-artifacts/pulsar/salina/${BUNDLE_NAME}" \
  "${INSTALL_DIR}/${BUNDLE_NAME}"

cd "$INSTALL_DIR" && tar -tzf ${BUNDLE_NAME} | grep -q "ainic_bundle_${VERSION}/firmware/"
```

### Stage 4: Install Firmware (Parallel)
```bash
INSTALL_DIR=$(cat /tmp/ainic-current-install-dir.txt)
BUNDLE_PATH="${INSTALL_DIR}/${BUNDLE_NAME}"

while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  grep -q "^${host}$" ./logs/skipped_hosts.txt 2>/dev/null && continue

  (
    echo "[${host}] Starting..."

    # Copy & extract
    scp -o ConnectTimeout=30 "${BUNDLE_PATH}" <SSH_USER>@${host}:/tmp/${BUNDLE_NAME} 2>&1 | tee -a ./logs/${host}-install.log || { echo "$host" >> ./logs/failed_hosts.txt; exit 1; }
    ssh <SSH_USER>@${host} "cd /tmp && tar -xzf ${BUNDLE_NAME} && cd ainic_bundle_${VERSION} && tar -xzf host_sw_pkg.tar.gz" 2>&1 | tee -a ./logs/${host}-install.log || { echo "$host" >> ./logs/failed_hosts.txt; exit 1; }

    # Install drivers + libraries
    ssh <SSH_USER>@${host} "cd /tmp/ainic_bundle_${VERSION}/host_sw_pkg && sudo ./install.sh -y" 2>&1 | tee -a ./logs/${host}-install.log || { echo "$host" >> ./logs/failed_hosts.txt; exit 1; }

    # Flash firmware
    ssh <SSH_USER>@${host} "sudo nicctl update firmware --image /tmp/ainic_bundle_${VERSION}/firmware/ainic_fw_salina.tar --all" 2>&1 | tee -a ./logs/${host}-install.log || { echo "$host" >> ./logs/failed_hosts.txt; exit 1; }

    # Reset & validate
    ssh <SSH_USER>@${host} "sudo nicctl reset card --all" 2>&1 | tee -a ./logs/${host}-install.log || { echo "$host" >> ./logs/failed_hosts.txt; exit 1; }
    sleep 90

    installed_version=$(ssh <SSH_USER>@${host} "nicctl show version firmware 2>/dev/null | grep -E 'Firmware-(A|B)' | head -n1 | awk '{print \$NF}'" 2>/dev/null)
    [ "$installed_version" != "${VERSION}" ] && echo "$host" >> ./logs/failed_hosts.txt && exit 1

    echo "[${host}] Firmware installed"
  ) &
done < <HOSTS_FILE>
wait
```

### Stage 4.5: Configure Card Profile (Optional)
If `--card-profile` is specified:
```bash
CARD_PROFILE=<CARD_PROFILE>

while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  grep -q "^${host}$" ./logs/skipped_hosts.txt 2>/dev/null && continue
  grep -q "^${host}$" ./logs/failed_hosts.txt 2>/dev/null && continue

  (
    echo "[${host}] Configuring card profile: ${CARD_PROFILE}" | tee -a ./logs/${host}-install.log

    # Check current profile (smart skip)
    CURRENT_PROFILE=$(ssh <SSH_USER>@${host} "nicctl show card profile 2>/dev/null | grep 'Profile name' | head -n1 | awk -F': ' '{print \$2}'" 2>/dev/null | tr -d '[:space:]')

    if [ "${CURRENT_PROFILE}" = "${CARD_PROFILE}" ]; then
      echo "[${host}] Already on profile ${CARD_PROFILE}, skipping update" | tee -a ./logs/${host}-install.log
    else
      echo "[${host}] Current profile: ${CURRENT_PROFILE}, updating to ${CARD_PROFILE}" | tee -a ./logs/${host}-install.log

      # Update profile
      OUTPUT=$(ssh <SSH_USER>@${host} "sudo nicctl update card profile -i /tmp/ainic_bundle_${VERSION}/firmware/ainic_fw_salina.tar -p ${CARD_PROFILE}" 2>&1)
      echo "$OUTPUT" | tee -a ./logs/${host}-install.log

      # Check if reboot required
      if echo "$OUTPUT" | grep -q "warm reboot MUST be done"; then
        echo "[${host}] Profile update requires reboot, rebooting..." | tee -a ./logs/${host}-install.log
        ssh <SSH_USER>@${host} "sudo systemctl reboot" &>/dev/null &
        sleep 30

        # Wait for recovery
        echo "[${host}] Waiting for host to come back up..." | tee -a ./logs/${host}-install.log
        for i in {1..30}; do
          ssh -o ConnectTimeout=10 <SSH_USER>@${host} "echo ok" &>/dev/null && { echo "[${host}] Host is back online" | tee -a ./logs/${host}-install.log; break; }
          sleep 10
        done
      fi
    fi

    # Verify profile applied
    echo "[${host}] Verifying profile..." | tee -a ./logs/${host}-install.log
    if ssh <SSH_USER>@${host} "nicctl show card profile | grep -q 'Profile name.*: ${CARD_PROFILE}'"; then
      echo "[${host}] Profile verified: ${CARD_PROFILE}" | tee -a ./logs/${host}-install.log
    else
      echo "[${host}] Profile verification failed" | tee -a ./logs/${host}-install.log
      echo "$host" >> ./logs/failed_hosts.txt
      exit 1
    fi

    # If pf1_vf1: Enable SR-IOV VFs
    if [ "${CARD_PROFILE}" = "pf1_vf1" ]; then
      echo "[${host}] Enabling SR-IOV VFs..." | tee -a ./logs/${host}-install.log
      PCI_ADDRS=$(ssh <SSH_USER>@${host} "nicctl show card -j | jq -r '.nic[].eth_bdf'" 2>/dev/null)
      for pci in $PCI_ADDRS; do
        echo "[${host}] Enabling VF for PCI ${pci}" | tee -a ./logs/${host}-install.log
        ssh <SSH_USER>@${host} "echo 1 | sudo tee /sys/bus/pci/devices/${pci}/sriov_numvfs" >/dev/null 2>&1
      done
    fi

    # Bring all LIFs admin up
    echo "[${host}] Bringing LIFs up..." | tee -a ./logs/${host}-install.log
    LIF_NAMES=$(ssh <SSH_USER>@${host} "nicctl show lif -j | jq -r '.nic[].lif[].status.name'" 2>/dev/null)
    for lif in $LIF_NAMES; do
      ssh <SSH_USER>@${host} "sudo ip link set $lif up" 2>/dev/null && echo "[${host}]   ${lif}: up" | tee -a ./logs/${host}-install.log
    done

    echo "[${host}] Profile configured: ${CARD_PROFILE}" | tee -a ./logs/${host}-install.log
  ) &
done < <HOSTS_FILE>
wait
```

### Stage 5: Summary
```bash
SKIPPED=$(wc -l < ./logs/skipped_hosts.txt 2>/dev/null || echo 0)
FAILED=$(wc -l < ./logs/failed_hosts.txt 2>/dev/null || echo 0)
SUCCEEDED=$((TOTAL_HOSTS - SKIPPED - FAILED))

echo "Total: $TOTAL_HOSTS | Succeeded: $SUCCEEDED | Skipped: $SKIPPED | Failed: $FAILED"
[ $FAILED -gt 0 ] && echo "Failed:" && cat ./logs/failed_hosts.txt
```

### Stage 6: Reboot (Optional)
If `--reboot` flag specified:
```bash
# Reboot all
while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  ssh <SSH_USER>@${host} "sudo systemctl reboot" &>/dev/null &
done < <HOSTS_FILE>
sleep 30

# Wait for recovery
while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  for i in {1..30}; do
    ssh -o ConnectTimeout=10 <SSH_USER>@${host} "echo ok" &>/dev/null && break
    sleep 10
  done
done < <HOSTS_FILE>

# Validate firmware
while IFS= read -r host; do
  [[ -z "$host" || "$host" =~ ^# ]] && continue
  fw=$(ssh <SSH_USER>@${host} "nicctl show version firmware 2>/dev/null | grep -E 'Firmware-(A|B)' | head -n1 | awk '{print \$NF}'" 2>/dev/null)
  [ "$fw" = "<VERSION>" ] && echo "[${host}] OK" || echo "[${host}] FAILED"
done < <HOSTS_FILE>
```

### Stage 6.5: Re-configure Profile (Optional)
If `--reboot` AND `--card-profile` were specified, re-run Stage 4.5 logic after reboot.

### Stage 7: Cleanup
```bash
INSTALL_DIR=$(cat /tmp/ainic-current-install-dir.txt)
rm -rf "$INSTALL_DIR" /tmp/ainic-current-install-dir.txt
```

## Examples

```bash
# Firmware only
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt

# Firmware + pf1_vf1 profile (SR-IOV)
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1

# Full workflow with reboot
upgrade-pollara --version 1.117.5-a-57 --hosts-file .hosts.txt --card-profile pf1_vf1 --reboot
```

See [reference/examples.md](reference/examples.md) for more detailed examples and troubleshooting.

## Notes for Claude

- Firmware can be in slot A or B: `grep -E 'Firmware-(A|B)'`
- Card profile JSON fields: `.nic[].eth_bdf` for PCI, `.nic[].lif[].status.name` for LIFs
- Profile update smart skip: Check current profile first, skip update if already on target profile, but still enable VFs and bring up LIFs
- Profile update may auto-reboot if output contains "warm reboot MUST be done"
- SR-IOV only for `pf1_vf1` profile
- All output (firmware + profile) must be logged to `./logs/${host}-install.log` using `tee -a`
- Replace `<VERSION>`, `<HOSTS_FILE>`, `<SSH_USER>`, `<CARD_PROFILE>` with actual values
- Be verbose - report progress after each stage
