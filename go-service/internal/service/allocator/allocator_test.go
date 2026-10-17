package allocator

import (
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func f(v float64) *float64 { return &v }

func lecPrac() []Kind { return []Kind{KindLecture, KindPractice} }

func twoModules() []Module {
	return []Module{
		{ID: "M1", Weight: 0.5, Topics: []Topic{
			{ID: "T1.1", Weight: 0.5, Kinds: lecPrac()},
			{ID: "T1.2", Weight: 0.5, Kinds: lecPrac()},
		}},
		{ID: "M2", Weight: 0.5, Topics: []Topic{
			{ID: "T2.1", Weight: 0.5, Kinds: lecPrac()},
			{ID: "T2.2", Weight: 0.5, Kinds: lecPrac()},
		}},
	}
}

// checkFormulas verifies the contract identities on one row.
func checkFormulas(t *testing.T, label string, h Hours) {
	t.Helper()
	if h.Contact != h.Lecture+h.Practice+h.Lab {
		t.Errorf("%s: contact %v != lecture+practice+lab %v", label, h.Contact, h.Lecture+h.Practice+h.Lab)
	}
	if h.Total != h.Contact+h.SelfStudy+h.Assessment {
		t.Errorf("%s: total %v != contact+selfStudy+assessment %v", label, h.Total, h.Contact+h.SelfStudy+h.Assessment)
	}
	for name, v := range map[string]float64{"lecture": h.Lecture, "practice": h.Practice, "lab": h.Lab,
		"selfStudy": h.SelfStudy, "assessment": h.Assessment} {
		if v < 0 || math.Mod(v*2, 1) != 0 {
			t.Errorf("%s: %s = %v is negative or not a multiple of 0.5", label, name, v)
		}
	}
}

// checkResult verifies every invariant the validator will check.
func checkResult(t *testing.T, in Input, cfg Config, res *Result) {
	t.Helper()

	var topicsSum, modulesSum float64
	for mi, m := range res.Modules {
		checkFormulas(t, m.ID, m.Hours)

		var inModule float64
		for _, topic := range m.Topics {
			checkFormulas(t, topic.ID, topic.Hours)
			if topic.Hours.Total < cfg.MinTopicHours {
				t.Errorf("%s: %v h is below the minimum %v h", topic.ID, topic.Hours.Total, cfg.MinTopicHours)
			}
			inModule += topic.Hours.Total
			topicsSum += topic.Hours.Total
		}
		if inModule != m.Hours.Total {
			t.Errorf("%s: topics add up to %v, module says %v", m.ID, inModule, m.Hours.Total)
		}
		if len(m.Topics) != len(in.Modules[mi].Topics) {
			t.Errorf("%s: topic count changed", m.ID)
		}
		modulesSum += m.Hours.Total
	}

	checkFormulas(t, "finalAttestation", res.FinalAttestation)
	checkFormulas(t, "totals", res.Totals)

	if res.Totals.Total != in.TotalHours {
		t.Errorf("totals.total = %v, want exactly %v", res.Totals.Total, in.TotalHours)
	}
	if modulesSum+res.FinalAttestation.Total != res.Totals.Total {
		t.Errorf("modules %v + attestation %v != totals %v", modulesSum, res.FinalAttestation.Total, res.Totals.Total)
	}
}

func TestAllocateSmallProgram(t *testing.T) {
	in := Input{TotalHours: 16, Modules: twoModules(), StudyForm: "очная"}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	if res.FinalAttestation.Assessment != 1 {
		t.Errorf("attestation = %v h, want 1 (5%% of 16 is below the 1 h minimum)", res.FinalAttestation.Assessment)
	}
	if res.Modules[0].Hours.Total != res.Modules[1].Hours.Total {
		t.Errorf("equal weights must give equal modules, got %v and %v",
			res.Modules[0].Hours.Total, res.Modules[1].Hours.Total)
	}
}

func TestFinalAttestationIsClamped(t *testing.T) {
	tests := []struct {
		total float64
		want  float64
	}{
		{16, 1},   // 0.8 -> clamped up to the 1 h minimum
		{72, 3.5}, // 3.6 -> nearest 0.5
		{500, 8},  // 25 -> clamped down to the 8 h maximum
	}
	for _, tt := range tests {
		res, err := Allocate(Input{TotalHours: tt.total, Modules: twoModules(), StudyForm: "очная"}, DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		if res.FinalAttestation.Assessment != tt.want {
			t.Errorf("%v h: attestation = %v, want %v", tt.total, res.FinalAttestation.Assessment, tt.want)
		}
	}
}

func TestKindsAreRespected(t *testing.T) {
	in := Input{TotalHours: 40, StudyForm: "очная", Modules: []Module{{
		ID: "M1", Weight: 1, Topics: []Topic{
			{ID: "T1.1", Weight: 1, Kinds: []Kind{KindLecture}},
			{ID: "T1.2", Weight: 1, Kinds: []Kind{KindLab}},
			{ID: "T1.3", Weight: 1, Kinds: []Kind{KindPractice, KindLab}},
			{ID: "T1.4", Weight: 1, Kinds: []Kind{KindSelfStudy}},
		}}}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	topics := res.Modules[0].Topics
	if h := topics[0].Hours; h.Practice+h.Lab+h.SelfStudy != 0 {
		t.Errorf("lecture-only topic has other kinds: %+v", h)
	}
	if h := topics[1].Hours; h.Lecture+h.Practice+h.SelfStudy != 0 {
		t.Errorf("lab-only topic has other kinds: %+v", h)
	}
	if h := topics[2].Hours; h.Practice == 0 || h.Lab == 0 || h.Lecture != 0 {
		t.Errorf("practice+lab topic should split between them: %+v", h)
	}
	if h := topics[3].Hours; h.SelfStudy != h.Total {
		t.Errorf("selfStudy-only topic: %+v", h)
	}
}

func TestSelfStudyGrowsForDistanceForms(t *testing.T) {
	mk := func(form string) float64 {
		in := Input{TotalHours: 72, StudyForm: form, Modules: []Module{{
			ID: "M1", Weight: 1, Topics: []Topic{
				{ID: "T1.1", Weight: 1, Kinds: []Kind{KindLecture, KindPractice, KindSelfStudy}},
			}}}}
		res, err := Allocate(in, DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		return res.Totals.SelfStudy
	}

	full, mixed, distance := mk("очная"), mk("очно-заочная"), mk("заочная")
	if !(full < mixed && mixed < distance) {
		t.Errorf("selfStudy should grow очная < очно-заочная < заочная, got %v, %v, %v", full, mixed, distance)
	}
}

func TestInternshipIsAllSelfStudy(t *testing.T) {
	in := Input{TotalHours: 72, IsInternship: true, StudyForm: "очная", Modules: twoModules()}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	if res.Totals.Lecture+res.Totals.Practice+res.Totals.Lab != 0 {
		t.Errorf("internship must have no contact hours: %+v", res.Totals)
	}
	if res.Totals.SelfStudy != 72-res.FinalAttestation.Assessment {
		t.Errorf("selfStudy = %v", res.Totals.SelfStudy)
	}
}

func TestMinPracticeShareIsReached(t *testing.T) {
	in := Input{TotalHours: 72, StudyForm: "очная", MinPracticeShare: f(0.8), Modules: twoModules()}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	share := (res.Totals.Practice + res.Totals.Lab) / res.Totals.Total
	if share < 0.8 {
		t.Errorf("practice share = %.3f, want >= 0.8 (warnings: %v)", share, res.Warnings)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
}

func TestMinPracticeShareUnreachableWarns(t *testing.T) {
	in := Input{TotalHours: 40, StudyForm: "очная", MinPracticeShare: f(0.5), Modules: []Module{{
		ID: "M1", Weight: 1, Topics: []Topic{{ID: "T1.1", Weight: 1, Kinds: []Kind{KindLecture}}},
	}}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	checkResult(t, in, DefaultConfig(), res)

	if len(res.Warnings) == 0 {
		t.Error("an unreachable practice share must be reported")
	}
}

func TestWeightsDecideTheSplit(t *testing.T) {
	in := Input{TotalHours: 100, StudyForm: "очная", Modules: []Module{
		{ID: "M1", Weight: 0.8, Topics: []Topic{{ID: "T1.1", Weight: 1, Kinds: lecPrac()}}},
		{ID: "M2", Weight: 0.2, Topics: []Topic{{ID: "T2.1", Weight: 1, Kinds: lecPrac()}}},
	}}

	res, err := Allocate(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if m1, m2 := res.Modules[0].Hours.Total, res.Modules[1].Hours.Total; m1 <= 3*m2 {
		t.Errorf("0.8 vs 0.2 should be about 4:1, got %v and %v", m1, m2)
	}
}

func TestNotEnoughHours(t *testing.T) {
	topics := make([]Topic, 20)
	for i := range topics {
		topics[i] = Topic{ID: "T", Weight: 1, Kinds: lecPrac()}
	}
	in := Input{TotalHours: 16, StudyForm: "очная", Modules: []Module{{ID: "M1", Weight: 1, Topics: topics}}}

	_, err := Allocate(in, DefaultConfig())
	if !errors.Is(err, ErrNotEnoughHours) {
		t.Errorf("err = %v, want ErrNotEnoughHours", err)
	}
}

func TestInvalidInput(t *testing.T) {
	good := func() Input { return Input{TotalHours: 16, StudyForm: "очная", Modules: twoModules()} }

	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{"zero hours", func(in *Input) { in.TotalHours = 0 }},
		{"no modules", func(in *Input) { in.Modules = nil }},
		{"zero module weight", func(in *Input) { in.Modules[0].Weight = 0 }},
		{"module without topics", func(in *Input) { in.Modules[0].Topics = nil }},
		{"zero topic weight", func(in *Input) { in.Modules[0].Topics[0].Weight = 0 }},
		{"topic without kinds", func(in *Input) { in.Modules[0].Topics[0].Kinds = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := good()
			tt.mutate(&in)
			if _, err := Allocate(in, DefaultConfig()); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

// Property test: any program the allocator accepts must satisfy every
// invariant, and the sum must be exact for every total from 16 to 500 h.
func TestAllocateProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	kindSets := [][]Kind{
		{KindLecture}, {KindPractice}, {KindLab}, {KindSelfStudy},
		{KindLecture, KindPractice}, {KindLecture, KindLab}, {KindPractice, KindLab},
		{KindLecture, KindPractice, KindSelfStudy}, {KindLecture, KindPractice, KindLab, KindSelfStudy},
	}
	forms := []string{"очная", "очно-заочная", "заочная"}
	cfg := DefaultConfig()

	accepted := 0
	for i := 0; i < 3000; i++ {
		in := Input{
			TotalHours:   float64(32+rng.Intn(969)) / 2, // 16.0 .. 500.0 in 0.5 steps
			StudyForm:    forms[rng.Intn(len(forms))],
			IsInternship: rng.Intn(10) == 0,
		}
		if rng.Intn(3) == 0 {
			in.MinPracticeShare = f(float64(rng.Intn(80)) / 100)
		}

		for m := 0; m < 1+rng.Intn(6); m++ {
			mod := Module{ID: "M", Weight: 0.01 + rng.Float64(), IsIntro: m == 0 && rng.Intn(2) == 0}
			for tp := 0; tp < 1+rng.Intn(6); tp++ {
				topic := Topic{ID: "T", Weight: 0.01 + rng.Float64(), Kinds: kindSets[rng.Intn(len(kindSets))]}
				if rng.Intn(4) == 0 {
					topic.PracticeShareHint = f(rng.Float64())
				}
				mod.Topics = append(mod.Topics, topic)
			}
			in.Modules = append(in.Modules, mod)
		}

		res, err := Allocate(in, cfg)
		if errors.Is(err, ErrNotEnoughHours) {
			continue
		}
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		accepted++
		checkResult(t, in, cfg, res)

		again, _ := Allocate(in, cfg)
		if !reflect.DeepEqual(res, again) {
			t.Fatalf("case %d: allocation is not deterministic", i)
		}
	}
	if accepted < 2000 {
		t.Errorf("only %d of 3000 random programs were accepted; the generator is too aggressive", accepted)
	}
}
