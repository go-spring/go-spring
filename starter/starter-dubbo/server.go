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
	"context"
	"runtime"
	"time"

	"dubbo.apache.org/dubbo-go/v3/config"
	"dubbo.apache.org/dubbo-go/v3/protocol"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleDubboServer := gs.OnProperty("spring.dubbo.provider.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleDubboServer, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(
			NewSimpleDubboServer,
		).Export(gs.As[gs.Server]()).Condition(
			gs.OnBean[ServiceRegister](),
			gs.OnBean[*Instance](),
		)
		return nil
	})
}

// ServiceRegister registers services on a Dubbo server.Server.
type ServiceRegister func(svr *server.Server) error

// options translates DubboService into dubbo-go server.ServiceOptions.
func (s DubboService) options() []server.ServiceOption {
	var opts []server.ServiceOption
	if s.Group != "" {
		opts = append(opts, server.WithGroup(s.Group))
	}
	if s.Version != "" {
		opts = append(opts, server.WithVersion(s.Version))
	}
	if s.Cluster != "" {
		opts = append(opts, server.WithCluster(s.Cluster))
	}
	if s.LoadBalance != "" {
		opts = append(opts, server.WithLoadBalance(s.LoadBalance))
	}
	if s.Serialization != "" {
		opts = append(opts, server.WithSerialization(s.Serialization))
	}
	if s.Retries > 0 {
		opts = append(opts, server.WithRetries(s.Retries))
	}
	if s.Filter != "" {
		opts = append(opts, server.WithFilter(s.Filter))
	}
	if s.Token != "" {
		opts = append(opts, server.WithToken(s.Token))
	}
	if s.AccessLog != "" {
		opts = append(opts, server.WithAccesslog(s.AccessLog))
	}
	if s.Auth != "" {
		opts = append(opts, server.WithAuth(s.Auth))
	}
	if s.Tag != "" {
		opts = append(opts, server.WithTag(s.Tag))
	}
	if s.Warmup != "" {
		if d, err := parseDuration(s.Warmup); err == nil && d > 0 {
			opts = append(opts, server.WithWarmup(d))
		}
	}
	if s.NotRegister {
		opts = append(opts, server.WithNotRegister())
	}
	if len(s.ProtocolIDs) > 0 {
		opts = append(opts, server.WithProtocolIDs(s.ProtocolIDs))
	}
	if len(s.RegistryIDs) > 0 {
		opts = append(opts, server.WithRegistryIDs(s.RegistryIDs))
	}
	if s.TpsLimiter != "" {
		opts = append(opts, server.WithTpsLimiter(s.TpsLimiter))
	}
	if s.TpsLimitRate > 0 {
		opts = append(opts, server.WithTpsLimitRate(s.TpsLimitRate))
	}
	if s.TpsLimitStrategy != "" {
		opts = append(opts, server.WithTpsLimitStrategy(s.TpsLimitStrategy))
	}
	if s.TpsLimitRejectedHandler != "" {
		opts = append(opts, server.WithTpsLimitRejectedHandler(s.TpsLimitRejectedHandler))
	}
	if s.ExecuteLimit != "" {
		opts = append(opts, server.WithExecuteLimit(s.ExecuteLimit))
	}
	if s.ExecuteLimitRejectedHandler != "" {
		opts = append(opts, server.WithExecuteLimitRejectedHandler(s.ExecuteLimitRejectedHandler))
	}
	if s.ParamSign != "" {
		opts = append(opts, server.WithParamSign(s.ParamSign))
	}
	for k, v := range s.Params {
		opts = append(opts, server.WithParam(k, v))
	}
	for name, m := range s.Methods {
		if m.Name == "" {
			m.Name = name
		}
		if mopts := m.options(); len(mopts) > 0 {
			opts = append(opts, server.WithMethod(mopts...))
		}
	}
	return opts
}

// options translates DubboMethod into dubbo-go config.MethodOptions.
func (m DubboMethod) options() []config.MethodOption {
	var opts []config.MethodOption
	if m.Name != "" {
		opts = append(opts, config.WithName(m.Name))
	}
	if m.Retries > 0 {
		opts = append(opts, config.WithRetries(m.Retries))
	}
	if m.LoadBalance != "" {
		opts = append(opts, config.WithLoadBalance(m.LoadBalance))
	}
	if m.Weight >= 0 {
		opts = append(opts, config.WithWeight(m.Weight))
	}
	if m.TpsLimitInterval > 0 {
		opts = append(opts, config.WithTpsLimitInterval(m.TpsLimitInterval))
	}
	if m.TpsLimitRate > 0 {
		opts = append(opts, config.WithTpsLimitRate(m.TpsLimitRate))
	}
	if m.TpsLimitStrategy != "" {
		opts = append(opts, config.WithTpsLimitStrategy(m.TpsLimitStrategy))
	}
	if m.ExecuteLimit > 0 {
		opts = append(opts, config.WithExecuteLimit(m.ExecuteLimit))
	}
	if m.ExecuteLimitRejectedHandler != "" {
		opts = append(opts, config.WithExecuteLimitRejectedHandler(m.ExecuteLimitRejectedHandler))
	}
	if m.Sticky {
		opts = append(opts, config.WithSticky())
	}
	if m.Timeout != "" {
		if d, err := parseDuration(m.Timeout); err == nil && d > 0 {
			opts = append(opts, config.WithRequestTimeout(d))
		}
	}
	return opts
}

// parseDuration is a helper that parses a duration string.
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// SimpleDubboServer adapts a Dubbo-go server.Server to the Go-Spring server lifecycle.
type SimpleDubboServer struct {
	d    *Instance
	Regs []ServiceRegister `autowire:"?"`
	svr  *server.Server
	done chan struct{}
}

