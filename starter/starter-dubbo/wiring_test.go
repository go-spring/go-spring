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

package StarterDubbo

import (
	"strings"
	"testing"

	"dubbo.apache.org/dubbo-go/v3/client"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"
)

// KNOWN BUG (recorded, not fixed here): DubboProtocol.Params is
// map[string]any, which the conf binder rejects ("target should be a value
// type"), so ANY spring.dubbo.protocols.<id>.* property makes the whole
// ${spring.dubbo} bind fail and the Instance bean silently drops out. The
// protocols block is therefore NOT covered by these binding tests; only the
// pure DubboProtocol.options translation is.

// setDubboProps sets the minimal properties needed to assemble the Instance
// bean offline: a registry pointing at an unreachable local port, tracing off
// (dubbo-go panics on the default stdout exporter without an OTel provider),
// and the provider server off (SimpleDubboServer would otherwise Run and try
// to serve against the dead registry).
func setDubboProps(g gs.App, extra ...[2]string) {
	g.Property("spring.dubbo.application.name", "wiring-test-app")
	g.Property("spring.dubbo.registries.demo.protocol", "zookeeper")
	g.Property("spring.dubbo.registries.demo.address", "zookeeper://127.0.0.1:1")
	g.Property("spring.dubbo.tracing.enable", "false")
	g.Property("spring.dubbo.provider.enabled", "false")
	for _, kv := range extra {
		g.Property(kv[0], kv[1])
	}
}

// --- NewInstance validation (pure, no dubbo engine) ---

func TestNewInstance_RequiresAppName(t *testing.T) {
	_, err := NewInstance(DubboConfig{
		Registries: map[string]DubboRegistry{"demo": {}},
	})
	if err == nil || !strings.Contains(err.Error(), "${spring.dubbo.application.name} is required") {
		t.Fatalf("want missing app name error, got %v", err)
	}
}

func TestNewInstance_RequiresRegistry(t *testing.T) {
	_, err := NewInstance(DubboConfig{Application: DubboApplication{Name: "x"}})
	if err == nil || !strings.Contains(err.Error(), "${spring.dubbo.registries} must define at least one registry") {
		t.Fatalf("want missing registries error, got %v", err)
	}
}

// --- selectRegistries (pure) ---

func TestSelectRegistries_EmptyIDsSelectsAll(t *testing.T) {
	all := map[string]DubboRegistry{"a": {Protocol: "nacos"}, "b": {Protocol: "zookeeper"}}
	got, err := selectRegistries(all, nil)
	if err != nil || len(got) != 2 {
		t.Fatalf("empty ids should select all: got=%v err=%v", got, err)
	}
}

func TestSelectRegistries_UnknownIDFails(t *testing.T) {
	all := map[string]DubboRegistry{"a": {}}
	_, err := selectRegistries(all, []string{"nope"})
	if err == nil || !strings.Contains(err.Error(), `registry id "nope" is not defined`) {
		t.Fatalf("want unknown registry id error, got %v", err)
	}
}

// --- pure option translation ---

func TestDubboProtocol_OptionsFallbackName(t *testing.T) {
	// Empty Name falls back to the protocol ID (the map key).
	opts := DubboProtocol{Port: 20000}.options("tri")
	if len(opts) < 3 {
		t.Fatalf("fallback protocol should still emit id/name/port options, got %d", len(opts))
	}
}

func TestDubboProvider_BuildOptions_FallbackProtocolAndRegistryError(t *testing.T) {
	p := DubboProvider{}
	opts, err := p.buildOptions(nil, map[string]DubboRegistry{"demo": {Protocol: "zookeeper"}})
	if err != nil || len(opts) == 0 {
		t.Fatalf("no protocols should inject the tri:20000 fallback, opts=%d err=%v", len(opts), err)
	}

	_, err = (&DubboProvider{RegistryIDs: []string{"missing"}}).
		buildOptions(nil, map[string]DubboRegistry{"demo": {}})
	if err == nil || !strings.Contains(err.Error(), `registry id "missing" is not defined`) {
		t.Fatalf("provider registry-ids should fail fast on unknown id, got %v", err)
	}
}

// --- DubboConfig binding through the real assembly path ---

