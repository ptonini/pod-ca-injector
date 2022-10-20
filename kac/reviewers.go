/*
 * Kubernetes Admission Controller.
 * Copyright (C) 2022 Pedro Tonini
 * mailto:pedro DOT tonini AT hotmail DOT com
 *
 * Kubernetes Admission Controller is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 3 of the License, or (at your option) any later version.
 *
 * Kubernetes Admission Controller is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with this program; if not, write to the Free Software Foundation,
 * Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
 */

package kac

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/wI2L/jsondiff"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
	keyPodNamespace = "POD_NAMESPACE"
)

var (
	podsGVR = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
	podGVK  = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
)

func createConfigMap(ctx context.Context, c kubernetes.Interface, ns string, bundleName string,
	config *Config) (*corev1.ConfigMap, error) {

	configMap, err := c.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: ns,
		},
		Data: map[string]string{
			bundleName: config.RootCA[bundleName].Bundle,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func validationReviewer(ctx context.Context, ar admissionv1.AdmissionReview) (*admissionv1.AdmissionResponse, error) {
	pt := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{Allowed: true, PatchType: &pt, Patch: []byte{}}, nil
}

func mutationReviewer(ctx context.Context, ar admissionv1.AdmissionReview) (*admissionv1.AdmissionResponse, error) {

	config, err := getConfig()
	if err != nil {
		return nil, err
	}

	// Deserialize and copy request object
	obj, err := validateAndDeserialize(ar, podsGVR, podGVK)
	if err != nil {
		return nil, err
	}
	pod := obj.(*corev1.Pod)
	newPod := pod.DeepCopy()

	namespace := ar.Request.Namespace

	if val, ok := pod.Annotations[config.Annotations.Inject]; ok {

		bundles := strings.Split(val, ",")
		log.Printf("Adding bundles %s to pod %s/%s%s", bundles, namespace, pod.GenerateName, pod.Name)

		clientSet, err := getKubernetesClientSet(ctx)
		if err != nil {
			return nil, err
		}

		for _, bundle := range bundles {

			configMap, _ := clientSet.CoreV1().ConfigMaps(namespace).Get(ctx, config.ConfigMapName, metav1.GetOptions{})

			// Create configmap if not found
			if configMap == nil || configMap.Name == "" {
				log.Printf("Creating bundles configmap on %s with %s", namespace, bundle)
				_, err = createConfigMap(ctx, clientSet, namespace, bundle, config)
			} else if configMap.Data[bundle] != config.RootCA[bundle].Bundle {
				log.Printf("Adding/Updating bundle %s on %s", bundle, namespace)
				configMap.Data[bundle] = config.RootCA[bundle].Bundle
				_, err = clientSet.CoreV1().ConfigMaps(fmt.Sprint(namespace)).Update(ctx, configMap, metav1.UpdateOptions{})
			}
			if err != nil {
				return nil, err
			}
		}

		// Add Volume to new pod
		log.Printf("Adding bundles volume to %s/%s%s", namespace, pod.GenerateName, pod.Name)
		newPod.Spec.Volumes = append(newPod.Spec.Volumes, corev1.Volume{
			Name: config.ConfigMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: config.ConfigMapName,
					},
				},
			},
		})

		for i := range newPod.Spec.Containers {
			for _, b := range bundles {
				log.Printf("Adding bundle %s volume mount to %s/%s%s", b, namespace, pod.GenerateName, pod.Name)
				newPod.Spec.Containers[i].VolumeMounts = append(newPod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      config.ConfigMapName,
					MountPath: fmt.Sprintf("/etc/ssl/certs/%s.pem", b),
					SubPath:   b,
				})
			}
		}
		newPod.ObjectMeta.Annotations[config.Annotations.Injected] = "true"
	}

	// Create mutation patch
	patch, _ := jsondiff.Compare(pod, newPod)
	encodedPatch, _ := json.Marshal(patch)

	// Return AdmissionReview object with AdmissionResponse
	pt := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{Allowed: true, PatchType: &pt, Patch: encodedPatch}, nil

}
