package webhooks

import (
	"context"
	"encoding/json"
	"fmt"

	//testjsonpatch "github.com/evanphx/json-patch"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var logger = logf.Log.WithName("mutator")

// implement admission handler
type KataRuntimeBuilderPodMutationHandler struct {
	Client  client.Client
	decoder *admission.Decoder
}

// admission handler to make sure all application pods that are privileged are using the kata-containers runtime
func (v *KataRuntimeBuilderPodMutationHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	if err := json.Unmarshal(req.Object.Raw, pod); err != nil {
		fmt.Printf("failed to unmarshal request object to pod: %v\n", err)
		return admission.Allowed("this webhook is for pods")
	}

	// toPatch, _ := json.Marshal([]interface{}{
	// 	jsonpatch.NewOperation("add", "/spec/runtimeClassName", "kata"),
	// 	jsonpatch.NewOperation("remove", "/metadata/annotations/kubernetes.io~1limit-ranger", nil),
	// 	jsonpatch.NewOperation("remove", "/spec/containers/0/resources/limits/ephemeral-storage", nil),
	// })
	// patch, _ := testjsonpatch.DecodePatch(toPatch)
	// a, _ := patch.Apply(req.Object.Raw)

	logger.Info("got pod", "name", pod.Name, "namespace", pod.Namespace) //, "desired", string(a))

	if !mustRunAsKata(pod) {
		return admission.Allowed("this pod does not require kata-containers")
	}

	if !ownerIsBuild(pod) {
		return admission.Denied("this pod has higher permissions but is not a builder pod")
	}

	patches := createJSONPatches(pod)

	return admission.Patched("this pod must run as kata", patches...)

	//return admission.Patched("", jsonpatch.NewOperation("add", "/spec/runtimeClassName", "kata"))
}

func createJSONPatches(pod *corev1.Pod) []jsonpatch.Operation {
	patches := []jsonpatch.Operation{
		jsonpatch.NewOperation("add", "/spec/runtimeClassName", "kata"),
	}

	for i := range pod.Spec.Volumes {
		if pod.Spec.Volumes[i].EmptyDir != nil {
			patches = append(patches, jsonpatch.NewOperation("add", fmt.Sprintf("/spec/volumes/%d/emptyDir/medium", i), "Memory"))
		}
	}

	return patches
}

// func mustRunAsKata(pod *corev1.Pod) bool {
// 	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil && *pod.Spec.SecurityContext.RunAsUser == 0 {
// 		return true
// 	}

// 	for i := range pod.Spec.Containers {
// 		if (pod.Spec.Containers[i].SecurityContext != nil) &&
// 			((pod.Spec.Containers[i].SecurityContext.Privileged != nil && *pod.Spec.Containers[i].SecurityContext.Privileged == true) ||
// 				(pod.Spec.Containers[i].SecurityContext.RunAsUser != nil && *pod.Spec.Containers[i].SecurityContext.RunAsUser == 0)) {
// 			return true
// 		}
// 	}

// 	for i := range pod.Spec.InitContainers {
// 		if (pod.Spec.InitContainers[i].SecurityContext != nil) &&
// 			((pod.Spec.InitContainers[i].SecurityContext.Privileged != nil && *pod.Spec.InitContainers[i].SecurityContext.Privileged == true) ||
// 				(pod.Spec.InitContainers[i].SecurityContext.RunAsUser != nil && *pod.Spec.InitContainers[i].SecurityContext.RunAsUser == 0)) {
// 			return true
// 		}
// 	}

// 	for i := range pod.Spec.EphemeralContainers {
// 		if (pod.Spec.EphemeralContainers[i].SecurityContext != nil) &&
// 			((pod.Spec.EphemeralContainers[i].SecurityContext.Privileged != nil && *pod.Spec.EphemeralContainers[i].SecurityContext.Privileged == true) ||
// 				(pod.Spec.EphemeralContainers[i].SecurityContext.RunAsUser != nil && *pod.Spec.EphemeralContainers[i].SecurityContext.RunAsUser == 0)) {
// 			return true
// 		}
// 	}

// 	return false
// }

func ownerIsBuild(pod *corev1.Pod) bool {
	for i := range pod.OwnerReferences {
		if pod.OwnerReferences[i].Kind == "Build" {
			return true
		}
	}
	return false
}

func (v *KataRuntimeBuilderPodMutationHandler) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
