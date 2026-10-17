package llm

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/danjos91/ALTDA/go-service/internal/program"
)

// moduleFiller produces content for one curriculum module (activities, etc.).
// Implementations must be safe for concurrent use and honour ctx cancellation.
type moduleFiller func(ctx context.Context, mod program.Object, modIndex int) (program.Object, error)

// fillModulesParallel runs filler over curriculum modules with bounded concurrency.
// Results are written back in original module order (deterministic).
func fillModulesParallel(
	ctx context.Context,
	content program.Object,
	concurrency int,
	filler moduleFiller,
) error {
	if concurrency < 1 {
		concurrency = 4
	}
	cur := program.EnsureObject(content, "curriculum")
	modulesAny, _ := program.AsArray(cur["modules"])
	if len(modulesAny) == 0 {
		return nil
	}

	results := make([]program.Object, len(modulesAny))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, raw := range modulesAny {
		i := i
		mod, ok := program.AsObject(raw)
		if !ok {
			return fmt.Errorf("%w: curriculum.modules[%d] is not an object", ErrSchemaInvalid, i)
		}
		// Snapshot for the worker; mutations go into the returned object.
		modCopy := program.CloneObject(mod)
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return fmt.Errorf("%w: %v", ErrTimeout, err)
			}
			out, err := filler(gctx, modCopy, i)
			if err != nil {
				return err
			}
			if out == nil {
				out = modCopy
			}
			results[i] = out
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	ordered := make(program.Array, len(results))
	for i, m := range results {
		if m == nil {
			return fmt.Errorf("%w: missing module result at %d", ErrUpstream, i)
		}
		ordered[i] = m
	}
	cur["modules"] = ordered
	return nil
}

// stubFillModule writes deterministic activities for each topic with non-zero hours.
// Skips topics that are locked or outside the target set (when target is non-nil).
func stubFillModule(mod program.Object, target, locked map[string]bool) program.Object {
	mid := program.GetString(mod, "id", "")
	if locked[mid] {
		return mod
	}
	if target != nil && mid != "" && !target[mid] {
		// Module not in regenerate set — still fill topics that are targeted.
	}
	topicsAny, _ := program.AsArray(mod["topics"])
	outTopics := make(program.Array, 0, len(topicsAny))
	for _, t := range topicsAny {
		to, ok := program.AsObject(t)
		if !ok {
			outTopics = append(outTopics, t)
			continue
		}
		tid := program.GetString(to, "id", "")
		if locked[tid] || locked[mid] {
			outTopics = append(outTopics, to)
			continue
		}
		if target != nil && !target[tid] && !target[mid] {
			outTopics = append(outTopics, to)
			continue
		}
		h := program.HoursFromObject(program.EnsureObject(to, "hours"))
		acts := program.Array{}
		if h.Lecture > 0 {
			acts = append(acts, program.Object{
				"kind": "lecture", "hours": h.Lecture,
				"content": fmt.Sprintf("Лекционный материал: %s", program.GetString(to, "title", tid)),
				"delivery": "onsite",
			})
		}
		if h.Practice > 0 {
			acts = append(acts, program.Object{
				"kind": "practice", "hours": h.Practice,
				"content": fmt.Sprintf("Практические задания: %s", program.GetString(to, "title", tid)),
				"delivery": "onsite",
			})
		}
		if h.Lab > 0 {
			acts = append(acts, program.Object{
				"kind": "lab", "hours": h.Lab,
				"content": fmt.Sprintf("Лабораторная работа: %s", program.GetString(to, "title", tid)),
				"delivery": "onsite",
			})
		}
		if h.SelfStudy > 0 {
			acts = append(acts, program.Object{
				"kind": "selfStudy", "hours": h.SelfStudy,
				"content": fmt.Sprintf("Самостоятельная работа: %s", program.GetString(to, "title", tid)),
				"delivery": "distance_async",
			})
		}
		if h.Assessment > 0 {
			acts = append(acts, program.Object{
				"kind": "control", "hours": h.Assessment,
				"content": fmt.Sprintf("Контроль по теме: %s", program.GetString(to, "title", tid)),
				"delivery": "onsite",
			})
		}
		if len(acts) > 0 {
			to["activities"] = acts
		}
		outTopics = append(outTopics, to)
	}
	mod["topics"] = outTopics
	return mod
}

// contentTargets resolves nodeIds → set of ids to regenerate (plus dependents), minus locked.
// A nil return means "all modules/topics".
func contentTargets(prog program.Object, nodeIDs []string) (target map[string]bool, skipped program.Array) {
	if nodeIDs == nil {
		return nil, program.Array{}
	}
	target = program.CollectDependents(prog, nodeIDs)
	locked := program.CollectLockedIDs(prog)
	for id := range target {
		if locked[id] {
			skipped = append(skipped, id)
			delete(target, id)
		}
	}
	return target, skipped
}

// ensureContentConcurrency returns a positive concurrency (default 4).
func ensureContentConcurrency(n int) int {
	if n < 1 {
		return 4
	}
	return n
}
