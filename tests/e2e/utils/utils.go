/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ROCm/network-operator/internal/kmmmodule"
	netattachdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	netclientset "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	log "github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// Constants / Global settings
// -----------------------------------------------------------------------------
const (
	ClusterTypeOpenShift = "openshift"
	ClusterTypeK8s       = "kubernetes"
	HttpServerPort       = "8084"

	defaultRetryTimeout  = 5 * time.Minute
	defaultRetryInterval = 5 * time.Second
)

var (
	kubectl    = "kubectl"
	ainicLabel = map[string]string{"e2e": "true"}

	rocmDs           = "e2e-rocm"
	ErrNotSupported  = errors.New("unsupported resource type")
	ErrNodeIPMissing = errors.New("node internal IP not found")
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------
type UserRequest struct {
	Command string `json:"command"`
}

// -----------------------------------------------------------------------------
// Init
// -----------------------------------------------------------------------------
func init() {
	c, err := exec.LookPath("kubectl")
	if err != nil {
		log.Fatalf("kubectl not found: %v", err)
	}
	kubectl = c
	log.SetReportCaller(true)
}

// -----------------------------------------------------------------------------
// Generic helpers
// -----------------------------------------------------------------------------
func Logger() *log.Logger { return log.StandardLogger() }

// Retry (original signature preserved). Returns nil on first success.
func Retry(f func() error, timeout time.Duration, period time.Duration) error {
	if period <= 0 {
		period = defaultRetryInterval
	}
	if timeout <= 0 {
		timeout = defaultRetryTimeout
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := f(); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout")
		}
		time.Sleep(period)
	}
}

// waitEventually waits for a condition to become true, with a descriptive message for logging
func waitEventually(description string, timeout time.Duration, pollInterval time.Duration, condition func() (bool, error)) {
	log.Infof("Waiting for: %s", description)

	if timeout <= 0 {
		timeout = defaultRetryTimeout
	}
	if pollInterval <= 0 {
		pollInterval = defaultRetryInterval
	}

	deadline := time.Now().Add(timeout)
	for {
		ready, err := condition()
		if err != nil {
			log.Debugf("Error checking condition for '%s': %v", description, err)
		}
		if ready {
			log.Infof("Condition met: %s", description)
			return
		}
		if time.Now().After(deadline) {
			log.Fatalf("timeout waiting for: %s", description)
		}
		time.Sleep(pollInterval)
	}
}

// -----------------------------------------------------------------------------
// Label / resource checks
// -----------------------------------------------------------------------------
func CheckNicLabel(rl v1.ResourceList) bool {
	nic, nicFound := rl["amd.com/nic"]
	vnic, vnicFound := rl["amd.com/vnic"]
	if !nicFound && !vnicFound {
		return false
	}
	if nic.String() == "0" && vnic.String() == "0" {
		return false
	}
	return true
}

// -----------------------------------------------------------------------------
// Helm deployment readiness
// -----------------------------------------------------------------------------
func CheckHelmDeployment(cl *kubernetes.Clientset, ns string, create bool) error {
	deployments := []struct{ ns, name string }{
		{ns: "kube-amd-network", name: "amd-network-operator-network-operator-charts-controller-manager"},
		{ns: "kube-amd-network", name: "amd-network-operator-node-feature-discovery-gc"},
		{ns: "kube-amd-network", name: "amd-network-operator-node-feature-discovery-master"},
	}
	for _, d := range deployments {
		s, err := cl.AppsV1().Deployments(d.ns).Get(context.TODO(), d.name, metav1.GetOptions{})
		if create {
			if err != nil {
				return fmt.Errorf("get deployment %s/%s: %w", d.ns, d.name, err)
			}
			if s.Status.Replicas == 0 || s.Status.ReadyReplicas != s.Status.Replicas {
				return fmt.Errorf("deployment not ready %s/%s status=%+v", d.ns, d.name, s.Status)
			}
		} else {
			if err == nil {
				return fmt.Errorf("deployment %s/%s still exists", d.ns, d.name)
			}
		}
	}
	dsList := []struct{ ns, name string }{
		{ns: "kube-amd-network", name: "amd-network-operator-node-feature-discovery-worker"},
	}
	for _, d := range dsList {
		s, err := cl.AppsV1().DaemonSets(d.ns).Get(context.TODO(), d.name, metav1.GetOptions{})
		if create {
			if err != nil {
				return fmt.Errorf("get daemonset %s/%s: %w", d.ns, d.name, err)
			}
			if s.Status.DesiredNumberScheduled == 0 || s.Status.DesiredNumberScheduled != s.Status.NumberReady {
				return fmt.Errorf("daemonset not ready %s/%s status=%+v", d.ns, d.name, s.Status)
			}
		} else {
			if err == nil {
				return fmt.Errorf("daemonset %s/%s still exists", d.ns, d.name)
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// NetworkAttachmentDefinition
// -----------------------------------------------------------------------------
func AddNetworkAttachmentDefinition(netcl *netclientset.Clientset) error {
	nad := &netattachdefv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "amd-host-device-nad",
			Namespace: v1.NamespaceDefault,
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/resourceName": "amd.com/nic",
			},
		},
		Spec: netattachdefv1.NetworkAttachmentDefinitionSpec{
			Config: `{
                "name":"amd-host-device-nad",
                "cniVersion":"0.3.1",
                "type":"amd-host-device"
            }`,
		},
	}
	_, err := netcl.K8sCniCncfIoV1().NetworkAttachmentDefinitions(v1.NamespaceDefault).
		Create(context.TODO(), nad, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create NAD: %w", err)
	}
	return nil
}

func DeleteNetworkAttachmentDefinition(netcl *netclientset.Clientset) error {
	err := netcl.K8sCniCncfIoV1().NetworkAttachmentDefinitions(v1.NamespaceDefault).
		Delete(context.TODO(), "amd-host-device-nad", metav1.DeleteOptions{})
	return err
}

// -----------------------------------------------------------------------------
// Workload (NIC resource) helpers
// -----------------------------------------------------------------------------
func DeployPodWithNICResource(ctx context.Context, cl *kubernetes.Clientset, res *v1.ResourceRequirements) error {
	log.Print("deploying ds with NIC resource")
	if err := CreateDaemonsetVerify(ctx, cl, v1.NamespaceDefault, rocmDs, initContainerImage, ainicLabel, res); err != nil {
		return fmt.Errorf("create ds: %w", err)
	}
	if err := Retry(func() error {
		pods, err := cl.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: kmmmodule.MapToLabelSelector(ainicLabel),
		})
		if err != nil {
			return fmt.Errorf("list pods: %w", err)
		}
		for _, p := range pods.Items {
			for _, cs := range p.Status.ContainerStatuses {
				if !cs.Ready {
					return fmt.Errorf("pod %s/%s not ready", p.Name, cs.Name)
				}
			}
		}
		return nil
	}, 5*time.Minute, 5*time.Second); err != nil {
		return fmt.Errorf("pods not ready: %w", err)
	}
	return nil
}