func TestWiring_DubboConfigBinding(t *testing.T) {
	gs.Web(false).Configure(func(g gs.App) {
		setDubboProps(g,
			[2]string{"spring.dubbo.application.organization", "my-org"},
			[2]string{"spring.dubbo.application.version", "1.2.3"},
			[2]string{"spring.dubbo.registries.demo.namespace", "ns1"},
			[2]string{"spring.dubbo.registries.demo.group", "g1"},
			[2]string{"spring.dubbo.registries.demo.ttl", "30s"},
			[2]string{"spring.dubbo.registries.demo.weight", "50"},
			[2]string{"spring.dubbo.registries.demo.username", "u"},
			[2]string{"spring.dubbo.registries.demo.password", "p"},
			[2]string{"spring.dubbo.registries.demo.params.extra", "v"},
			[2]string{"spring.dubbo.consumer.protocol", "tri"},
			[2]string{"spring.dubbo.consumer.request-timeout", "3s"},
			[2]string{"spring.dubbo.consumer.check", "false"},
			[2]string{"spring.dubbo.consumer.retries", "2"},
			[2]string{"spring.dubbo.consumer.registry-ids", "demo"},
			[2]string{"spring.dubbo.consumer.references.greet.interface", "greet.GreetService"},
			[2]string{"spring.dubbo.consumer.references.greet.timeout", "500ms"},
			[2]string{"spring.dubbo.consumer.references.greet.retries", "3"},
			[2]string{"spring.dubbo.provider.services.echo.interface", "echo.EchoService"},
			[2]string{"spring.dubbo.provider.services.echo.cluster", "failfast"},
		)
	}).RunTest(t, func(s *struct {
		Ins *Instance `autowire:""`
	}) {
		if s.Ins == nil {
			t.Fatal("Instance bean should exist when spring.dubbo.registries is configured")
		}
		cfg := s.Ins.Config

		// application block + tag defaults.
		if cfg.Application.Name != "wiring-test-app" || cfg.Application.Organization != "my-org" ||
			cfg.Application.Version != "1.2.3" || cfg.Application.MetadataType != "local" {
			t.Fatalf("application bound wrong: %+v", cfg.Application)
		}

		// registries block: explicit values plus tag defaults.
		reg := cfg.Registries["demo"]
		if reg.Protocol != "zookeeper" || reg.Address != "zookeeper://127.0.0.1:1" ||
			reg.Namespace != "ns1" || reg.Group != "g1" || reg.Username != "u" || reg.Password != "p" {
			t.Fatalf("registry bound wrong: %+v", reg)
		}
		if reg.Timeout != "5s" || reg.TTL != "30s" || reg.Weight != 50 {
			t.Fatalf("registry defaults bound wrong: timeout=%q ttl=%q weight=%d", reg.Timeout, reg.TTL, reg.Weight)
		}
		if reg.Params["extra"] != "v" {
			t.Fatalf("registry params bound wrong: %v", reg.Params)
		}

		// metadata-report is dead config and was removed: nothing to bind.

		// consumer block and nested references.
		c := cfg.Consumer
		if c.Protocol != "tri" || c.RequestTimeout != "3s" || c.Check || c.Retries != 2 {
			t.Fatalf("consumer bound wrong: %+v", c)
		}
		ref, ok := c.References["greet"]
		if !ok || ref.Interface != "greet.GreetService" || ref.Timeout != "500ms" || ref.Retries != 3 {
			t.Fatalf("reference greet bound wrong: %+v (ok=%v)", ref, ok)
		}
		if ref.Cluster != "failover" || ref.LoadBalance != "random" || !ref.Check {
			t.Fatalf("reference tag defaults wrong: %+v", ref)
		}

		// provider.services block. retries default is -1 (unset sentinel).
		svc, ok := cfg.Provider.Services["echo"]
		if !ok || svc.Interface != "echo.EchoService" || svc.Cluster != "failfast" || svc.Retries != -1 {
			t.Fatalf("service echo bound wrong: %+v (ok=%v)", svc, ok)
		}
	})
}

// --- assembly conditions ---

