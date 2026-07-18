/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterNacos

import (
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/spring/gs"
)

func init() {

	// Register a naming client (service discovery) and a config client
	// (configuration management). Both are created only when the property
	// "spring.nacos.ip-addr" is set, using the "${spring.nacos}" configuration.
	// They are distinct interface types, so both keep the default name.
	gs.Provide(newNamingClient, gs.TagArg("${spring.nacos}")).
		Condition(gs.OnProperty("spring.nacos.ip-addr")).
		Destroy(destroyNamingClient)

	gs.Provide(newConfigClient, gs.TagArg("${spring.nacos}")).
		Condition(gs.OnProperty("spring.nacos.ip-addr")).
		Destroy(destroyConfigClient)

	// Register multiple naming clients as a group.
	// Each instance is created according to the configuration in "${spring.nacos.instances}".
	gs.Group("${spring.nacos.instances}", newNamingClient, destroyNamingClient)

	// Register the refresh bridge as a root object so it is always created.
	// It links the "nacos" remote config provider's change listener to the
	// application's property refresh, enabling hot-reload of bound beans.
	gs.Provide(newConfigRefreshBridge).Export(gs.As[gs.Rooter]())
}

// configRefreshBridge connects remote Nacos config changes to the
// application-wide property refresh mechanism.
type configRefreshBridge struct{}

// newConfigRefreshBridge installs the refresh hook used by the "nacos" config
// provider. It injects the framework's PropertiesRefresher so that a remote
// config change reloads all sources and updates bound gs.Dync fields.
func newConfigRefreshBridge(r *gs.PropertiesRefresher) *configRefreshBridge {
	setRefreshHook(r.RefreshProperties)
	return &configRefreshBridge{}
}

// buildParam converts Config into the parameter object expected by the SDK.
func buildParam(c Config) vo.NacosClientParam {
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(c.IpAddr, c.Port),
	}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId(c.Namespace),
		constant.WithTimeoutMs(c.TimeoutMs),
		constant.WithUsername(c.Username),
		constant.WithPassword(c.Password),
		constant.WithLogLevel(c.LogLevel),
		constant.WithLogDir(c.LogDir),
		constant.WithCacheDir(c.CacheDir),
		constant.WithNotLoadCacheAtStart(true),
	)
	return vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc}
}

// newNamingClient creates a Nacos naming (service discovery) client.
func newNamingClient(c Config) (naming_client.INamingClient, error) {
	return clients.NewNamingClient(buildParam(c))
}

// destroyNamingClient closes the naming client.
func destroyNamingClient(client naming_client.INamingClient) error {
	client.CloseClient()
	return nil
}

// newConfigClient creates a Nacos config (configuration management) client.
func newConfigClient(c Config) (config_client.IConfigClient, error) {
	return clients.NewConfigClient(buildParam(c))
}

// destroyConfigClient closes the config client.
func destroyConfigClient(client config_client.IConfigClient) error {
	client.CloseClient()
	return nil
}
