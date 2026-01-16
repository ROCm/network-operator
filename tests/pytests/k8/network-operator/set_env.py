#!/usr/bin/env python3
import json
import subprocess
import os
import sys
from pathlib import Path

"""
copy_kubeconfig (fixed)

- Reads env.json and looks up the given environment entry.
- Uses sshpass+scp to copy:
    - /home/<remote_user>/.kube/config -> ~/ .kube/config (in the account running this script)
    - /home/<remote_user>/ca.crt
    - /home/<remote_user>/client.crt
    - /home/<remote_user>/client.key
  into a local certs directory under the current user's home: ~/certs/
- Creates local target dirs if missing.
- Uses universal_newlines for subprocess compatibility across Python3 versions.
- Handles each scp separately and reports missing certs without failing the whole run.
"""

def die(msg, code=1):
    print(f"✗ {msg}")
    sys.exit(code)


def run(cmd, check=True):
    """
    Run subprocess and return CompletedProcess.
    Use universal_newlines=True for broad Python3 compatibility.
    """
    return subprocess.run(cmd, check=check, stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)


def try_scp(cmd, desc):
    """
    Try an scp command. Return True on success, False on failure.
    Prints stdout/stderr for diagnostics.
    """
    try:
        print(f"→ Running: {' '.join(cmd)}")
        res = run(cmd)
        if res.stdout:
            print(res.stdout.strip())
        if res.stderr:
            # print stderr even when empty to help debugging
            print(res.stderr.strip())
        return True
    except subprocess.CalledProcessError as e:
        print(f"scp error copying {desc}: returncode={e.returncode}")
        if e.stdout:
            print("stdout:", e.stdout)
        if e.stderr:
            print("stderr:", e.stderr)
        return False
    except Exception as e:
        print(f"Unexpected error copying {desc}: {e}")
        return False


def copy_kubeconfig(environment):
    # read env.json
    try:
        with open("env.json", "r") as f:
            cfg = json.load(f)
    except Exception as e:
        die(f"Failed to read env.json: {e}")

    envs = cfg.get("environments", {})
    if environment not in envs:
        die(f"Environment '{environment}' not found in env.json")

    env_cfg = envs[environment]
    ip = env_cfg.get("ip")
    remote_user = env_cfg.get("username")
    password = env_cfg.get("password")

    if not ip or not remote_user or password is None:
        die("env.json entry must include 'ip', 'username' and 'password' fields")

    # local targets - use current logged-in user's home
    local_home = Path(os.path.expanduser("~"))
    kube_dir = local_home / ".kube"
    cert_dir = local_home / "certs"

    # remote sources (under /home/<remote_user>/ on remote host)
    remote_kube_config = f"/home/{remote_user}/.kube/config"
    remote_ca = f"/home/{remote_user}/ca.crt"
    remote_client_crt = f"/home/{remote_user}/client.crt"
    remote_client_key = f"/home/{remote_user}/client.key"

    # ensure local dirs exist
    try:
        kube_dir.mkdir(parents=True, exist_ok=True)
        cert_dir.mkdir(parents=True, exist_ok=True)
    except Exception as e:
        die(f"Failed to create local target directories '{kube_dir}' or '{cert_dir}': {e}")

    # scp commands
    scp_config = [
        "sshpass", "-p", password,
        "scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
        f"{remote_user}@{ip}:{remote_kube_config}",
        str(kube_dir / "config")
    ]

    scp_ca = [
        "sshpass", "-p", password,
        "scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
        f"{remote_user}@{ip}:{remote_ca}",
        str(cert_dir / "ca.crt")
    ]

    scp_client_crt = [
        "sshpass", "-p", password,
        "scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
        f"{remote_user}@{ip}:{remote_client_crt}",
        str(cert_dir / "client.crt")
    ]

    scp_client_key = [
        "sshpass", "-p", password,
        "scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
        f"{remote_user}@{ip}:{remote_client_key}",
        str(cert_dir / "client.key")
    ]

    # copy kubeconfig (fatal if fails)
    if not try_scp(scp_config, "kubeconfig"):
        die(f"Failed to copy remote kubeconfig '{remote_kube_config}' from {ip}")

    # copy certs (non-fatal)
    if try_scp(scp_ca, "ca.crt"):
        print("✓ ca.crt copied")
    else:
        print(f"⚠ ca.crt not copied (may not exist at {remote_ca})")

    if try_scp(scp_client_crt, "client.crt"):
        print("✓ client.crt copied")
    else:
        print(f"⚠ client.crt not copied (may not exist at {remote_client_crt})")

    if try_scp(scp_client_key, "client.key"):
        print("✓ client.key copied")
    else:
        print(f"⚠ client.key not copied (may not exist at {remote_client_key})")

    # set permissions
    try:
        cfg_path = kube_dir / "config"
        if cfg_path.exists():
            cfg_path.chmod(0o600)
        client_path = cert_dir / "client.crt"
        key_path = cert_dir / "client.key"
        ca_path = cert_dir / "ca.crt"
        if client_path.exists():
            client_path.chmod(0o600)
        if key_path.exists():
            key_path.chmod(0o600)
        if ca_path.exists():
            ca_path.chmod(0o644)
    except Exception as e:
        print(f"⚠ Failed to set permissions on files: {e}")

    print(f"✓ Successfully copied kubeconfig to {cfg_path}")
    print(f"✓ Certificates (if available) copied into {cert_dir}")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python set_env.py <environment>")
        sys.exit(1)
    copy_kubeconfig(sys.argv[1])
