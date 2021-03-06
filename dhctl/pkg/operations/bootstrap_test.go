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

package operations

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestBootstrapGetNodesFromCache(t *testing.T) {
	log.InitLogger("simple")
	dir, err := ioutil.TempDir(os.TempDir(), "dhctl-test-bootstrap-*")
	defer os.RemoveAll(dir)

	require.NoError(t, err)

	for _, name := range []string{
		"base-infrastructure.tfstate",
		"some_trash",
		"test-master-0.tfstate",
		"test-master-1.tfstate",
		"test-master-without-index.tfstate",
		"test-master-1.tfstate.backup",
		"uuid.tfstate",
		"test-static-ingress-0.tfstate",
	} {
		_, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
	}

	t.Run("Should get only nodes state from cache", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(dir)
		require.NoError(t, err)

		result, err := BootstrapGetNodesFromCache(&config.MetaConfig{ClusterPrefix: "test"}, stateCache)
		require.NoError(t, err)

		require.Len(t, result["master"], 2)
		require.Len(t, result["static-ingress"], 1)

		require.Equal(t, "test-master-0", result["master"][0])
		require.Equal(t, "test-master-1", result["master"][1])

		require.Equal(t, "test-static-ingress-0", result["static-ingress"][0])
	})
}
