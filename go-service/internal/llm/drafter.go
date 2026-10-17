package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	skel "github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// Drafter is an LLM-backed skeleton.Drafter.
// It requests up to three variants as separate parallel calls (gateway max_tokens=4096).
type Drafter struct {
	client *Client
}

func NewDrafter(client *Client) skeleton.Drafter {
	return &Drafter{client: client}
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

	type result struct {
		draft skel.Draft
		err   error
	}
	results := make([]result, len(variantSpecs))
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
			if err := d.client.ChatJSON(ctx, skeletonSystemPrompt, user, &out); err != nil {
				results[i] = result{err: err}
				return
			}
			draft, err := pickVariant(out, key)
			if err != nil {
				results[i] = result{err: err}
				return
			}
			draft.Key = key
			draft.Normalize()
			results[i] = result{draft: draft}
		}(i, spec.Key, spec.Hint)
	}
	wg.Wait()

	var drafts []skel.Draft
	var errs []error
	for i, r := range results {
		if r.err != nil {
			slog.WarnContext(ctx, "skeleton variant failed",
				slog.String("key", variantSpecs[i].Key),
				slog.Any("error", r.err),
			)
			errs = append(errs, r.err)
			continue
		}
		drafts = append(drafts, r.draft)
	}
	if len(drafts) == 0 {
		if len(errs) == 0 {
			return nil, fmt.Errorf("%w: no variants", ErrUpstream)
		}
		return nil, fmt.Errorf("%w: all variants failed: %v", ErrUpstream, errs[0])
	}
	return drafts, nil
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
