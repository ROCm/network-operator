# Grafana Dashboard

The [**AINIC System Grafana Dashboard**](https://github.com/ROCm/device-metrics-exporter/tree/main/grafana) relies on the `CLUSTER_NAME` label to query and display metrics.
If this label is not present, the dashboard will render **blank**.

There are **two ways** to resolve this issue.


## Option 1: Remove the `CLUSTER_NAME` Dependency from the Dashboard

This option updates the dashboard configuration directly to remove the dependency on the `CLUSTER_NAME` label.

1. Open the **AINIC System Dashboard** in Grafana.
2. Click **Edit** → **Settings** → **Variables**.
3. Select **`g_hostname`** (third variable in the list).
4. Under **Query Options → Label Filters**, remove `$g_cluster_name`:

   * Click the **✕ (cross)** icon next to `$g_cluster_name`.
5. Save the dashboard.

The dashboard will start displaying metrics without requiring the `CLUSTER_NAME` label.


## Option 2: Add the `CLUSTER_NAME` Label (Recommended for Multi-Cluster Setups)

This option preserves the dashboard as-is and explicitly provides the required `CLUSTER_NAME` label via configuration.
This is the **recommended approach** when managing multiple clusters.

### 1. Create a ConfigMap with the `CLUSTER_NAME` label

Create a file named `configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nic-exporter-custom-config
  namespace: kube-amd-network
data:
  config.json: |
    {
      "NICConfig": {
        "CustomLabels": {
          "CLUSTER_NAME": "SantaClaraLab-PF"
        }
      }
    }
```

> Replace `SantaClaraLab-PF` with your actual cluster name.

### 2. Update `networkconfig.yaml`

Ensure the following configuration is present under the exporter section:

```yaml
config:
  name: nic-exporter-custom-config
```

### 3. Apply the configuration

```bash
kubectl apply -f configmap.yaml
kubectl apply -f networkconfig.yaml
```

Once the configuration is applied, the dashboard will correctly display metrics using the configured `CLUSTER_NAME`.