func DeletePodWithNICResource(ctx context.Context, cl *kubernetes.Clientset) error {
	log.Print("deleting NIC resource ds")
	if err := DelDaemonset(cl, v1.NamespaceDefault, rocmDs); err != nil {
		return fmt.Errorf("delete ds: %w", err)
	}
	if err := Retry(func() error {
		pods, err := cl.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: kmmmodule.MapToLabelSelector(ainicLabel),
		})
		if err != nil {
			return fmt.Errorf("list pods: %w", err)
		}
		if len(pods.Items) > 0 {
			return fmt.Errorf("still %d pods", len(pods.Items))
		}
		return nil
	}, 5*time.Minute, 5*time.Second); err != nil {
		return fmt.Errorf("pods remain: %w", err)
	}
	return nil
}

func ListAinicPods(ctx context.Context, cl *kubernetes.Clientset) ([]string, error) {
	pods, err := cl.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: kmmmodule.MapToLabelSelector(ainicLabel),
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		out = append(out, p.Name)
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// Basic resource CRUD helpers
// -----------------------------------------------------------------------------
func DeletePod(ctx context.Context, cl *kubernetes.Clientset, ns, name string) error {
	return cl.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
}

func CreateTLSSecret(ctx context.Context, cl *kubernetes.Clientset, name, ns string, crt, key []byte) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			"tls.crt": crt,
			"tls.key": key,
		},
		Type: v1.SecretTypeTLS,
	}
	_, err := cl.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

func DeleteTLSSecret(ctx context.Context, cl *kubernetes.Clientset, name, ns string) error {
	return cl.CoreV1().Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{})
}

// -----------------------------------------------------------------------------
// DaemonSet helpers
// -----------------------------------------------------------------------------
func CreateDaemonsetVerify(ctx context.Context, cl *kubernetes.Clientset, ns, name, image string,
	matchLabels map[string]string, res *v1.ResourceRequirements) error {

	if res == nil {
		res = &v1.ResourceRequirements{
			Limits:   v1.ResourceList{"amd.com/nic": resource.MustParse("1")},
			Requests: v1.ResourceList{"amd.com/nic": resource.MustParse("1")},
		}
	}
	dsCli := cl.AppsV1().DaemonSets(ns)
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: matchLabels,
					Annotations: map[string]string{
						"k8s.v1.cni.cncf.io/networks": "amd-host-device-nad",
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"feature.node.kubernetes.io/amd-nic": "true"},
					Containers: []v1.Container{{
						Name:            name,
						Image:           image,
						Command:         []string{"sh", "-c", "--"},
						Args:            []string{"sleep infinity"},
						Resources:       *res,
						ImagePullPolicy: v1.PullIfNotPresent,
					}},
				},
			},
		},
	}
	if _, err := dsCli.Create(ctx, ds, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create ds: %w", err)
	}
	return Retry(func() error {
		cur, err := dsCli.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cur.Status.NumberReady == 0 || cur.Status.DesiredNumberScheduled != cur.Status.NumberReady {
			return fmt.Errorf("ds %s not ready status=%+v", name, cur.Status)
		}
		return nil
	}, 10*time.Minute, 5*time.Second)
}

func DelDaemonset(cl *kubernetes.Clientset, ns, name string) error {
	policy := metav1.DeletePropagationForeground
	return cl.AppsV1().DaemonSets(ns).Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}

