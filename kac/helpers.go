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
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"net/http"
)

const (
	keyFakeClientSet = "fakeClientSet"
	keyFakeObjects   = "fakeObjects"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecFactory  = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecFactory.UniversalDeserializer()
)

type AdmissionReviewer func(context.Context, admissionv1.AdmissionReview) (*admissionv1.AdmissionResponse, error)

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionv1.AddToScheme(runtimeScheme)
}

func errorResponse(c *gin.Context, statusCode int, err error) {
	_ = c.Error(err)
	c.JSON(statusCode, gin.H{"error": err.Error()})
}

func validateAndDeserialize(ar admissionv1.AdmissionReview, expectedGVR metav1.GroupVersionResource,
	expectedGVK schema.GroupVersionKind) (runtime.Object, error) {
	// Validate resource type
	if ar.Request.Resource != expectedGVR {
		return nil, fmt.Errorf("expect resource to be %s, got %s", expectedGVR, ar.Request.Resource)
	}
	// Deserialize object from AdmissionRequest
	obj, gvk, err := deserializer.Decode(ar.Request.Object.Raw, nil, nil)
	if *gvk != expectedGVK {
		return nil, fmt.Errorf("deserialized object is invalid: %v", obj)
	} else if err != nil {
		return nil, err
	} else {
		return obj, nil
	}
}

func getKubernetesClientSet(ctx context.Context) (kubernetes.Interface, error) {
	if ctx.Value(keyFakeClientSet) != nil && ctx.Value(keyFakeClientSet).(bool) {
		var objList []runtime.Object
		if ctx.Value(keyFakeObjects) != nil {
			objList = ctx.Value(keyFakeObjects).([]runtime.Object)
		}
		c := fake.NewSimpleClientset(objList...)
		return c, nil
	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return kubernetes.NewForConfig(config)
	}

}

func serve(c *gin.Context, admissionReviewer AdmissionReviewer) {

	var resp *admissionv1.AdmissionReview
	var body []byte
	var ctx = c.Request.Context()

	if c.Request.Body != nil {
		body, _ = io.ReadAll(c.Request.Body)
	} else {
		errorResponse(c, http.StatusBadRequest, fmt.Errorf("request body is empty"))
		return
	}

	if obj, gvk, err := deserializer.Decode(body, nil, nil); err != nil {
		errorResponse(c, http.StatusBadRequest, err)
		return
	} else {
		req, ok := obj.(*admissionv1.AdmissionReview)
		if !ok {
			errorResponse(c, http.StatusBadRequest, fmt.Errorf("expected v1.AdmissionReview but got: %T", obj))
			return
		}
		resp = &admissionv1.AdmissionReview{}
		resp.SetGroupVersionKind(*gvk)
		resp.Response, err = admissionReviewer(ctx, *req)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, err)
			return
		}
		resp.Response.UID = req.Request.UID

	}

	c.JSON(http.StatusOK, resp)

}
