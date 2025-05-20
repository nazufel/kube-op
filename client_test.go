package main

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	validKubeconfigContent = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://fake-cluster.local
    # No certificate-authority-data needed for this test if we don't make calls
  name: fake-cluster
users:
- name: fake-user
  # No user token or certs needed for this test
contexts:
- context:
    cluster: fake-cluster
    user: fake-user
  name: fake-context
current-context: fake-context
`
	invalidKubeconfigContent = `` // Empty content is invalid
)

func TestNewClientFromKubeconfig_ValidKubeconfigEnv(t *testing.T) {
	tempDir := t.TempDir()
	kubeconfigFile := filepath.Join(tempDir, "config")
	if err := os.WriteFile(kubeconfigFile, []byte(validKubeconfigContent), 0600); err != nil {
		t.Fatalf("Failed to write temp kubeconfig: %v", err)
	}

	originalKubeconfig := os.Getenv("KUBECONFIG")
	t.Setenv("KUBECONFIG", kubeconfigFile)
	defer func() {
		t.Setenv("KUBECONFIG", originalKubeconfig)
	}()

	clientset, err := NewClientFromKubeconfig()
	if err != nil {
		t.Errorf("NewClientFromKubeconfig() returned error = %v, want nil", err)
	}
	if clientset == nil {
		t.Errorf("NewClientFromKubeconfig() returned clientset = nil, want non-nil")
	}
}

func TestNewClientFromKubeconfig_InvalidKubeconfigEnv(t *testing.T) {
	tempDir := t.TempDir()
	kubeconfigFile := filepath.Join(tempDir, "invalid_config")
	if err := os.WriteFile(kubeconfigFile, []byte(invalidKubeconfigContent), 0600); err != nil {
		t.Fatalf("Failed to write temp invalid kubeconfig: %v", err)
	}

	originalKubeconfig := os.Getenv("KUBECONFIG")
	t.Setenv("KUBECONFIG", kubeconfigFile)
	defer func() {
		t.Setenv("KUBECONFIG", originalKubeconfig)
	}()

	clientset, err := NewClientFromKubeconfig()
	if err == nil {
		t.Errorf("NewClientFromKubeconfig() with invalid config returned error = nil, want non-nil error")
	}
	if clientset != nil {
		t.Errorf("NewClientFromKubeconfig() with invalid config returned clientset != nil, want nil")
	}
}

func TestNewClientFromKubeconfig_NoKubeconfigPath(t *testing.T) {
	// To test this, we rely on the default ~/.kube/config not existing or being invalid,
	// and KUBECONFIG env var being unset.
	// We also need to simulate homedir.HomeDir() returning a path that doesn't lead to a valid config,
	// or homedir.HomeDir() returning an empty string.
	// Forcing homedir.HomeDir() to return specific values for testing without a mocking framework is hard.
	// So, this test makes a best effort by unsetting KUBECONFIG and hoping that
	// the default resolution path (like ~/.kube/config) isn't present and valid in the test environment.
	// If this test becomes flaky due to a valid default kubeconfig in the environment,
	// it might need a more sophisticated setup (e.g., running in a container with no ~/.kube).

	originalKubeconfig := os.Getenv("KUBECONFIG")
	t.Setenv("KUBECONFIG", "") // Unset KUBECONFIG
	defer func() {
		t.Setenv("KUBECONFIG", originalKubeconfig)
	}()

	// To make this test more robust against an existing default kubeconfig,
	// we can temporarily set HOME to a non-existent directory.
	// This should make homedir.HomeDir() return an error or a path that won't resolve.
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", "/tmp/nonexistent-home-for-test-"+t.Name()) // A path that almost certainly won't exist
	defer func() {
		t.Setenv("HOME", originalHome)
	}()

	clientset, err := NewClientFromKubeconfig()
	if err == nil {
		t.Errorf("NewClientFromKubeconfig() with no kubeconfig path returned error = nil, want non-nil error")
	}
	if clientset != nil {
		t.Errorf("NewClientFromKubeconfig() with no kubeconfig path returned clientset != nil, want nil")
	}
}
