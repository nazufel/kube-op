package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Attempting to connect to Kubernetes cluster...")

	clientset, err := NewClientFromKubeconfig()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	fmt.Println("Successfully connected to Kubernetes cluster!")

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		log.Fatalf("Failed to get server version: %v", err)
	}

	fmt.Printf("Kubernetes server version: %s\n", serverVersion.GitVersion)
}
