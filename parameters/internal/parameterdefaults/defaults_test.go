package parameterdefaults

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

type provider struct{ cleared atomic.Int32 }

func (p *provider) ClearCache() { p.cleared.Add(1) }

func TestLazyInitializationRetryAndClear(t *testing.T) {
	lazy := New[*provider]()
	boom := errors.New("configuration failed")
	_, err := lazy.Get(context.Background(), func(context.Context) (*provider, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	var creates atomic.Int32
	want := &provider{}
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			value, err := lazy.Get(context.Background(), func(context.Context) (*provider, error) { creates.Add(1); return want, nil })
			if err != nil || value != want {
				t.Errorf("default: %v %v", value, err)
			}
		})
	}
	wg.Wait()
	if creates.Load() != 1 {
		t.Fatal("default initialized more than once")
	}
	ClearCaches()
	if want.cleared.Load() != 1 {
		t.Fatal("default cache not cleared")
	}
	lazy.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = lazy.Get(ctx, nil)
	<-lazy.gate
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