// -----------------------------------------------------------------------------
// Name helpers
// -----------------------------------------------------------------------------
func DevicePluginName(cfgName string) string    { return cfgName + "-device-plugin" }
func NodeLabellerName(cfgName string) string    { return cfgName + "-node-labeller" }
func CNIPluginsName(cfgName string) string      { return cfgName + "-cni-plugins" }
func MetricsExporterName(cfgName string) string { return cfgName + "-metrics-exporter" }
func MultusName() string                        { return "amd-network-operator-multus-multus" }
func NFDWorkerName(isOpenshift bool) string {
	if isOpenshift {
		return "nfd-worker"
	}
	return "amd-network-operator-node-feature-discovery-worker"
}

// -----------------------------------------------------------------------------
// Exec helpers
// -----------------------------------------------------------------------------
func ExecPodCmd(command, ns, name, container string) (string, error) {
	args := []string{"exec", "-n", ns, name}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--", "sh", "-c", command)
	cmd := exec.Command(kubectl, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// -----------------------------------------------------------------------------
// Cluster detection
// -----------------------------------------------------------------------------
func GetClusterType(cfg *rest.Config) string {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return ClusterTypeK8s
	}
	groups, err := dc.ServerGroups()
	if err != nil {
		return ClusterTypeK8s
	}
	for _, gp := range groups.Groups {
		if gp.Name == "route.openshift.io" {
			return ClusterTypeOpenShift
		}
	}
	return ClusterTypeK8s
}

// -----------------------------------------------------------------------------
// Command execution / logging
// -----------------------------------------------------------------------------
func RunCommand(command string) {
	log.Infof("exec: %s", command)
	cmd := exec.Command("bash", "-c", command)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		log.Errorf("start: %v", err)
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		log.Infof("  %s", scanner.Text())
	}
	if err := cmd.Wait(); err != nil {
		log.Errorf("wait: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Node command proxy (HTTP sidecar expected)
// -----------------------------------------------------------------------------
func RunCommandOnNode(ctx context.Context, cl *kubernetes.Clientset, nodeName, command string) (string, error) {
	nodeip, err := GetNodeIP(ctx, cl, nodeName)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("http://%s:%s/runcommand", nodeip, HttpServerPort)
	body, err := json.Marshal(UserRequest{Command: command})
	if err != nil {
		return "", err
	}
	httpCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	log.Infof("runcommand resp status=%s err=%v", resp.Status, err)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}
	return string(data), nil
}

// -----------------------------------------------------------------------------
// Node listing / selection
// -----------------------------------------------------------------------------
func GetWorkerNodes(cl *kubernetes.Clientset) []*v1.Node {
	sel := labels.NewSelector()
	req, _ := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.DoesNotExist, nil)
	sel = sel.Add(*req)
	nodes, err := cl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		log.Errorf("GetWorkerNodes: %v", err)
		return nil
	}
	out := make([]*v1.Node, 0, len(nodes.Items))
	for i := range nodes.Items {
		out = append(out, &nodes.Items[i])
	}
	return out
}

func GetAMDNicWorker(cl *kubernetes.Clientset, _ bool) []v1.Node {
	log.Print("list nodes with AMD NIC label")
	sel := labels.NewSelector()
	req, _ := labels.NewRequirement("feature.node.kubernetes.io/amd-nic", selection.Equals, []string{"true"})
	sel = sel.Add(*req)
	nodes, err := cl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		log.Errorf("GetAMDNicWorker: %v", err)
		return nil
	}
	return nodes.Items
}

func GetNonAMDNicWorker(cl *kubernetes.Clientset) []v1.Node {
	sel := labels.NewSelector()
	r1, _ := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.DoesNotExist, nil)
	r2, _ := labels.NewRequirement("nic.vendor", selection.NotEquals, []string{"amd"})
	sel = sel.Add(*r1, *r2)
	nodes, err := cl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		log.Errorf("GetNonAMDNicWorker: %v", err)
		return nil
	}
	return nodes.Items
}

// -----------------------------------------------------------------------------
// Simple Pod creation
// -----------------------------------------------------------------------------
func CreatePod(ctx context.Context, cl *kubernetes.Clientset, ns, name, image, workerNodeName string) error {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PodSpec{
			NodeName: workerNodeName,
			Containers: []v1.Container{{
				Name:    name,
				Image:   image,
				Command: []string{"sh", "-c", "--"},
				Args:    []string{"sleep infinity"},
			}},
		},
	}
	_, err := cl.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Simple Configmap creation/deletion