// NewSimpleDubboServer creates a SimpleDubboServer from the shared *Instance's
// provider configuration.
func NewSimpleDubboServer(d *Instance) *SimpleDubboServer {
	return &SimpleDubboServer{d: d, done: make(chan struct{})}
}

// buildOptions translates DubboProvider into dubbo-go server options.
func (c *DubboProvider) buildOptions(protocols map[string]DubboProtocol, global map[string]DubboRegistry) ([]server.ServerOption, error) {
	var opts []server.ServerOption

	// Provider-wide defaults.
	if c.Group != "" {
		opts = append(opts, server.WithServerGroup(c.Group))
	}
	if c.Version != "" {
		opts = append(opts, server.WithServerVersion(c.Version))
	}
	if c.Cluster != "" {
		opts = append(opts, server.WithServerCluster(c.Cluster))
	}
	if c.LoadBalance != "" {
		opts = append(opts, server.WithServerLoadBalance(c.LoadBalance))
	}
	if c.Serialization != "" {
		opts = append(opts, server.WithServerSerialization(c.Serialization))
	}
	if c.Retries > 0 {
		opts = append(opts, server.WithServerRetries(c.Retries))
	}
	if c.Filter != "" {
		opts = append(opts, server.WithServerFilter(c.Filter))
	}
	if c.Token != "" {
		opts = append(opts, server.WithServerToken(c.Token))
	}
	if c.AccessLog != "" {
		opts = append(opts, server.WithServerAccesslog(c.AccessLog))
	}
	if c.Auth != "" {
		opts = append(opts, server.WithServerAuth(c.Auth))
	}
	if c.Tag != "" {
		opts = append(opts, server.WithServerTag(c.Tag))
	}
	if c.Warmup != "" {
		if d, err := parseDuration(c.Warmup); err == nil && d > 0 {
			opts = append(opts, server.WithServerWarmup(d))
		}
	}
	if c.NotRegister {
		opts = append(opts, server.WithServerNotRegister())
	}
	if c.AdaptiveService {
		opts = append(opts, server.WithServerAdaptiveService())
	}
	if c.AdaptiveServiceVerbose {
		opts = append(opts, server.WithServerAdaptiveServiceVerbose())
	}
	if c.TpsLimiter != "" {
		opts = append(opts, server.WithServerTpsLimiter(c.TpsLimiter))
	}
	if c.TpsLimitRate > 0 {
		opts = append(opts, server.WithServerTpsLimitRate(c.TpsLimitRate))
	}
	if c.TpsLimitStrategy != "" {
		opts = append(opts, server.WithServerTpsLimitStrategy(c.TpsLimitStrategy))
	}
	if c.TpsLimitRejectedHandler != "" {
		opts = append(opts, server.WithServerTpsLimitRejectedHandler(c.TpsLimitRejectedHandler))
	}
	if c.ExecuteLimit != "" {
		opts = append(opts, server.WithServerExecuteLimit(c.ExecuteLimit))
	}
	if c.ExecuteLimitRejectedHandler != "" {
		opts = append(opts, server.WithServerExecuteLimitRejectedHandler(c.ExecuteLimitRejectedHandler))
	}
	if c.ParamSign != "" {
		opts = append(opts, server.WithServerParamSign(c.ParamSign))
	}
	for k, v := range c.Params {
		opts = append(opts, server.WithServerParam(k, v))
	}
	if len(c.ProtocolIDs) > 0 {
		opts = append(opts, server.WithServerProtocolIDs(c.ProtocolIDs))
	}

	// Fallback protocol when none are configured globally.
	if len(protocols) == 0 {
		opts = append(opts, server.WithServerProtocol(
			protocol.WithID("tri"),
			protocol.WithProtocol("tri"),
			protocol.WithPort(20000),
		))
	}

	// Registry publish targets, selected from the global block by RegistryIDs.
	registries, err := selectRegistries(global, c.RegistryIDs)
	if err != nil {
		return nil, err
	}
	for name, r := range registries {
		opts = append(opts, server.WithServerRegistry(r.options(name)...))
	}

	return opts, nil
}

// Run assembles the Dubbo server and starts serving.
func (s *SimpleDubboServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	cfg := s.d.Provider()
	opts, err := cfg.buildOptions(s.d.Protocols(), s.d.Registries())
	if err != nil {
		return err
	}
	svr, err := s.d.NewServer(opts...)
	if err != nil {
		return errutil.Explain(err, "failed to create dubbo server")
	}
	if err = s.regAll(svr); err != nil {
		return errutil.Explain(err, "failed to register dubbo service")
	}
	s.svr = svr

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		errCh <- svr.Serve()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve dubbo server")
	case <-s.done:
		return nil
	}
}

// Stop signals Run to return so Go-Spring can complete its shutdown sequence.
func (s *SimpleDubboServer) Stop() error {
	close(s.done)
	return nil
}

// regAll invokes every collected ServiceRegister bean against the assembled server.
func (s *SimpleDubboServer) regAll(svr *server.Server) error {
	for _, reg := range s.Regs {
		if err := reg(svr); err != nil {
			return err
		}
	}
	return nil
}

// RegisterService registers a Dubbo service as a ServiceRegister bean.
// name is the key under ${spring.dubbo.provider.services} to bind.
func RegisterService[T any](name string, register func(*server.Server, T, ...server.ServiceOption) error, hdlr T) {
	b := gs.Provide(func(svc DubboService) ServiceRegister {
		return func(svr *server.Server) error {
			return register(svr, hdlr, svc.options()...)
		}
	}, gs.IndexArg(0, gs.TagArg("${spring.dubbo.provider.services."+name+"}")))
	if _, file, line, ok := runtime.Caller(1); ok {
		b.SetFileLine(file, line)
	}
}
