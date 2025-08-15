package main

import (
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestCreateNodeAffinityPatchPreservesExistingAffinity(t *testing.T) {
	existing := &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{},
	}
	zones := []string{"us-east-1a"}
	patches := createNodeAffinityPatch(existing, zones)
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch["path"] != "/spec/affinity/nodeAffinity" {
		t.Fatalf("unexpected patch path: %v", patch["path"])
	}
	if _, ok := patch["value"].(*corev1.NodeAffinity); !ok {
		t.Fatalf("patch value is not NodeAffinity")
	}
}