// -----------------------------------------------------------------------------
func CreateConfigMap(ctx context.Context, cl *kubernetes.Clientset, ns string, cmName string, data map[string]string) error {
	cmClient := cl.CoreV1().ConfigMaps(ns)

	// Check if ConfigMap exists
	_, err := cmClient.Get(ctx, cmName, metav1.GetOptions{})
	if err == nil {
		// ConfigMap exists, delete it
		log.Infof("ConfigMap %s/%s already exists. Deleting it.", ns, cmName)
		err = cmClient.Delete(ctx, cmName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete existing ConfigMap %s/%s: %w", ns, cmName, err)
		}
		// Wait for the ConfigMap to be fully deleted before recreating
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(context.Context) (bool, error) {
			_, getErr := cmClient.Get(ctx, cmName, metav1.GetOptions{})
			if getErr != nil && apierrors.IsNotFound(getErr) {
				return true, nil // Successfully deleted
			}
			if getErr != nil {
				return false, getErr // Some other error, stop polling
			}
			return false, nil // Still exists, continue polling
		})
		if err != nil {
			return fmt.Errorf("error waiting for ConfigMap %s/%s to be deleted: %w", ns, cmName, err)
		}
		log.Infof("Successfully deleted existing ConfigMap %s/%s.", ns, cmName)
	} else if !apierrors.IsNotFound(err) {
		// Some other error occurred while trying to get the ConfigMap
		return fmt.Errorf("failed to get ConfigMap %s/%s: %w", ns, cmName, err)
	}

	// Define the new ConfigMap
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns,
		},
		Data: data,
	}

	// Create the ConfigMap
	log.Infof("Creating ConfigMap %s/%s.", ns, cmName)
	_, err = cmClient.Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap %s/%s: %w", ns, cmName, err)
	}
	log.Infof("Successfully created ConfigMap %s/%s.", ns, cmName)
	return nil
}

func DeleteConfigMap(ctx context.Context, cl *kubernetes.Clientset, ns string, cmName string) error {
	cmClient := cl.CoreV1().ConfigMaps(ns)
	err := cmClient.Delete(ctx, cmName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ConfigMap %s/%s: %w", ns, cmName, err)
	}
	log.Infof("Successfully deleted ConfigMap %s/%s.", ns, cmName)
	return nil
}

// -----------------------------------------------------------------------------
// Node-app DaemonSet (health / remote command utility)
// -----------------------------------------------------------------------------
func DeployNodeAppDaemonSet(cl *kubernetes.Clientset) error {
	hostPathDirectoryType := v1.HostPathDirectory
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-nodeapp-ds"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "e2e-nodeapp-ds"}},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"name": "e2e-nodeapp-ds"}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:            "e2e-nodeapp-container",
						Image:           nodeAppImage,
						ImagePullPolicy: v1.PullAlways,
						Lifecycle: &v1.Lifecycle{
							PreStop: &v1.LifecycleHandler{Exec: &v1.ExecAction{Command: []string{"./docker-exitpoint.sh"}}},
						},
						Env: []v1.EnvVar{{
							Name: "NODE_IP",
							ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{
								FieldPath: "status.hostIP",
							}},
						}},
						VolumeMounts: []v1.VolumeMount{{Name: "ssh-volume", MountPath: "/root/.ssh"}},
					}},
					Volumes: []v1.Volume{{
						Name: "ssh-volume",
						VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{Path: "/root/.ssh", Type: &hostPathDirectoryType},
						},
					}},
				},
			},
		},
	}
	dsCli := cl.AppsV1().DaemonSets(v1.NamespaceDefault)
	if _, err := dsCli.Create(context.TODO(), &ds, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create nodeapp ds: %w", err)
	}
	return Retry(func() error {
		cur, err := dsCli.Get(context.TODO(), ds.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cur.Status.NumberReady == 0 || cur.Status.DesiredNumberScheduled != cur.Status.NumberReady {
			return fmt.Errorf("nodeapp ds not ready status=%+v", cur.Status)
		}
		return nil
	}, 10*time.Minute, 5*time.Second)
}

func DeleteNodeAppDaemonSet(cl *kubernetes.Clientset) error {
	return cl.AppsV1().DaemonSets(v1.NamespaceDefault).
		Delete(context.TODO(), "e2e-nodeapp-ds", metav1.DeleteOptions{})
}

// -----------------------------------------------------------------------------
// Service helpers
// -----------------------------------------------------------------------------
func GetClusterIP(clientset *kubernetes.Clientset, serviceName, namespace string) (string, error) {
	svc, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get service: %w", err)
	}
	return svc.Spec.ClusterIP, nil
}

// -----------------------------------------------------------------------------
// YAML utilities
// -----------------------------------------------------------------------------
func SplitYAML(data []byte) [][]byte {
	docs := strings.Split(string(data), "---")
	out := make([][]byte, 0, len(docs))
	for _, d := range docs {
		td := strings.TrimSpace(d)
		if td != "" {
			out = append(out, []byte(td))
		}
	}
	return out
}

