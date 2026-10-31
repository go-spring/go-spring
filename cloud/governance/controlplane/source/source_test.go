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

package source

import (
	"context"
	"reflect"
	"testing"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

// otherDoc exists only to prove two Source instantiations are distinct.
type otherDoc struct{}

// TestAs_GenericInterfaceExport pins the mechanism the whole split depends on:
// gs.As can name an INSTANTIATED generic interface, and two instantiations are
// distinct types, so the two roots can never receive each other's source.
func TestAs_GenericInterfaceExport(t *testing.T) {
	it := gs.As[Source[testDoc]]()
	if it.Kind() != reflect.Interface {
		t.Fatalf("gs.As[Source[testDoc]]() kind = %v, want interface", it.Kind())
	}
	if !reflect.TypeFor[*FileSource[testDoc]]().Implements(it) {
		t.Fatal("*FileSource[testDoc] must implement Source[testDoc]")
	}

	other := gs.As[Source[otherDoc]]()
	if other == it {
		t.Fatal("Source[A] and Source[B] must be distinct types")
	}
	if reflect.TypeFor[*FileSource[testDoc]]().Implements(other) {
		t.Fatal("Source[otherDoc] must not accept a Source[testDoc] implementation")
	}
}

// TestPushSource_InitialSnapshotAndSubscribe covers the mutable core: the
// constructor's snapshot is what Snapshot returns, and a Push moves both the
// snapshot and the subscriber.
func TestPushSource_InitialSnapshotAndSubscribe(t *testing.T) {
	src := NewPushSource(testDoc{Client: testClient{Timeout: time.Second}})
	if got := src.Snapshot().Client.Timeout; got != time.Second {
		t.Fatalf("initial snapshot: %v", got)
	}

	var got testDoc
	src.Subscribe(func(_ context.Context, doc testDoc) { got = doc })

	src.Push(context.Background(), testDoc{Client: testClient{Timeout: 2 * time.Second}})
	if got.Client.Timeout != 2*time.Second {
		t.Fatalf("subscriber got %v", got.Client.Timeout)
	}
	if src.Snapshot().Client.Timeout != 2*time.Second {
		t.Fatal("push must move the snapshot")
	}
}

// TestPushSource_ArmBeforeLoop pins the one cross-goroutine read the contract
// allows: a center's initial Snapshot during wiring, before the delivery
// goroutine starts. Goroutine creation orders the two, so -race stays quiet;
// anything beyond this pattern (concurrent Push, concurrent Snapshot) is a
// caller bug the contract declines to defend.
func TestPushSource_ArmBeforeLoop(t *testing.T) {
	src := NewPushSource(testDoc{})
	src.Subscribe(func(context.Context, testDoc) {})

	// The wiring-phase read: sees the constructor-committed document.
	if d := src.Snapshot(); d.Client.Timeout != 0 {
		t.Fatalf("initial snapshot must be the constructor's: %v", d.Client.Timeout)
	}

	// The loop starts only after arming: pushes land after the read.
	done := make(chan struct{})
	go func() {
		defer close(done)
		src.Push(context.Background(), testDoc{Client: testClient{Timeout: time.Second}})
	}()
	<-done
	if d := src.Snapshot(); d.Client.Timeout != time.Second {
		t.Fatalf("push after arm must move the snapshot: %v", d.Client.Timeout)
	}
}

// TestPushDeliversTheChangesContext proves the change's context survives the
// seam unchanged: the subscriber — and through it a root's center — logs what it
// did about the change against the identity the source named it with.
func TestPushDeliversTheChangesContext(t *testing.T) {
	src := NewPushSource(testDoc{})
	var got context.Context
	src.Subscribe(func(ctx context.Context, _ testDoc) { got = ctx })

	change := log.RootFields(log.String("trace_id", "t1"))
	src.Push(change, testDoc{Client: testClient{Timeout: time.Second}})

	if got != change {
		t.Fatal("the pushed context must reach the subscriber unchanged")
	}
	if fields := log.CarriedFields(got); len(fields) != 1 || fields[0].Key != "trace_id" {
		t.Fatalf("the change's fields must ride along: %+v", fields)
	}
}
