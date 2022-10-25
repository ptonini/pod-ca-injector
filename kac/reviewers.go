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

var (
	podsGVR = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
	podGVK  = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
)

func upsertConfigMap(ctx context.Context, config *Config, c kubernetes.Interface, ns string, bundle string) error {

	var err error
	configMap, _ := c.CoreV1().ConfigMaps(ns).Get(ctx, config.ConfigMapName, metav1.GetOptions{})

	if configMap == nil || configMap.Name == "" {
		log.Printf("Creating bundles configmap on %s", ns)
		configMap, err = c.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.ConfigMapName,
				Namespace: ns,
			},
			Data: map[string]string{
				bundle: config.Bundles[bundle],
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	if configMap.Data[bundle] != config.Bundles[bundle] {
		log.Printf("Adding/Updating bundle %s on %s", bundle, ns)
		configMap.Data[bundle] = config.Bundles[bundle]
		_, err = c.CoreV1().ConfigMaps(fmt.Sprint(ns)).Update(ctx, configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func mutatePod(pod *corev1.Pod, config *Config, ns string, bundles []string) {

	// Add Volume to new pod
	log.Printf("Adding bundles volume to %s/%s%s", ns, pod.GenerateName, pod.Name)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: config.ConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: config.ConfigMapName,
				},
			},
		},
	})

	for i := range pod.Spec.Containers {
		for _, b := range bundles {
			log.Printf("Adding bundle %s volume mount to %s/%s%s", b, ns, pod.GenerateName, pod.Name)
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      config.ConfigMapName,
				MountPath: fmt.Sprintf("/etc/ssl/certs/%s.pem", b),
				SubPath:   b,
			})
		}
	}
	pod.ObjectMeta.Annotations[config.Annotations.Injected] = "true"
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
	namespace := ar.Request.Namespace
	obj, err := validateAndDeserialize(ar, podsGVR, podGVK)
	if err != nil {
		return nil, err
	}
	pod := obj.(*corev1.Pod)
	newPod := pod.DeepCopy()

	if val, ok := pod.Annotations[config.Annotations.Inject]; ok {
		bundles := strings.Split(val, ",")
		log.Printf("Adding bundles %s to pod %s/%s%s", bundles, namespace, pod.GenerateName, pod.Name)
		clientSet, err := getKubernetesClientSet(ctx)
		if err != nil {
			return nil, err
		}
		for _, bundle := range bundles {
			err = upsertConfigMap(ctx, config, clientSet, namespace, bundle)
			if err != nil {
				return nil, err
			}
		}
		mutatePod(newPod, config, namespace, bundles)
	}

	// Create mutation patch
	patch, _ := jsondiff.Compare(pod, newPod)
	encodedPatch, _ := json.Marshal(patch)

	// Return AdmissionReview object with AdmissionResponse
	pt := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{Allowed: true, PatchType: &pt, Patch: encodedPatch}, nil

}