func DeployResourcesFromFile(pathOrURL string, cl *kubernetes.Clientset, apiCl *apiextClient.Clientset, create bool) error {
	var data []byte
	var err error
	var fileName string
	if strings.HasPrefix(pathOrURL, "http") || strings.HasPrefix(pathOrURL, "https") {
		resp, err := http.Get(pathOrURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get file from URL: %s", pathOrURL)
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %s", pathOrURL)
		}
	} else {
		fileName = pathOrURL
		if !strings.HasPrefix(fileName, "/") {
			fileName = "./yamls/config/" + fileName
		}
		data, err = os.ReadFile(fileName)
		if err != nil {
			return fmt.Errorf("failed to read file: %s", fileName)
		}
	}

	// Decode the YAML data
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()

	// Split the YAML data into separate documents
	documents := SplitYAML(data)
	for _, doc := range documents {
		obj, _, err := decoder.Decode(doc, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to decode yaml %+v: %+v", doc, err)
		}

		switch resource := obj.(type) {
		case *v1.Namespace:
			if create {
				_, err = cl.CoreV1().Namespaces().Create(context.TODO(), resource, metav1.CreateOptions{})
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("failed to create namespace %+v: %+v", resource, err)
				}
			} else {
				err = cl.CoreV1().Namespaces().Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
				if err != nil {
					return fmt.Errorf("failed to delete namespace %+v: %+v", resource, err)
				}
				// wait for namespace to be completely removed
				waitEventually(fmt.Sprintf("namespace %s deleted", resource.Name), defaultRetryTimeout, defaultRetryInterval, func() (bool, error) {
					_, err := cl.CoreV1().Namespaces().Get(context.TODO(), resource.Name, metav1.GetOptions{})
					return apierrors.IsNotFound(err), nil
				})
			}

		case *rbacv1.ClusterRole:
			if create {
				_, err = cl.RbacV1().ClusterRoles().Create(context.TODO(), resource, metav1.CreateOptions{})
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("failed to create clusterrole %+v: %+v", resource, err)
				}
			} else {
				err = cl.RbacV1().ClusterRoles().Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
				if err != nil {
					return fmt.Errorf("failed to delete clusterrole %+v: %+v", resource, err)
				}
				// wait for clusterrole to be completely removed
				waitEventually(fmt.Sprintf("clusterrole %s deleted", resource.Name), defaultRetryTimeout, defaultRetryInterval, func() (bool, error) {
					_, err := cl.RbacV1().ClusterRoles().Get(context.TODO(), resource.Name, metav1.GetOptions{})
					return apierrors.IsNotFound(err), nil
				})
			}

		case *rbacv1.ClusterRoleBinding:
			if create {
				_, err = cl.RbacV1().ClusterRoleBindings().Create(context.TODO(), resource, metav1.CreateOptions{})
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("failed to create clusterrole binding %+v: %+v", resource, err)
				}
			} else {
				err = cl.RbacV1().ClusterRoleBindings().Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
				if err != nil {
					return fmt.Errorf("failed to delete clusterrole binding %+v: %+v", resource, err)
				}
				// wait for clusterrolebinding to be completely removed
				waitEventually(fmt.Sprintf("clusterrolebinding %s deleted", resource.Name), defaultRetryTimeout, defaultRetryInterval, func() (bool, error) {
					_, err := cl.RbacV1().ClusterRoleBindings().Get(context.TODO(), resource.Name, metav1.GetOptions{})
					return apierrors.IsNotFound(err), nil
				})
			}

		case *batchv1.Job:
			if create {
				_, err = cl.BatchV1().Jobs(resource.Namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create batch job %+v: %+v", resource, err)
				}
			} else {
				err = cl.BatchV1().Jobs(resource.Namespace).Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
				if err != nil {
					return fmt.Errorf("failed to delete batch job %+v: %+v", resource, err)
				}
				// wait for batch job to be completely removed
				waitEventually(fmt.Sprintf("job %s deleted", resource.Name), defaultRetryTimeout, defaultRetryInterval, func() (bool, error) {
					_, err := cl.BatchV1().Jobs(resource.Namespace).Get(context.TODO(), resource.Name, metav1.GetOptions{})
					return apierrors.IsNotFound(err), nil
				})
			}

		case *apiextv1.CustomResourceDefinition:
			if create {
				_, err = apiCl.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), resource, metav1.CreateOptions{})
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("failed to create CRD %+v: %+v", resource, err)
				}
			} else {
				err = apiCl.ApiextensionsV1().CustomResourceDefinitions().Delete(context.TODO(), resource.Name, metav1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete CRD %+v: %+v", resource, err)
				}
				// wait for CRD to be completely removed
				waitEventually(fmt.Sprintf("CRD %s deleted", resource.Name), defaultRetryTimeout, defaultRetryInterval, func() (bool, error) {
					_, err := apiCl.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), resource.Name, metav1.GetOptions{})
					return apierrors.IsNotFound(err), nil
				})
			}

		default:
			return fmt.Errorf("unsupported resource type %+v", resource)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Node IP / health / reboot
// -----------------------------------------------------------------------------
func GetNodeIP(_ context.Context, cl *kubernetes.Clientset, nodeName string) (string, error) {
	node, err := cl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get node: %w", err)
	}
	for _, a := range node.Status.Addresses {
		if a.Type == v1.NodeInternalIP {
			return a.Address, nil
		}
	}
	return "", ErrNodeIPMissing
}

func IsNodeHealthy(cl *kubernetes.Clientset, nodeip string) error {
	url := fmt.Sprintf("http://%s:%s/health", nodeip, HttpServerPort)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Infof("health resp: %s body=%s", resp.Status, string(body))
	if resp.StatusCode != http.StatusOK || string(body) != "healthy" {
		return fmt.Errorf("unhealthy status=%s body=%s", resp.Status, body)
	}
	return nil
}

func RebootNode(cl *kubernetes.Clientset, nodeip string) error {
	url := fmt.Sprintf("http://%s:%s/reboot", nodeip, HttpServerPort)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reboot failed status=%s body=%s", resp.Status, string(body))
	}
	return nil
}

func RebootNodeWithWait(ctx context.Context, cl *kubernetes.Clientset, nodeName string) error {
	nodeip, err := GetNodeIP(ctx, cl, nodeName)
	if err != nil {
		return err
	}
	if err := RebootNode(cl, nodeip); err != nil {
		return err
	}
	if err := Retry(func() error {
		return IsNodeHealthy(cl, nodeip)
	}, 10*time.Minute, 20*time.Second); err != nil {
		return fmt.Errorf("node not healthy post reboot: %w", err)
	}
	return nil
}

