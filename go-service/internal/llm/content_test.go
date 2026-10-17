package llm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danjos91/ALTDA/go-service/internal/program"
)

func TestFillModulesParallelPreservesOrder(t *testing.T) {
	content := program.Object{
		"curriculum": program.Object{
			"modules": program.Array{
				program.Object{"id": "M1", "title": "first"},
				program.Object{"id": "M2", "title": "second"},
				program.Object{"id": "M3", "title": "third"},
			},
		},
	}
	var maxInFlight atomic.Int32
	var inFlight atomic.Int32
	err := fillModulesParallel(context.Background(), content, 2, func(ctx context.Context, mod program.Object, idx int) (program.Object, error) {
		cur := inFlight.Add(1)
		for {
			prev := maxInFlight.Load()
			if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		mod["filled"] = idx
		return mod, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if maxInFlight.Load() > 2 {
		t.Fatalf("concurrency exceeded: %d", maxInFlight.Load())
	}
	mods, _ := program.AsArray(program.Nested(content, "curriculum")["modules"])
	for i, m := range mods {
		mo, _ := program.AsObject(m)
		if int(program.GetFloat(mo, "filled", -1)) != i {
			t.Fatalf("order broken at %d: %+v", i, mo)
		}
		want := fmt.Sprintf("M%d", i+1)
		if program.GetString(mo, "id", "") != want {
			t.Fatalf("id order broken: got %s want %s", program.GetString(mo, "id", ""), want)
		}
	}
}

func TestFillModulesParallelCancels(t *testing.T) {
	content := program.Object{
		"curriculum": program.Object{
			"modules": program.Array{
				program.Object{"id": "M1"},
				program.Object{"id": "M2"},
				program.Object{"id": "M3"},
				program.Object{"id": "M4"},
			},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := fillModulesParallel(ctx, content, 2, func(ctx context.Context, mod program.Object, idx int) (program.Object, error) {
		if idx == 0 {
			cancel()
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %v", ErrTimeout, ctx.Err())
		case <-time.After(200 * time.Millisecond):
			return mod, nil
		}
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, ErrTimeout) && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStubFillModuleActivities(t *testing.T) {
	mod := program.Object{
		"id": "M1",
		"topics": program.Array{
			program.Object{
				"id": "T1.1", "title": "Intro",
				"hours": program.Object{
					"lecture": 2, "practice": 1, "lab": 0, "selfStudy": 0.5,
					"assessment": 0, "contact": 3, "total": 3.5,
				},
			},
		},
	}
	out := stubFillModule(mod, nil, map[string]bool{})
	topics, _ := program.AsArray(out["topics"])
	to, _ := program.AsObject(topics[0])
	acts, _ := program.AsArray(to["activities"])
	if len(acts) < 2 {
		t.Fatalf("expected activities, got %d", len(acts))
	}
}
