package main

import (
	"context"
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	http.HandleFunc("/inject", mutateHandler)
	http.ListenAndServe(":8080", nil)
}

func mutateHandler(w http.ResponseWriter, r *http.Request) {
	admissionReview := handleAdmissionRequest(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(admissionReview)
}

func handleAdmissionRequest(r *http.Request) *admissionv1.AdmissionReview {
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		return errorResponse(err)
	}

	pod := corev1.Pod{}
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, &pod); err != nil {
		return errorResponse(err)
	}

	// Get all availability zones from referenced PVCs
	zones := getPVCAvailabilityZones(pod.Namespace, pod.Spec.Volumes)

	if len(zones) == 0 {
		return &admissionReview // No modification needed
	}

	// Create JSON patch for node affinity
	patch := createNodeAffinityPatch(pod.Spec.Affinity, zones)
	patchBytes, _ := json.Marshal(patch)

	admissionReview.Response = &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}

	return &admissionReview
}

func getPVCAvailabilityZones(namespace string, volumes []corev1.Volume) []string {
	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	uniqueZones := make(map[string]struct{})

	for _, vol := range volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		ctx := context.Background()
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx,
			vol.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if err != nil || pvc.Status.Phase != corev1.ClaimBound {
			continue
		}

		pv, err := clientset.CoreV1().PersistentVolumes().Get(ctx,
			pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		if zone := pv.Labels["topology.kubernetes.io/zone"]; zone != "" {
			uniqueZones[zone] = struct{}{}
		}
	}

	zones := make([]string, 0, len(uniqueZones))
	for zone := range uniqueZones {
		zones = append(zones, zone)
	}

	return zones
}

func createNodeAffinityPatch(existingAffinity *corev1.Affinity, zones []string) []map[string]interface{} {
	nodeSelectorTerms := []corev1.NodeSelectorTerm{{
		MatchExpressions: []corev1.NodeSelectorRequirement{{
			Key:      "topology.kubernetes.io/zone",
			Operator: corev1.NodeSelectorOpIn,
			Values:   zones,
		}},
	}}

	var patches []map[string]interface{}

	if existingAffinity == nil || existingAffinity.NodeAffinity == nil {
		patches = append(patches, map[string]interface{}{
			"op":   "add",
			"path": "/spec/affinity",
			"value": &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: nodeSelectorTerms,
					},
				},
			},
		})
	} else {
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms/-",
			"value": nodeSelectorTerms[0],
		})
	}

	return patches
}

func errorResponse(err error) *admissionv1.AdmissionReview {
	return &admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		},
	}
}