func RebootNodesWithWait(ctx context.Context, cl *kubernetes.Clientset, nodes []v1.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(nodes))
	for _, n := range nodes {
		wg.Add(1)
		go func(node v1.Node) {
			defer wg.Done()
			if err := RebootNodeWithWait(ctx, cl, node.Name); err != nil {
				errCh <- fmt.Errorf("node %s: %w", node.Name, err)
			}
		}(n)
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		return e
	}
	return nil
}

// Pod-based reboot (operator specific)
func GetRebootPod(nodeName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("amd-network-operator-%s-reboot-worker", nodeName),
			Namespace: "kube-amd-network",
		},
		Spec: v1.PodSpec{
			HostPID:       true,
			HostNetwork:   true,
			RestartPolicy: v1.RestartPolicyNever,
			NodeSelector:  map[string]string{"kubernetes.io/hostname": nodeName},
			Containers: []v1.Container{{
				Name:            "reboot-container",
				Image:           "docker.io/amdpsdo/network-operator-utils:v1.0.0",
				Command:         []string{"/nsenter", "--all", "--target=1", "--", "sudo", "reboot"},
				Stdin:           true,
				TTY:             true,
				SecurityContext: &v1.SecurityContext{Privileged: ptr.To(true)},
				ImagePullPolicy: v1.PullIfNotPresent,
			}},
			Tolerations: []v1.Toleration{{
				Key:      "amd-nic-driver-upgrade",
				Value:    "true",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			}},
		},
	}
}

func HandleNodesReboot(ctx context.Context, cl *kubernetes.Clientset, nodes []v1.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(nodes))
	for _, node := range nodes {
		wg.Add(1)
		go func(n v1.Node) {
			defer wg.Done()
			rp := GetRebootPod(n.Name)
			// Ensure stale pod removed
			if _, err := cl.CoreV1().Pods(rp.Namespace).Get(ctx, rp.Name, metav1.GetOptions{}); err == nil {
				_ = cl.CoreV1().Pods(rp.Namespace).Delete(ctx, rp.Name, metav1.DeleteOptions{})
			}
			if _, err := cl.CoreV1().Pods(rp.Namespace).Create(ctx, rp, metav1.CreateOptions{}); err != nil {
				errCh <- fmt.Errorf("create reboot pod %s: %w", n.Name, err)
				return
			}
			// Wait for pod appear then delete (triggers host reboot)
			_ = Retry(func() error {
				_, err := cl.CoreV1().Pods(rp.Namespace).Get(ctx, rp.Name, metav1.GetOptions{})
				return err
			}, 10*time.Minute, 2*time.Second)
			DeleteRebootPod(ctx, cl, n.Name, false)
		}(node)
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		return e
	}
	return nil
}

func DeleteRebootPod(ctx context.Context, cl *kubernetes.Clientset, nodeName string, force bool) {
	rp := GetRebootPod(nodeName)
	_, err := cl.CoreV1().Pods(rp.Namespace).Get(ctx, rp.Name, metav1.GetOptions{})
	if err != nil {
		return
	}
	if !force {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		timeout := time.After(60 * time.Minute)
		for {
			select {
			case <-ticker.C:
				pod, err := cl.CoreV1().Pods(rp.Namespace).Get(ctx, rp.Name, metav1.GetOptions{})
				if err != nil {
					return
				}
				if len(pod.Status.ContainerStatuses) > 0 {
					cs := pod.Status.ContainerStatuses[0]
					if cs.State.Terminated != nil && !cs.State.Terminated.FinishedAt.IsZero() {
						_ = cl.CoreV1().Pods(rp.Namespace).Delete(ctx, rp.Name, metav1.DeleteOptions{})
						return
					}
				}
			case <-timeout:
				goto FORCE
			}
		}
	}
FORCE:
	_ = cl.CoreV1().Pods(rp.Namespace).Delete(ctx, rp.Name, metav1.DeleteOptions{})
}

// -----------------------------------------------------------------------------
// Additional simple helpers
// -----------------------------------------------------------------------------
func CreateDaemonset(cl *kubernetes.Clientset, ns, name, image string, matchLabels map[string]string, res *v1.ResourceRequirements) error {
	if res == nil {
		res = &v1.ResourceRequirements{
			Limits:   v1.ResourceList{"amd.com/nic": resource.MustParse("1")},
			Requests: v1.ResourceList{"amd.com/nic": resource.MustParse("1")},
		}
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: matchLabels},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"feature.node.kubernetes.io/amd-nic": "true"},
					Containers: []v1.Container{{
						Name:      name,
						Image:     image,
						Command:   []string{"sh", "-c", "--"},
						Args:      []string{"sleep infinity"},
						Resources: *res,
					}},
				},
			},
		},
	}
	_, err := cl.AppsV1().DaemonSets(ns).Create(context.TODO(), ds, metav1.CreateOptions{})
	return err
}

