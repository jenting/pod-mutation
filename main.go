package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	successCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pod_mutation_success_total",
			Help: "Total number of successful pod mutations",
		},
		[]string{"pod_mutation_name", "namespace", "pod"},
	)
	failureCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pod_mutation_failure_total",
			Help: "Total number of failed pod mutations",
		},
		[]string{"pod_mutation_name", "namespace", "pod"},
	)
	podMutationName string
)

func init() {
	prometheus.MustRegister(successCounter)
	prometheus.MustRegister(failureCounter)

	// Get the pod name from the environment variable
	podMutationName = os.Getenv("POD_NAME")
	if podMutationName == "" {
		podMutationName = "unknown"
	}
}

func mutateHandler(w http.ResponseWriter, r *http.Request) {
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		http.Error(w, "could not decode request", http.StatusBadRequest)
		return
	}

	admissionResponse := &admissionv1.AdmissionResponse{
		UID: admissionReview.Request.UID,
	}

	pod := admissionReview.Request.Object.Raw
	var podObject map[string]interface{}
	if err := json.Unmarshal(pod, &podObject); err != nil {
		failureCounter.WithLabelValues(podMutationName, admissionReview.Request.Namespace, admissionReview.Request.Name).Inc()
		admissionResponse.Allowed = false
		admissionResponse.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else {
		containers := podObject["spec"].(map[string]interface{})["containers"].([]interface{})
		var patches []map[string]interface{}
		for i, container := range containers {
			containerMap := container.(map[string]interface{})
			if containerMap["name"] != "istio-proxy" {
				resources := containerMap["resources"].(map[string]interface{})
				requests := resources["requests"].(map[string]interface{})
				cpu := requests["cpu"].(string)
				cpuInt, err := strconv.Atoi(cpu[:len(cpu)-1])
				if err != nil {
					log.Printf("invalid CPU format: %s %s/%s", cpu, admissionReview.Request.Name, admissionReview.Request.Namespace)
					continue
				}
				if cpuInt > 100 {
					patch := map[string]interface{}{
						"op":    "replace",
						"path":  fmt.Sprintf("/spec/containers/%d/resources/requests/cpu", i),
						"value": "100m",
					}
					patches = append(patches, patch)
				}
			}
		}

		if len(patches) > 0 {
			patchBytes, err := json.Marshal(patches)
			if err != nil {
				failureCounter.WithLabelValues(podMutationName, admissionReview.Request.Namespace, admissionReview.Request.Name).Inc()
				admissionResponse.Allowed = false
				admissionResponse.Result = &metav1.Status{
					Message: err.Error(),
				}
			} else {
				successCounter.WithLabelValues(podMutationName, admissionReview.Request.Namespace, admissionReview.Request.Name).Inc()
				admissionResponse.Allowed = true
				admissionResponse.Patch = patchBytes
				patchType := admissionv1.PatchTypeJSONPatch
				admissionResponse.PatchType = &patchType
			}
		} else {
			successCounter.WithLabelValues(podMutationName, admissionReview.Request.Namespace, admissionReview.Request.Name).Inc()
			admissionResponse.Allowed = true
		}
	}

	admissionReview.Response = admissionResponse
	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

func main() {
	http.HandleFunc("/mutate", mutateHandler)
	http.HandleFunc("/metrics", metricsHandler)

	certFile := "/etc/webhook/certs/tls.crt"
	keyFile := "/etc/webhook/certs/tls.key"

	log.Fatal(http.ListenAndServeTLS(":443", certFile, keyFile, nil))
}