func TestWiring_NoConfig_NoBeans(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Ins *Instance      `autowire:"?"`
		Cli *client.Client `autowire:"?"`
	}) {
		if s.Ins != nil {
			t.Fatal("Instance bean must not register without spring.dubbo.registries")
		}
		if s.Cli != nil {
			t.Fatal("client bean must not register without the Instance bean")
		}
	})
}

func TestWiring_InstanceAndClientBeans(t *testing.T) {
	gs.Web(false).Configure(func(g gs.App) {
		setDubboProps(g, [2]string{"spring.dubbo.consumer.protocol", "tri"})
	}).RunTest(t, func(s *struct {
		Ins *Instance      `autowire:""`
		Cli *client.Client `autowire:""`
	}) {
		if s.Ins == nil || s.Cli == nil {
			t.Fatalf("Instance and client beans should assemble offline: ins=%v cli=%v", s.Ins, s.Cli)
		}
	})
}

// --- RegisterReference: refs live under consumer.references.<name> ---

// greetStub is a fake Triple stub; the ctor records the bound DubboReference
// instead of dialing so the test stays offline.
type greetStub struct {
	cli *client.Client
	n   int
}

var lastGreetStub *greetStub

func init() {
	// The DubboReference bound from ${spring.dubbo.consumer.references.greet}
	// arrives pre-translated as opts; the raw values are covered separately by
	// TestWiring_DubboConfigBinding, so this ctor only records the seam inputs.
	RegisterReference("greet", func(cli *client.Client, opts ...client.ReferenceOption) (*greetStub, error) {
		stub := &greetStub{cli: cli, n: len(opts)}
		lastGreetStub = stub
		return stub, nil
	})
}

func TestWiring_RegisterReference(t *testing.T) {
	gs.Web(false).Configure(func(g gs.App) {
		setDubboProps(g,
			[2]string{"spring.dubbo.consumer.references.greet.interface", "greet.GreetService"},
			[2]string{"spring.dubbo.consumer.references.greet.group", "g1"},
			[2]string{"spring.dubbo.consumer.references.greet.version", "v2"},
			[2]string{"spring.dubbo.consumer.references.greet.timeout", "1s"},
		)
	}).RunTest(t, func(s *struct {
		Stub *greetStub `autowire:""`
	}) {
		if lastGreetStub == nil {
			t.Fatal("reference ctor should run when the stub bean is wired")
		}
		if lastGreetStub.n < 4 { // interface+group+version+timeout should each emit an option
			t.Fatalf("reference bound from consumer.references.greet wrong: only %d options", lastGreetStub.n)
		}
		if lastGreetStub.cli == nil {
			t.Fatal("reference ctor should receive the assembled *client.Client")
		}
		if lastGreetStub.n == 0 {
			t.Fatal("reference ctor should receive options translated from the bound config")
		}
	})
}

// --- RegisterService: services live under provider.services.<name> ---

// echoSvc is a fake provider handler.
type echoSvc struct{}

var gotEchoOpts []server.ServiceOption

func init() {
	RegisterService("echo", func(_ *server.Server, _ echoSvc, opts ...server.ServiceOption) error {
		gotEchoOpts = opts
		return nil
	}, echoSvc{})
}

func TestWiring_RegisterService(t *testing.T) {
	// Registered in init (gs.Provide is init-only); no registries here, so
	// the Instance bean stays away and SimpleDubboServer
	// (a gs.Server) never Runs; only the ServiceRegister bean's config
	// binding is exercised.
	gs.Web(false).Configure(func(g gs.App) {
		g.Property("spring.dubbo.provider.services.echo.interface", "echo.EchoService")
		g.Property("spring.dubbo.provider.services.echo.group", "sg")
	}).RunTest(t, func(s *struct {
		Regs []ServiceRegister `autowire:""`
	}) {
		if len(s.Regs) == 0 {
			t.Fatal("RegisterService should contribute a ServiceRegister bean")
		}
		var regErr error
		for _, reg := range s.Regs {
			if err := reg(nil); err != nil { // fake register tolerates a nil server
				regErr = err
			}
		}
		if regErr != nil {
			t.Fatalf("registered services should run against the collected config: %v", regErr)
		}
		if len(gotEchoOpts) == 0 {
			t.Fatal("register func should receive options translated from provider.services.echo")
		}
	})
}