func IsJSONParsable(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// -----------------------------------------------------------------------------
// Node label / taint management
// -----------------------------------------------------------------------------
func AddNodeLabel(cl *kubernetes.Clientset, nodeName, key, value string) error {
	node, err := cl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	node.Labels[key] = value
	_, err = cl.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func DeleteNodeLabel(cl *kubernetes.Clientset, nodeName, key string) error {
	node, err := cl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	delete(node.Labels, key)
	_, err = cl.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func NodeTaint(cl *kubernetes.Clientset, nodeName string) error {
	node, err := cl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	taint := v1.Taint{Key: "dcm", Value: "up", Effect: v1.TaintEffectNoExecute}
	node.Spec.Taints = append(node.Spec.Taints, taint)
	_, err = cl.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

// -----------------------------------------------------------------------------
// Secrets
// -----------------------------------------------------------------------------
func CreateOpaqueSecret(ctx context.Context, cl *kubernetes.Clientset, name, ns string, keys map[string]string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		StringData: keys,
		Type:       v1.SecretTypeOpaque,
	}
	_, err := cl.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

func DeleteOpaqueSecret(ctx context.Context, cl *kubernetes.Clientset, name, ns string) {
	if err := cl.CoreV1().Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		log.Errorf("delete secret %s/%s: %v", ns, name, err)
	}
}

// -----------------------------------------------------------------------------
// MinIO helpers
// -----------------------------------------------------------------------------
func CreateMinioService(ctx context.Context, cl *kubernetes.Clientset, ns, hostName string) error {
	hpDir := v1.HostPathDirectoryOrCreate
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio",
			Namespace: ns,
			Labels:    map[string]string{"app": "minio"},
		},
		Spec: v1.PodSpec{
			NodeSelector: map[string]string{"kubernetes.io/hostname": hostName},
			Containers: []v1.Container{{
				Name:    "minio",
				Image:   minioImage,
				Command: []string{"/bin/bash", "-c"},
				Args:    []string{"minio server /data --console-address :9090"},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "localvolume",
					MountPath: "/data",
				}},
			}},
			Volumes: []v1.Volume{{
				Name: "localvolume",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{Path: "/data", Type: &hpDir},
				},
			}},
		},
	}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "minio", Namespace: ns},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"app": "minio"},
			Type:     v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{Name: "console", Port: 9090, NodePort: 31250},
				{Name: "s3", Port: 9000, NodePort: 31260},
			},
		},
	}
	if _, err := cl.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return err
	}
	_, err := cl.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	return err
}

func DeleteMinioService(ctx context.Context, cl *kubernetes.Clientset, ns string) {
	if err := cl.CoreV1().Pods(ns).Delete(ctx, "minio", metav1.DeleteOptions{}); err != nil {
		log.Errorf("delete minio pod: %v", err)
	}
	if err := cl.CoreV1().Services(ns).Delete(ctx, "minio", metav1.DeleteOptions{}); err != nil {
		log.Errorf("delete minio service: %v", err)
	}
}

func SetupAccessKeysOnMinioServer(ns, pod, container, accessKey, secretKey string) {
	cmd := fmt.Sprintf(`mc alias set local http://localhost:9000 minioadmin minioadmin && mc admin accesskey create local/ minioadmin --access-key %s --secret-key %s`, accessKey, secretKey)
	if _, err := ExecPodCmd(cmd, ns, pod, container); err != nil {
		log.Errorf("setup access key: %v", err)
	}
}

// -----------------------------------------------------------------------------
// DoCurl performs a curl request with given options, using a client pod or directly on host
// Parameters:
//   - endpoint: The Endpoint to curl
//   - token: Bearer token for authentication (optional)
//   - verifyTLS: Verify server TLS certificate
//   - caCertPath: Path to CA certificate file for SSL verification
//   - clientCertPath: Path to client certificate for mTLS
//   - clientKeyPath: Path to client key for mTLS
//   - clientPod: Pod to execute curl from (if nil, executes locally)
//   - silent: If true, adds -s flag (silent mode, no progress/error output)
//   - showError: If true, adds -S flag (show errors even in silent mode)
//
// -----------------------------------------------------------------------------
func DoCurl(
	url string,
	token string,
	verifyTLS bool,
	caCertPath string,
	clientCertPath string,
	clientKeyPath string,
	clientPod *v1.Pod,
	silent bool,
	showError bool,
) (string, error) {
	// Choose scheme
	tlsArg := ""
	if !verifyTLS {
		tlsArg = "-k"
	}

	// CA bundle
	caArg := ""
	if caCertPath != "" {
		caArg = fmt.Sprintf("--cacert %s", caCertPath)
	}

	// client cert & key for mTLS (both must be provided)
	certArg := ""
	if clientCertPath != "" && clientKeyPath != "" {
		certArg = fmt.Sprintf("--cert %s --key %s", clientCertPath, clientKeyPath)
	}

	// bearer token header, if given
	authArg := ""
	if token != "" {
		authArg = fmt.Sprintf("-H \"Authorization: Bearer %s\"", token)
	}

	// build curl args with optional flags
	var argList []string

	if silent {
		argList = append(argList, "-s")
	}
	if showError {
		argList = append(argList, "-S")
	}
	if tlsArg != "" {
		argList = append(argList, tlsArg)
	}
	if caArg != "" {
		argList = append(argList, caArg)
	}
	if certArg != "" {
		argList = append(argList, certArg)
	}
	if authArg != "" {
		argList = append(argList, authArg)
	}

	args := strings.Join(argList, " ")

	cmd := fmt.Sprintf("curl %s %s", args, url)

	// Execute command based on whether client pod is provided
	if clientPod == nil {
		execCmd := exec.Command("sh", "-c", cmd)
		output, err := execCmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to execute curl command %v: %w", cmd, err)
		}
		return string(output), nil
	}

	// Use pod-based execution
	output, err := ExecPodCmd(cmd, clientPod.Namespace, clientPod.Name, clientPod.Spec.Containers[0].Name)
	if err != nil {
		return "", fmt.Errorf("failed to execute curl command inside client pod %v: %w", cmd, err)
	}
	return output, nil
}

