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
		Bundle string
	}
}

func LoadConfig(configFile string) {
	log.Printf("Loding config")
	ctx := context.Background()
	err := readConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = fetchBundles(ctx)
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
	viper.SetEnvPrefix("CA_INJECTOR")
	viper.AutomaticEnv()
	return viper.ReadInConfig()
}

func fetchBundles(ctx context.Context) error {
	config, err := getConfig()
	for k, v := range config.RootCA {
		var bundle string
		log.Printf("Fetching bundle %s from %s: %s", k, v.Type, v.Source)
		switch v.Type {
		case "local":
			if err = validateCertificate(v.Bundle); err == nil {
				bundle = v.Bundle
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
		viper.Set(fmt.Sprintf(`rootCA.%s.Bundle`, k), bundle)
	}
	return err
}

func getConfig() (*Config, error) {
	var config Config
	err := viper.Unmarshal(&config)
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
	secret, _ := clientSet.CoreV1().Secrets(ns).Get(ctx, secretName, v1.GetOptions{})
	certificate := base64.StdEncoding.EncodeToString(secret.Data[key])
	return certificate, validateCertificate(certificate)
}

func getCAFromConfigMap(ctx context.Context, ns string, configMapName string, key string) (string, error) {
	clientSet, err := getKubernetesClientSet(ctx)
	if err != nil {
		return "", err
	}
	configMap, _ := clientSet.CoreV1().ConfigMaps(ns).Get(ctx, configMapName, v1.GetOptions{})
	certificate := configMap.Data[key]
	return certificate, validateCertificate(certificate)
}

func validateCertificate(bundle string) error {
	if strings.Contains(bundle, "-----BEGIN CERTIFICATE-----") {
		return nil
	} else {
		return fmt.Errorf("invalid certificate")
	}
}
