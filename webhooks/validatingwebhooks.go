package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var podValidatorLog = logf.Log.WithName("pod")

// implement admission handler
type PodValidationHandler struct {
	Client  client.Client
	decoder *admission.Decoder
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func getValidKataImages() []string {
	var images []string

	validKataImagesJson := getEnv("VALID_KATA_IMAGES", "[]")

	err := json.Unmarshal([]byte(validKataImagesJson), &images)
	if err != nil {
		fmt.Printf("an error occurred while unmarshaling the valid kata images: %v\n", err)
		os.Exit(1)
	}
	return images
}

var validKataImages = getValidKataImages()

// admission handler to make sure all application pods that are privileged are using the kata-containers runtime
func (v *PodValidationHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	if err := json.Unmarshal(req.Object.Raw, pod); err != nil {
		fmt.Printf("failed to unmarshal request object to pod: %v\n", err)
		return admission.Allowed("this webhook only validates pods")
	}
	podValidatorLog.Info("validating pod", "name", pod.Name, "namespace", pod.Namespace)

	if !mustRunAsKata(pod) {
		return admission.Allowed("this pod does not require kata-containers")
	}

	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata" {
		return admission.Denied("this pod must run using kata-containers")
	}

	if !validImages(pod) {
		return admission.Denied("this pod must only use images that paas team allows")
	}

	return admission.Allowed("this pod is valid")
}

func containsString(arr []string, str string) bool {
	for _, item := range arr {
		if item == str {
			return true
		}
	}
	return false
}

func validImages(pod *corev1.Pod) bool {
	for i := range pod.Spec.Containers {
		if !containsString(validKataImages, pod.Spec.Containers[i].Image) {
			return false
		}
	}

	for i := range pod.Spec.InitContainers {
		if !containsString(validKataImages, pod.Spec.InitContainers[i].Image) {
			return false
		}
	}

	for i := range pod.Spec.EphemeralContainers {
		if !containsString(validKataImages, pod.Spec.EphemeralContainers[i].Image) {
			return false
		}
	}

	return true
}

func mustRunAsKata(pod *corev1.Pod) bool {
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil && *pod.Spec.SecurityContext.RunAsUser == 0 {
		return true
	}

	for i := range pod.Spec.Containers {
		if (pod.Spec.Containers[i].SecurityContext != nil) &&
			((pod.Spec.Containers[i].SecurityContext.Privileged != nil && *pod.Spec.Containers[i].SecurityContext.Privileged == true) ||
				(pod.Spec.Containers[i].SecurityContext.RunAsUser != nil && *pod.Spec.Containers[i].SecurityContext.RunAsUser == 0)) {
			return true
		}
	}

	for i := range pod.Spec.InitContainers {
		if (pod.Spec.InitContainers[i].SecurityContext != nil) &&
			((pod.Spec.InitContainers[i].SecurityContext.Privileged != nil && *pod.Spec.InitContainers[i].SecurityContext.Privileged == true) ||
				(pod.Spec.InitContainers[i].SecurityContext.RunAsUser != nil && *pod.Spec.InitContainers[i].SecurityContext.RunAsUser == 0)) {
			return true
		}
	}

	for i := range pod.Spec.EphemeralContainers {
		if (pod.Spec.EphemeralContainers[i].SecurityContext != nil) &&
			((pod.Spec.EphemeralContainers[i].SecurityContext.Privileged != nil && *pod.Spec.EphemeralContainers[i].SecurityContext.Privileged == true) ||
				(pod.Spec.EphemeralContainers[i].SecurityContext.RunAsUser != nil && *pod.Spec.EphemeralContainers[i].SecurityContext.RunAsUser == 0)) {
			return true
		}
	}

	return false
}

func (v *PodValidationHandler) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
