package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
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

// GetExposedEndpoints lists services of type LoadBalancer, NodePort, and Ingresses.
func GetExposedEndpoints(clientset *kubernetes.Clientset) ([]string, error) {
	var endpoints []string

	// List Services
	services, err := clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, svc := range services.Items {
		switch svc.Spec.Type {
		case corev1.ServiceTypeLoadBalancer:
			var lbIPs []string
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					lbIPs = append(lbIPs, ingress.IP)
				} else if ingress.Hostname != "" {
					lbIPs = append(lbIPs, ingress.Hostname) // For ELBs that return DNS names
				}
			}
			var portStrings []string
			for _, port := range svc.Spec.Ports {
				portStrings = append(portStrings, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
			}
			if len(lbIPs) > 0 {
				endpoint := fmt.Sprintf("Service (LoadBalancer): %s/%s - External Endpoint(s): [%s], Port(s): [%s]",
					svc.Namespace, svc.Name, strings.Join(lbIPs, ", "), strings.Join(portStrings, ", "))
				endpoints = append(endpoints, endpoint)
			}
		case corev1.ServiceTypeNodePort:
			var portStrings []string
			for _, port := range svc.Spec.Ports {
				portStrings = append(portStrings, fmt.Sprintf("%d:%d/%s", port.Port, port.NodePort, port.Protocol))
			}
			endpoint := fmt.Sprintf("Service (NodePort): %s/%s - NodePort(s): [%s] (exposed on all node IPs)",
				svc.Namespace, svc.Name, strings.Join(portStrings, ", "))
			endpoints = append(endpoints, endpoint)
		}
	}

	// List Ingresses
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = "*" // Default host if not specified
			}
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					backend := fmt.Sprintf("%s:%d", path.Backend.Service.Name, path.Backend.Service.Port.Number)
					// Some ingress controllers might populate status with load balancer IPs/hostnames
					var ingStatusIPs []string
					for _, lbIngress := range ing.Status.LoadBalancer.Ingress {
						if lbIngress.IP != "" {
							ingStatusIPs = append(ingStatusIPs, lbIngress.IP)
						} else if lbIngress.Hostname != "" {
							ingStatusIPs = append(ingStatusIPs, lbIngress.Hostname)
						}
					}
					var endpoint string
					if len(ingStatusIPs) > 0 {
						endpoint = fmt.Sprintf("Ingress: %s/%s - Host: %s, Path: %s -> %s, External Endpoint(s): [%s]",
							ing.Namespace, ing.Name, host, path.Path, backend, strings.Join(ingStatusIPs, ", "))
					} else {
						endpoint = fmt.Sprintf("Ingress: %s/%s - Host: %s, Path: %s -> %s",
							ing.Namespace, ing.Name, host, path.Path, backend)
					}
					endpoints = append(endpoints, endpoint)
				}
			}
		}
	}

	return endpoints, nil
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

	exposedEndpoints, err := GetExposedEndpoints(clientset)
	if err != nil {
		fmt.Printf("Could not get exposed endpoints: %v\n", err)
	} else {
		fmt.Println("Detected Exposed Endpoints:")
		if len(exposedEndpoints) == 0 {
			fmt.Println("  No exposed LoadBalancer, NodePort services, or Ingresses found.")
		} else {
			for _, endpoint := range exposedEndpoints {
				fmt.Printf("  - %s\n", endpoint)
			}
		}
	}
}
