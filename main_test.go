package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMutateHandlerSuccess(t *testing.T) {
	// Set up the environment variable for the pod name
	os.Setenv("POD_NAME", "test-webhook-pod")
	defer os.Unsetenv("POD_NAME")

	// Create a sample AdmissionReview request
	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "nginx",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu": "200m",
						},
					},
				},
			},
		},
	}
	podBytes, _ := json.Marshal(pod)

	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "12345",
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	admissionReviewBytes, _ := json.Marshal(admissionReview)
	req, err := http.NewRequest("POST", "/mutate", bytes.NewReader(admissionReviewBytes))
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decode the response
	var response admissionv1.AdmissionReview
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Could not decode response: %v", err)
	}

	// Check if the response is allowed
	if !response.Response.Allowed {
		t.Errorf("Expected response to be allowed")
	}

	// Check if the patch is correct
	expectedPatch := `[{"op":"replace","path":"/spec/containers/0/resources/requests/cpu","value":"100m"}]`
	if string(response.Response.Patch) != expectedPatch {
		t.Errorf("Unexpected patch: got %v want %v",
			string(response.Response.Patch), expectedPatch)
	}
}

func TestMutateHandlerNoMutationNeeded(t *testing.T) {
	// Set up the environment variable for the pod name
	os.Setenv("POD_NAME", "test-webhook-pod")
	defer os.Unsetenv("POD_NAME")

	// Create a sample AdmissionReview request
	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "nginx",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu": "50m",
						},
					},
				},
			},
		},
	}
	podBytes, _ := json.Marshal(pod)

	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "12345",
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	admissionReviewBytes, _ := json.Marshal(admissionReview)
	req, err := http.NewRequest("POST", "/mutate", bytes.NewReader(admissionReviewBytes))
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(mutateHandler)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decode the response
	var response admissionv1.AdmissionReview
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Could not decode response: %v", err)
	}

	// Check if the response is allowed
	if !response.Response.Allowed {
		t.Errorf("Expected response to be allowed")
	}

	// Check if there is no patch
	if response.Response.Patch != nil {
		t.Errorf("Unexpected patch: got %v want nil", string(response.Response.Patch))
	}
}