// -----------------------------------------------------------------------------
// Node IP utilities
// -----------------------------------------------------------------------------
func GetNodeIPs(clientset *kubernetes.Clientset) ([]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	var ips []string
	for _, n := range nodes.Items {
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP || a.Type == v1.NodeExternalIP {
				ips = append(ips, a.Address)
			}
		}
	}
	return ips, nil
}

func GetNodeIPsForDaemonSet(clientset *kubernetes.Clientset, dsName, namespace string) ([]string, error) {
	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), dsName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get ds: %w", err)
	}
	labelSelector := metav1.FormatLabelSelector(ds.Spec.Selector)
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	var ips []string
	for _, p := range pods.Items {
		ips = append(ips, p.Status.HostIP)
	}
	return ips, nil
}

// -----------------------------------------------------------------------------
// Job logs
// -----------------------------------------------------------------------------
func GetPodNamesFromJob(clientset *kubernetes.Clientset, job *batchv1.Job) ([]string, error) {
	if job == nil {
		return nil, fmt.Errorf("nil job")
	}
	ls := fmt.Sprintf("job-name=%s", job.Name)
	pods, err := clientset.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: ls})
	if err != nil {
		return nil, fmt.Errorf("list job pods: %w", err)
	}
	out := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		out = append(out, p.Name)
	}
	return out, nil
}

func GetJobLogs(clientset *kubernetes.Clientset, job *batchv1.Job) ([]string, error) {
	podNames, err := GetPodNamesFromJob(clientset, job)
	if err != nil {
		return nil, err
	}
	var logsOut []string
	for _, podName := range podNames {
		stream, err := clientset.CoreV1().Pods(job.Namespace).
			GetLogs(podName, &v1.PodLogOptions{}).Stream(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("stream logs %s: %w", podName, err)
		}
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stream)
		_ = stream.Close()
		logsOut = append(logsOut, fmt.Sprintf("Pod %s logs:\n%s", podName, buf.String()))
	}
	return logsOut, nil
}

// -----------------------------------------------------------------------------
// ServiceAccount token
// -----------------------------------------------------------------------------
func GenerateServiceAccountToken(clientset *kubernetes.Clientset, saName, ns string) (string, error) {
	secs := int64(24 * 3600)
	tr := &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &secs}}
	resp, err := clientset.CoreV1().ServiceAccounts(ns).CreateToken(context.TODO(), saName, tr, metav1.CreateOptions{})
	if err != nil || resp.Status.Token == "" {
		return "", fmt.Errorf("create token sa=%s: %w resp=%+v", saName, err, resp)
	}
	return resp.Status.Token, nil
}

// -----------------------------------------------------------------------------
// Temp file helpers
// -----------------------------------------------------------------------------
func CreateTempFile(fileName string, data []byte) (*os.File, error) {
	f, err := os.CreateTemp("", fileName)
	if err != nil {
		return nil, fmt.Errorf("temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("write temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close temp: %w", err)
	}
	return f, nil
}

func DeleteTempFile(file *os.File) error {
	if file == nil {
		return fmt.Errorf("nil file")
	}
	return os.Remove(file.Name())
}

// -----------------------------------------------------------------------------
// Patching deployments for CI ENV
// -----------------------------------------------------------------------------
func PatchOperatorControllerDeploymentWithCIENVFlag(cl *kubernetes.Clientset) error {
	patch := []map[string]interface{}{
		{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": map[string]string{"name": "CI_ENV", "value": "true"}},
		{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": map[string]string{"name": "INTERNAL_UBUNTU_BASE", "value": ubuntuBaseImage}},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	if _, err = cl.AppsV1().Deployments("kube-amd-network").Patch(
		context.TODO(),
		"amd-network-operator-network-operator-charts-controller-manager",
		types.JSONPatchType,
		data,
		metav1.PatchOptions{},
	); err != nil {
		return fmt.Errorf("patch controller: %w", err)
	}
	time.Sleep(60 * time.Second)
	return nil
}

func PatchKMMDeploymentWithCIENVFlag(cl *kubernetes.Clientset) error {
	patch := []map[string]interface{}{
		{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": map[string]string{"name": "CI_ENV", "value": "true"}},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	if _, err = cl.AppsV1().Deployments("kube-amd-network").Patch(
		context.TODO(),
		"amd-network-operator-kmm-controller",
		types.JSONPatchType,
		data,
		metav1.PatchOptions{},
	); err != nil {
		return fmt.Errorf("patch kmm: %w", err)
	}
	time.Sleep(60 * time.Second)
	return nil
}
