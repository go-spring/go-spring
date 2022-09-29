package StarterReplay

import (
	"context"
	"sync"

	"github.com/go-spring/spring-base/cache"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/run"
)

type replayInterface struct {
	next cache.Driver
}

func NewReplayInterface(next cache.Driver) cache.Driver {
	return &replayInterface{
		next: next,
	}
}

func (d *replayInterface) Load(ctx context.Context, m *sync.Map, key string, loader cache.ResultLoader, arg cache.OptionArg) (loadType cache.LoadType, result cache.Result, err error) {

	if run.ReplayMode() {
		var v interface{}
		const ctxKey = "::CacheOnContext::"
		v, _, err = knife.LoadOrStore(ctx, ctxKey, &sync.Map{})
		if err != nil {
			return cache.LoadNone, nil, err
		}
		m = v.(*sync.Map)
	}

	defer func() {
		if loadType == cache.LoadCache && run.RecordMode() {
			// ...............
		}
	}()

	if run.ReplayMode() {
		return d.next.Load(ctx, m, key, func(ctx context.Context, key string) (cache.Result, error) {
			// .................
			return loader(ctx, key)
		}, arg)
	}
	return d.next.Load(ctx, m, key, loader, arg)
}

type contextInterface struct {
	next cache.Driver
}

func NewContextInterface(next cache.Driver) cache.Driver {
	return &contextInterface{
		next: next,
	}
}

type ctxItem struct {
	val cache.Result
	err error
	wg  sync.WaitGroup
}

func newCtxItem() *ctxItem {
	c := &ctxItem{}
	c.wg.Add(1)
	return c
}

func (d *contextInterface) Load(ctx context.Context, m *sync.Map, key string, loader cache.ResultLoader, arg cache.OptionArg) (loadType cache.LoadType, result cache.Result, err error) {

	i, loaded, err := knife.LoadOrStore(ctx, key, newCtxItem())
	if err != nil {
		return cache.LoadNone, nil, err
	}
	cItem := i.(*ctxItem)
	if loaded {
		cItem.wg.Wait()
		return cache.LoadCache, cItem.val, cItem.err
	}

	defer func() {
		cItem.val = result
		cItem.err = err
		cItem.wg.Done()
		if err != nil {
			knife.Delete(ctx, key)
		}
	}()

	return d.next.Load(ctx, m, key, loader, arg)
}
