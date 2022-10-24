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
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

type Config struct {
	Annotations struct {
		Inject   string
		Injected string
	}
	ConfigMapName string
	RootCA        map[string]struct {
		Type   string
		Source string
	}
	Bundles map[string]string
}

func LoadConfig(configFile string) {
	log.Printf("Loding config")
	err := readConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = fetchBundles(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func StartConfigWatch(configFile string) {
	viper.OnConfigChange(func(in fsnotify.Event) {
		LoadConfig(configFile)
	})
	viper.WatchConfig()
}

func readConfig(configFile string) error {
	viper.SetConfigName(strings.Split(filepath.Base(configFile), ".")[0])
	viper.AddConfigPath(filepath.Dir(configFile))
	viper.MustBindEnv("configmapname", "CA_INJECTOR_CONFIGMAP_NAME")
	viper.MustBindEnv("annotations.inject", "CA_INJECTOR_ANNOTATIONS_INJECT")
	viper.MustBindEnv("annotations.injected", "CA_INJECTOR_ANNOTATIONS_INJECTED")
	viper.MustBindEnv("rootca", "CA_INJECTOR_ROOTCA")
	return viper.ReadInConfig()
}

func fetchBundles(ctx context.Context) error {
	config, err := getConfig()
	for k, v := range config.RootCA {
		var bundle string
		log.Printf("Fetching bundle %s from %s: %q", k, v.Type, v.Source)
		switch v.Type {
		case "local":
			if err = validateCertificate(v.Source); err == nil {
				bundle = v.Source
			} else {
				return err
			}
		case "url":
			bundle, err = getBundleFromURL(v.Source)
			if err != nil {
				return err
			}
		case "secret":
			s := strings.Split(v.Source, "/")
			bundle, err = getCAFromSecret(ctx, s[0], s[1], s[2])
			if err != nil {
				return err
			}
		case "configMap":
			s := strings.Split(v.Source, "/")
			bundle, err = getCAFromConfigMap(ctx, s[0], s[1], s[2])
			if err != nil {
				return err
			}
		}
		viper.Set(fmt.Sprintf(`bundles.%s`, k), bundle)
	}
	return err
}

func getConfig() (*Config, error) {
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		err = json.Unmarshal([]byte(viper.Get("rootca").(string)), &config.RootCA)
	}
	return &config, err
}

func getBundleFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	body, _ := ioutil.ReadAll(resp.Body)
	defer func() { _ = resp.Body.Close() }()
	bodyStr := string(body)
	return bodyStr, validateCertificate(bodyStr)
}

func getCAFromSecret(ctx context.Context, ns string, secretName string, key string) (string, error) {
	clientSet, err := getKubernetesClientSet(ctx)
	if err != nil {
		return "", err
	}
	secret, err := clientSet.CoreV1().Secrets(ns).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return "", err
	}
	certificateBytes, err := base64.StdEncoding.DecodeString(string(secret.Data[key]))
	if err != nil {
		return "", err
	}
	certificate := string(certificateBytes)
	return certificate, validateCertificate(certificate)
}

func getCAFromConfigMap(ctx context.Context, ns string, configMapName string, key string) (string, error) {
	clientSet, err := getKubernetesClientSet(ctx)
	if err != nil {
		return "", err
	}
	configMap, err := clientSet.CoreV1().ConfigMaps(ns).Get(ctx, configMapName, v1.GetOptions{})
	if err != nil {
		return "", err
	}
	certificate := configMap.Data[key]
	return certificate, validateCertificate(certificate)
}

func validateCertificate(bundle string) error {
	block, _ := pem.Decode([]byte(bundle))
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("invalid certificate")
	}
	return nil
}
