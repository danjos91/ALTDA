package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	skel "github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// Drafter is an LLM-backed skeleton.Drafter.
// It requests up to three variants as separate parallel calls (gateway max_tokens=4096).
// Near the endpoint deadline it returns whatever variants finished (soft deadline);
// llm_timeout only if none are ready.
type Drafter struct {
	client     *Client
	callTO     time.Duration
	softMargin time.Duration
}

func NewDrafter(client *Client, cfg Config) skeleton.Drafter {
	callTO := cfg.SkeletonCallTimeout
	if callTO < 1 {
		callTO = 90 * time.Second
	}
	margin := cfg.SkeletonSoftMargin
	if margin < 1 {
		margin = 15 * time.Second
	}
	return &Drafter{client: client, callTO: callTO, softMargin: margin}
}

type singleVariantResponse struct {
	Variant *skel.Draft `json:"variant"`
	// Also accept {"variants":[one]} from a looser model.
	Variants []skel.Draft `json:"variants"`
}

var variantSpecs = []struct {
	Key  string
	Hint string
}{
	{"A", "линейный курс: от основ к сложному"},
	{"B", "проектный курс: практика с первого модуля"},
	{"C", "спираль или лекционно-семинарский курс"},
}

func (d *Drafter) Draft(ctx context.Context, req skeleton.Request) ([]skel.Draft, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %v", ErrUpstream, err)
	}

	softCtx, softCancel := context.WithDeadline(ctx, softDeadlineFor(ctx, d.softMargin))
	defer softCancel()

	type result struct {
		draft skel.Draft
		err   error
		ok    bool // completed successfully with a draft
	}
	results := make([]result, len(variantSpecs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, spec := range variantSpecs {
		wg.Add(1)
		go func(i int, key, hint string) {
			defer wg.Done()
			user := fmt.Sprintf(
				"Параметры программы (JSON). Сгенерируй РОВНО ОДИН вариант каркаса с key=%q (%s), без hours.\nВерни JSON: {\"variant\":{...}}.\n%s",
				key, hint, string(body),
			)
			var out singleVariantResponse
			err := d.client.ChatJSONTimeout(softCtx, skeletonSystemPrompt, user, &out, d.callTO)
			if err != nil {
				mu.Lock()
				results[i] = result{err: err}
				mu.Unlock()
				return
			}
			draft, err := pickVariant(out, key)
			if err != nil {
				mu.Lock()
				results[i] = result{err: err}
				mu.Unlock()
				return
			}
			draft.Key = key
			draft.Normalize()
			mu.Lock()
			results[i] = result{draft: draft, ok: true}
			mu.Unlock()
		}(i, spec.Key, spec.Hint)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-softCtx.Done():
		slog.WarnContext(ctx, "skeleton soft deadline: returning finished variants",
			slog.Duration("soft_margin", d.softMargin),
			slog.Any("cause", softCtx.Err()),
		)
		// softCtx cancel aborts in-flight calls; wait for workers to record results.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			slog.WarnContext(ctx, "skeleton soft deadline: workers still finishing")
		}
	}

	mu.Lock()
	defer mu.Unlock()

	var drafts []skel.Draft
	var errs []error
	timedOut := false
	for i, r := range results {
		if r.ok {
			drafts = append(drafts, r.draft)
			continue
		}
		if r.err != nil {
			slog.WarnContext(ctx, "skeleton variant failed",
				slog.String("key", variantSpecs[i].Key),
				slog.Any("error", r.err),
			)
			errs = append(errs, r.err)
			if errors.Is(r.err, ErrTimeout) || errors.Is(r.err, context.DeadlineExceeded) || errors.Is(r.err, context.Canceled) {
				timedOut = true
			}
		}
	}
	if len(drafts) > 0 {
		return drafts, nil
	}
	if timedOut || softCtx.Err() != nil || ctx.Err() != nil {
		return nil, fmt.Errorf("%w: no skeleton variants ready before deadline", ErrTimeout)
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("%w: no variants", ErrUpstream)
	}
	return nil, fmt.Errorf("%w: all variants failed: %v", ErrUpstream, errs[0])
}

// softDeadlineFor is a bit below the request/endpoint deadline so we can
// return finished variants before the handler hits 504.
func softDeadlineFor(ctx context.Context, margin time.Duration) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl.Add(-margin)
	}
	return time.Now().Add(280*time.Second - margin)
}

func pickVariant(out singleVariantResponse, wantKey string) (skel.Draft, error) {
	if out.Variant != nil {
		return *out.Variant, nil
	}
	for _, v := range out.Variants {
		if v.Key == wantKey || v.Key == "" {
			return v, nil
		}
	}
	if len(out.Variants) == 1 {
		return out.Variants[0], nil
	}
	return skel.Draft{}, fmt.Errorf("%w: missing variant %s", ErrSchemaInvalid, wantKey)
}
