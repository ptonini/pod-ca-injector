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

package main

import (
	"flag"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"log"

	"github.com/ptonini/pod-ca-injector/kac"
)

func main() {
	var tlsKey, tlsCert, configFile string
	flag.StringVar(&tlsKey, "tlsKey", "/certs/tls.key", "Path to the TLS key")
	flag.StringVar(&tlsCert, "tlsCert", "/certs/tls.crt", "Path to the TLS certificate")
	flag.StringVar(&configFile, "configFile", "/config/config.yaml", "Path to the TLS certificate")
	flag.Parse()

	kac.LoadConfig(configFile)
	viper.OnConfigChange(func(in fsnotify.Event) {
		kac.LoadConfig(configFile)
	})
	viper.WatchConfig()

	log.Printf("Server started")
	router := kac.NewRouter()
	log.Fatal(router.RunTLS(":8443", tlsCert, tlsKey))
}
