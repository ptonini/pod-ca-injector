package kac

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var (
	configMap = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
	}
	configMapsGVR = metav1.GroupVersionResource{
		Version:  "v1",
		Resource: "ConfigMaps",
	}
	pod = corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{},
			Containers: []corev1.Container{
				{
					VolumeMounts: []corev1.VolumeMount{},
				},
			},
		},
	}
)

func admissionReviewFactory(gvr metav1.GroupVersionResource, obj interface{}) string {
	rawObject, _ := json.Marshal(obj)
	a, err := json.Marshal(admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Request: &admissionv1.AdmissionRequest{
			Resource: gvr,
			Object: runtime.RawExtension{
				Raw: rawObject,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	return string(a)
}

func fakeRequest(ctx context.Context, r *gin.Engine, method string, route string, rawBody string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, route, strings.NewReader(rawBody))
	req = req.WithContext(ctx)
	r.ServeHTTP(w, req)
	return w
}

func Test_HealthcheckRoute(t *testing.T) {
	router := NewRouter()
	w := fakeRequest(context.Background(), router, http.MethodGet, "/health", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"status":"ok"}`, w.Body.String())
}

func Test_ReviewerRoutes(t *testing.T) {

	ctx := context.Background()
	router := NewRouter()

	_ = readConfig("../config.yaml")
	config, _ := getConfig()

	pod.Annotations = map[string]string{config.Annotations.Inject: "baltimore"}
	pod.Namespace = os.Getenv(keyPodNamespace)

	for _, route := range []string{"/mutate", "/validate"} {
		t.Run("test route "+route+" with nil body", func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, route, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
		t.Run("test route "+route+" with empty body", func(t *testing.T) {
			w := fakeRequest(ctx, router, http.MethodPost, route, "")
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
		t.Run("test route "+route+" with invalid body", func(t *testing.T) {
			invalidBody, _ := json.Marshal(configMap)
			w := fakeRequest(ctx, router, http.MethodPost, route, string(invalidBody))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}

	t.Run("test route /validate with valid request", func(t *testing.T) {
		body := admissionReviewFactory(podsGVR, pod)
		w := fakeRequest(ctx, router, http.MethodPost, "/validate", body)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("test route /mutate with invalid admission request resource", func(t *testing.T) {
		body := admissionReviewFactory(configMapsGVR, configMap)
		w := fakeRequest(ctx, router, http.MethodPost, "/mutate", body)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("test route /mutate with invalid admission request resource kind", func(t *testing.T) {
		body := admissionReviewFactory(podsGVR, configMap)
		w := fakeRequest(ctx, router, http.MethodPost, "/mutate", body)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("test route /mutate with valid request missing annotation", func(t *testing.T) {
		pod.Annotations = map[string]string{}
		defer func() { pod.Annotations = map[string]string{config.Annotations.Inject: "baltimore"} }()
		body := admissionReviewFactory(podsGVR, pod)
		w := fakeRequest(ctx, router, http.MethodPost, "/mutate", body)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("test route /mutate with valid request, no fake client", func(t *testing.T) {
		body := admissionReviewFactory(podsGVR, pod)
		w := fakeRequest(ctx, router, http.MethodPost, "/mutate", body)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	ctx = context.WithValue(ctx, keyFake, true)

	t.Run("test route /mutate with valid request missing namespace", func(t *testing.T) {
		pod.Namespace = ""
		defer func() { pod.Namespace = os.Getenv(keyPodNamespace) }()
		body := admissionReviewFactory(podsGVR, pod)
		w := fakeRequest(ctx, router, http.MethodPost, "/mutate", body)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("test route /mutate with valid request", func(t *testing.T) {
		body := admissionReviewFactory(podsGVR, pod)
		w := fakeRequest(ctx, router, http.MethodPost, "/mutate", body)
		assert.Equal(t, http.StatusOK, w.Code)
	})

}
