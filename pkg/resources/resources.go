package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qdnqnv1 "clientmgr.io/tutorial/api/v1"
)

func getLabels(clientResource *qdnqnv1.Client) map[string]string {
	return map[string]string{
		"app":     clientResource.Spec.ContainerImage,
		"version": clientResource.Spec.ContainerTag,
	}
}

func CreatePod(clientResource *qdnqnv1.Client) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clientResource.Spec.ContainerImage + clientResource.Spec.ContainerTag,
			Namespace: clientResource.Namespace,
			Labels:    getLabels(clientResource),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  clientResource.Spec.ContainerImage,
					Image: clientResource.Spec.ContainerImage + ":" + clientResource.Spec.ContainerTag,
					Env: []corev1.EnvVar{
						{
							Name:  "CLIENT_ID",
							Value: clientResource.Spec.ClientId,
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
}
