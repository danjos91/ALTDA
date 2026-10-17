package allocator

import (
	"errors"
	"math/rand"
	"testing"
)

func fixedHours(lecture, practice float64) *Hours {
	return &Hours{Lecture: lecture, Practice: practice, Contact: lecture + practice, Total: lecture + practice}
}

func TestFixedTopicKeepsItsHours(t *testing.T) {
	in := Input{TotalHours: 40, StudyForm: "очная", Modules: []Module{
		{ID: "M1", Weight: 0.5, Topics: []Topic{
			{ID: "T1.1", Fixed: fixedHours(3, 5)},
			{ID: "T1.2", Weight: 1, Kinds: lecPrac()},
		}},
		{ID: "M2", Weight: 0.5, Topics: []Topic{{ID: "T2.1", Weight: 1, Kinds: lecPrac()}}},
	}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	locked := res.Modules[0].Topics[0].Hours
	if locked.Lecture != 3 || locked.Practice != 5 || locked.Total != 8 {
		t.Errorf("locked topic changed: %+v", locked)
	}
}

func TestFixedTopicHoursAreCorrectedToTheFormulas(t *testing.T) {
	// A manual edit left contact/total inconsistent with the parts.
	bad := &Hours{Lecture: 2, Practice: 2, Contact: 99, Total: 99}
	in := Input{TotalHours: 20, StudyForm: "очная", Modules: []Module{
		{ID: "M1", Weight: 1, Topics: []Topic{
			{ID: "T1.1", Fixed: bad},
			{ID: "T1.2", Weight: 1, Kinds: lecPrac()},
		}},
	}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	if got := res.Modules[0].Topics[0].Hours; got.Contact != 4 || got.Total != 4 {
		t.Errorf("contact/total must be recomputed from the parts: %+v", got)
	}
}

func TestFixedTopicsAbsorbTheBalance(t *testing.T) {
	// Locking 10 h of a 16 h program leaves 6 h for the other topic and the attestation.
	in := Input{TotalHours: 16, StudyForm: "очная", Modules: []Module{
		{ID: "M1", Weight: 1, Topics: []Topic{
			{ID: "T1.1", Fixed: fixedHours(4, 6)},
			{ID: "T1.2", Weight: 1, Kinds: lecPrac()},
		}},
	}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	if got := res.Modules[0].Topics[1].Hours.Total + res.FinalAttestation.Total; got != 6 {
		t.Errorf("free topic + attestation = %v, want 6", got)
	}
}

func TestFixedAttestation(t *testing.T) {
	in := Input{TotalHours: 72, StudyForm: "очная", FixedAttestation: f(6), Modules: twoModules()}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	if res.FinalAttestation.Assessment != 6 {
		t.Errorf("attestation = %v, want the fixed 6", res.FinalAttestation.Assessment)
	}
}

func TestAllTopicsFixed(t *testing.T) {
	mk := func(total float64) Input {
		return Input{TotalHours: total, StudyForm: "очная", FixedAttestation: f(2), Modules: []Module{
			{ID: "M1", Topics: []Topic{{ID: "T1.1", Fixed: fixedHours(5, 9)}}},
		}}
	}

	t.Run("hours already match", func(t *testing.T) {
		res, err := Allocate(mk(16), DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		checkResult(t, mk(16), DefaultConfig(), res)
	})

	t.Run("hours do not match", func(t *testing.T) {
		if _, err := Allocate(mk(20), DefaultConfig()); !errors.Is(err, ErrNothingToBalance) {
			t.Errorf("err = %v, want ErrNothingToBalance", err)
		}
	})
}

func TestFixedHoursLargerThanProgram(t *testing.T) {
	in := Input{TotalHours: 16, StudyForm: "очная", Modules: []Module{
		{ID: "M1", Weight: 1, Topics: []Topic{
			{ID: "T1.1", Fixed: fixedHours(10, 10)},
			{ID: "T1.2", Weight: 1, Kinds: lecPrac()},
		}},
	}}

	if _, err := Allocate(in, DefaultConfig()); !errors.Is(err, ErrNotEnoughHours) {
		t.Errorf("err = %v, want ErrNotEnoughHours", err)
	}
}

func TestPracticeRaiseNeverTouchesFixedTopics(t *testing.T) {
	in := Input{TotalHours: 60, StudyForm: "очная", MinPracticeShare: f(0.9), Modules: []Module{
		{ID: "M1", Weight: 1, Topics: []Topic{
			{ID: "T1.1", Fixed: fixedHours(20, 0), Kinds: lecPrac()},
			{ID: "T1.2", Weight: 1, Kinds: lecPrac()},
		}},
	}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Modules[0].Topics[0].Hours; got.Lecture != 20 || got.Practice != 0 {
		t.Errorf("locked topic was changed to reach the practice share: %+v", got)
	}
}

// Property test: with random locked topics every invariant still holds and
// locked topics come out exactly as they went in.
func TestAllocateWithFixedProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	cfg := DefaultConfig()

	accepted := 0
	for i := 0; i < 3000; i++ {
		in := Input{TotalHours: float64(64+rng.Intn(937)) / 2, StudyForm: "очная"}

		for m := 0; m < 1+rng.Intn(5); m++ {
			mod := Module{ID: "M", Weight: 0.01 + rng.Float64()}
			for tp := 0; tp < 1+rng.Intn(5); tp++ {
				topic := Topic{ID: "T", Weight: 0.01 + rng.Float64(), Kinds: lecPrac()}
				if rng.Intn(4) == 0 {
					topic.Fixed = fixedHours(float64(rng.Intn(10))/2, float64(1+rng.Intn(10))/2)
				}
				mod.Topics = append(mod.Topics, topic)
			}
			in.Modules = append(in.Modules, mod)
		}

		res, err := Allocate(in, cfg)
		if errors.Is(err, ErrNotEnoughHours) || errors.Is(err, ErrNothingToBalance) {
			continue
		}
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		accepted++

		// Locked topics may be below MinTopicHours by choice, so check the
		// remaining invariants by hand instead of through checkResult.
		for mi, m := range in.Modules {
			for ti, topic := range m.Topics {
				got := res.Modules[mi].Topics[ti].Hours
				if topic.Fixed != nil {
					if got.Lecture != topic.Fixed.Lecture || got.Practice != topic.Fixed.Practice {
						t.Fatalf("case %d: locked topic changed: %+v -> %+v", i, topic.Fixed, got)
					}
				}
				checkFormulas(t, topic.ID, got)
			}
			checkFormulas(t, m.ID, res.Modules[mi].Hours)
		}
		checkFormulas(t, "totals", res.Totals)
		if res.Totals.Total != in.TotalHours {
			t.Fatalf("case %d: total %v != requested %v", i, res.Totals.Total, in.TotalHours)
		}
	}
	if accepted < 1500 {
		t.Errorf("only %d of 3000 random programs were accepted", accepted)
	}
}
