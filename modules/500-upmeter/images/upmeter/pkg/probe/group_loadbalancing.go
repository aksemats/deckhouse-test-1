/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package probe

import (
	"time"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func initLoadBalancing(access kubernetes.Access) []runnerConfig {
	const (
		groupName = "load-balancing"
		cpTimeout = 5 * time.Second
	)

	return []runnerConfig{
		{
			group:  groupName,
			probe:  "load-balancer-configuration",
			check:  "cloud-controller-manager",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:                    access,
				Timeout:                   5 * time.Second,
				Namespace:                 access.CloudControllerManagerNamespace(),
				LabelSelector:             "app=cloud-controller-manager",
				ControlPlaneAccessTimeout: cpTimeout,
			},
		}, {
			group:  groupName,
			probe:  "metallb",
			check:  "controller",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:                    access,
				Timeout:                   5 * time.Second,
				Namespace:                 "d8-metallb",
				LabelSelector:             "app=controller",
				ControlPlaneAccessTimeout: cpTimeout,
			},
		}, {
			group:  groupName,
			probe:  "metallb",
			check:  "speaker",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:                    access,
				Timeout:                   5 * time.Second,
				Namespace:                 "d8-metallb",
				LabelSelector:             "app=speaker",
				ControlPlaneAccessTimeout: cpTimeout,
			},
		},
	}
}
