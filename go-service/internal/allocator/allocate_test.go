package allocator

import (
	"testing"

	"github.com/danjos91/ALTDA/go-service/internal/program"
)

func TestAllocateProgramSumsAndHalfSteps(t *testing.T) {
	for _, total := range []float64{16, 24, 72, 36.5} {
		prog := sampleProgram(total)
		AllocateProgram(prog, Options{})
		cur := program.Nested(prog, "curriculum")
		totals := program.HoursFromObject(program.EnsureObject(cur, "totals"))
		if program.Half(totals.Total) != program.Half(total) {
			t.Fatalf("totalHours=%v: totals.total=%v", total, totals.Total)
		}
		contact := totals.Lecture + totals.Practice + totals.Lab
		if program.Half(totals.Contact) != program.Half(contact) {
			t.Fatalf("contact identity failed: %v vs %v", totals.Contact, contact)
		}
		expectedTotal := contact + totals.SelfStudy + totals.Assessment
		if program.Half(totals.Total) != program.Half(expectedTotal) {
			t.Fatalf("total identity failed: %v vs %v", totals.Total, expectedTotal)
		}
		assertHalfSteps(t, totals)
		mods, _ := program.AsArray(cur["modules"])
		for _, m := range mods {
			mo, _ := program.AsObject(m)
			mh := program.HoursFromObject(program.EnsureObject(mo, "hours"))
			assertHalfSteps(t, mh)
			topics, _ := program.AsArray(mo["topics"])
			var sum program.Hours
			for _, topic := range topics {
				to, _ := program.AsObject(topic)
				th := program.HoursFromObject(program.EnsureObject(to, "hours"))
				assertHalfSteps(t, th)
				sum.Lecture += th.Lecture
				sum.Practice += th.Practice
				sum.Lab += th.Lab
				sum.SelfStudy += th.SelfStudy
			}
			sum.Assessment = mh.Assessment
			sum.Total = sum.Lecture + sum.Practice + sum.Lab + sum.SelfStudy + sum.Assessment
			if !program.EqualHalf(sum, mh, append(program.HourKeys, "total")) {
				t.Fatalf("module rollup mismatch: sum=%+v mod=%+v", sum, mh)
			}
		}
	}
}

func TestAllocateLeavesLockedRowsUntouched(t *testing.T) {
	prog := sampleProgram(24)
	AllocateProgram(prog, Options{})
	cur := program.Nested(prog, "curriculum")
	mods, _ := program.AsArray(cur["modules"])
	mod0, _ := program.AsObject(mods[0])
	topics, _ := program.AsArray(mod0["topics"])
	t0, _ := program.AsObject(topics[0])
	before := program.CloneObject(program.EnsureObject(t0, "hours"))
	t0["_meta"] = program.Object{"locked": true}

	// Change requested hours and reallocate — locked topic must stay.
	program.EnsureObject(prog, "header")["totalHours"] = 48.0
	AllocateProgram(prog, Options{LockedIDs: program.CollectLockedIDs(prog)})

	after := program.HoursFromObject(program.EnsureObject(t0, "hours"))
	beforeH := program.HoursFromObject(before)
	if !program.EqualHalf(beforeH, after, program.AllHourKeys) {
		t.Fatalf("locked topic hours changed: before=%+v after=%+v", beforeH, after)
	}
	totals := program.HoursFromObject(program.EnsureObject(cur, "totals"))
	if program.Half(totals.Total) != program.Half(48) {
		t.Fatalf("rebalance did not hit 48: got %v", totals.Total)
	}
}

func TestAllocateVariant(t *testing.T) {
	variant := program.Object{
		"key":  "A",
		"goal": "g",
		"modules": program.Array{
			program.Object{
				"id": "M1", "title": "M", "isIntro": false, "weight": 0.5,
				"topics": program.Array{
					program.Object{
						"id": "T1.1", "title": "t", "weight": 1,
						"kinds": program.Array{"lecture", "practice"}, "practiceShareHint": 0.6,
						"outcomeIds": program.Array{"C1.K1"},
					},
				},
			},
			program.Object{
				"id": "M2", "title": "M2", "isIntro": false, "weight": 0.5,
				"topics": program.Array{
					program.Object{
						"id": "T2.1", "title": "t2", "weight": 1,
						"kinds": program.Array{"lecture", "practice"}, "practiceShareHint": 0.7,
						"outcomeIds": program.Array{"C1.U1"},
					},
				},
			},
		},
		"finalAttestation": program.Object{"form": "защита", "description": "d"},
	}
	AllocateVariant(variant, 16)
	totals := program.HoursFromObject(mustObj(variant["totals"]))
	if program.Half(totals.Total) != program.Half(16) {
		t.Fatalf("variant totals=%v", totals.Total)
	}
	fa := program.HoursFromObject(program.EnsureObject(mustObj(variant["finalAttestation"]), "hours"))
	if fa.Total <= 0 {
		t.Fatal("final attestation hours missing")
	}
}

func sampleProgram(total float64) program.Object {
	return program.Object{
		"schemaVersion": "1.1",
		"header":        program.Object{"totalHours": total},
		"curriculum": program.Object{
			"modules": program.Array{
				program.Object{
					"id": "M1", "title": "M1", "isIntro": false, "weight": 0.6, "assessmentForm": "нет",
					"topics": program.Array{
						program.Object{
							"id": "T1.1", "title": "a", "weight": 0.5,
							"kinds": program.Array{"lecture", "practice"}, "practiceShareHint": 0.5,
							"outcomeIds": program.Array{"C1.K1"},
						},
						program.Object{
							"id": "T1.2", "title": "b", "weight": 0.5,
							"kinds": program.Array{"lecture", "lab"}, "practiceShareHint": 0.7,
							"outcomeIds": program.Array{"C1.U1"},
						},
					},
				},
				program.Object{
					"id": "M2", "title": "M2", "isIntro": false, "weight": 0.4, "assessmentForm": "нет",
					"topics": program.Array{
						program.Object{
							"id": "T2.1", "title": "c", "weight": 1,
							"kinds": program.Array{"lecture", "practice"}, "practiceShareHint": 0.6,
							"outcomeIds": program.Array{"C1.N1"},
						},
					},
				},
			},
		},
		"finalAttestation": program.Object{"form": "зачёт", "description": "d"},
	}
}

func assertHalfSteps(t *testing.T, h program.Hours) {
	t.Helper()
	for _, v := range []float64{h.Lecture, h.Practice, h.Lab, h.SelfStudy, h.Assessment, h.Contact, h.Total} {
		if program.Half(v)*1 != program.Half(v) {
			t.Fatalf("not half-step: %v", v)
		}
		if v*2 != float64(int(v*2+1e-9)) && v*2 != float64(int(v*2-1e-9)) {
			// half() already rounds; ensure FromHalf round-trip
			if program.FromHalf(program.Half(v)) != program.FromHalf(program.Half(v)) {
				t.Fatalf("bad half: %v", v)
			}
		}
		if program.Half(v) < 0 {
			t.Fatalf("negative: %v", v)
		}
	}
}

func mustObj(v any) program.Object {
	o, _ := program.AsObject(v)
	return o
}
