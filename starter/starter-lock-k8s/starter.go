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

// Package StarterLockK8s wires a Kubernetes-Lease-backed distributed lock into
// Go-Spring. Blank-importing the package registers one [lock.Locker] bean per
// entry under "${spring.lock}", each owning its own Kubernetes clientset and
// backing every lock with a coordination.k8s.io/Lease object.
//
// It is the K8s-native backend of the lock abstraction: leader election and
// distributed locking use the control plane's own Lease API (the mechanism
// behind kube-controller-manager's --leader-elect and spring-cloud-kubernetes),
// so an in-cluster application needs no extra middleware (etcd/consul/redis).
// Business code depends only on lock.Locker/lock.Election and never on this
// package, so switching backend is a blank-import swap under the shared
// "spring.lock" prefix.
package StarterLockK8s

import (
	"runtime"

	"go-spring.org/spring/cloud/lock"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register one Lease-backed Locker per entry under "${spring.lock}". A
	// hand-rolled gs.Module (rather than gs.Group) is required so each bean can
	// Export the lock.Locker interface — consumers inject that interface and
	// never see the concrete *k8sLocker type, which is what makes the
	// blank-import backend swap possible.
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.lock"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.lock}"); err != nil {
			return err
		}
		for name, c := range m {
			b := r.Provide(newK8sLocker, gs.ValueArg(c)).
				Name(name).
				Export(gs.As[lock.Locker]()).
				Destroy(destroyK8sLocker)
			b.SetFileLine(file, line)
		}
		return nil
	})
}

// destroyK8sLocker releases the Locker's backend resources on shutdown. Locks
// handed out before shutdown own their own renewal goroutines and Lease objects;
// their leases expire naturally once the process exits.
func destroyK8sLocker(l *k8sLocker) error {
	return l.Close()
}
