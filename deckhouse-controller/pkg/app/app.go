// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	AppName        = "deckhouse"
	AppDescription = ""
)

var (
	PodName       = ""
	ContainerName = "deckhouse"
)

var (
	FeatureWatchRegistry               = "yes"
	InsecureRegistry                   = "no"
	SkipTLSVerifyRegistry              = "no"
	RegistrySecretPath                 = "/etc/registrysecret"
	RegistryErrorsMaxTimeBeforeRestart = time.Hour
)

const (
	DeckhouseLogTypeDefault         = "json"
	DeckhouseKubeClientQPSDefault   = "20"
	DeckhouseKubeClientBurstDefault = "40"

	DeckhouseHookMetricsListenPort = "9651"
)

func DefineStartCommandFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("pod-name", "Pod name to get image digest.").
		Envar("DECKHOUSE_POD").
		Required().
		StringVar(&PodName)
	cmd.Flag("feature-watch-registry", "Enable docker registry watcher (yes|no).").
		Envar("DECKHOUSE_WATCH_REGISTRY").
		Default(FeatureWatchRegistry).
		StringVar(&FeatureWatchRegistry)
	cmd.Flag("insecure-registry", "Use http to access registry (yes|no).").
		Envar("DECKHOUSE_INSECURE_REGISTRY").
		Default(InsecureRegistry).
		StringVar(&InsecureRegistry)
	cmd.Flag("skip-tls-verify-registry", "Trust self signed certificate of registry (yes|no).").
		Envar("DECKHOUSE_SKIP_TLS_VERIFY_REGISTRY").
		Default(SkipTLSVerifyRegistry).
		StringVar(&SkipTLSVerifyRegistry)
}
