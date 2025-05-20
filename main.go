package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetKubernetesAPIServerVersion retrieves the server version from the Kubernetes cluster.
func GetKubernetesAPIServerVersion(clientset *kubernetes.Clientset) (string, error) {
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}
	return serverVersion.GitVersion, nil
}

// GetEtcdVersion retrieves the etcd version by inspecting etcd pods in kube-system.
func GetEtcdVersion(clientset *kubernetes.Clientset) (string, error) {
	pods, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "component=etcd",
	})
	if err != nil {
		return "", fmt.Errorf("failed to list etcd pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no etcd pods found in kube-system namespace")
	}

	// Assume all etcd pods run the same version, take the first one.
	etcdPod := pods.Items[0]
	for _, container := range etcdPod.Spec.Containers {
		// The etcd container might not always be named 'etcd'.
		// A common convention is that it's the main container or simply named 'etcd'.
		// We check if the image name contains 'etcd'. This is a heuristic.
		if strings.Contains(container.Image, "etcd") {
			imageParts := strings.Split(container.Image, ":")
			if len(imageParts) > 1 {
				// The part after the last colon is typically the tag/version.
				// For images like k8s.gcr.io/etcd:3.5.1-0 or similar.
				versionPart := imageParts[len(imageParts)-1]
				// Further stripping might be needed if there are build suffixes, e.g., "3.5.1-0"
				// For simplicity, we return the full tag here.
				return versionPart, nil
			}
			return "", fmt.Errorf("etcd container image '%s' does not have a discernible version tag", container.Image)
		}
	}

	return "", fmt.Errorf("could not find etcd container in pod %s", etcdPod.Name)
}

// GetNodeVersions retrieves the Kubelet versions from all nodes in the cluster.
// It returns a comma-separated string of unique versions.
func GetNodeVersions(clientset *kubernetes.Clientset) (string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no nodes found in the cluster")
	}

	uniqueVersions := make(map[string]struct{})
	for _, node := range nodes.Items {
		uniqueVersions[node.Status.NodeInfo.KubeletVersion] = struct{}{}
	}

	versions := make([]string, 0, len(uniqueVersions))
	for v := range uniqueVersions {
		versions = append(versions, v)
	}

	return strings.Join(versions, ", "), nil
}

func main() {
	fmt.Println("Attempting to connect to Kubernetes cluster...")

	clientset, err := NewClientFromKubeconfig()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	fmt.Println("Successfully connected to Kubernetes cluster!")

	kubeVersion, err := GetKubernetesAPIServerVersion(clientset)
	if err != nil {
		log.Fatalf("Failed to get Kubernetes version: %v", err)
	}
	fmt.Printf("Kubernetes API server version: %s\n", kubeVersion)

	etcdVersion, err := GetEtcdVersion(clientset)
	if err != nil {
		// For now, just print a warning if etcd version can't be fetched, as it's not critical.
		fmt.Printf("Could not get etcd version: %v\n", err)
	} else {
		fmt.Printf("Detected etcd version: %s\n", etcdVersion)
	}

	nodeVersions, err := GetNodeVersions(clientset)
	if err != nil {
		fmt.Printf("Could not get node versions: %v\n", err)
	} else {
		fmt.Printf("Detected node versions: %s\n", nodeVersions)
	}
}
