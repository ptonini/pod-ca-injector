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
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"io/ioutil"
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
	err := readConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = fetchBundles()
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

func fetchBundles() error {
	config, err := getConfig()
	for k, v := range config.RootCA {
		var bundle string
		switch v.Type {
		case "url":
			log.Printf("Fetching bundle %s from %s", k, v.Source)
			bundle, err = getBundleFromURL(v.Source)
			if err != nil {
				return err
			}
		case "local":
			log.Printf("Fetching bundle %s from local config", k)
			if err = validateCertificate(v.Bundle); err == nil {
				bundle = v.Bundle
			} else {
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

func validateCertificate(bundle string) error {
	if strings.Contains(bundle, "-----BEGIN CERTIFICATE-----") {
		return nil
	} else {
		return fmt.Errorf("invalid certificate")
	}
}
