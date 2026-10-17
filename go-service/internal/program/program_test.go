package program

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func roundTrip(t *testing.T, in string) (Program, any, any) {
	t.Helper()

	var p Program
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var want, got any
	if err := json.Unmarshal([]byte(in), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	return p, want, got
}

func TestRoundTripKeepsUnknownFields(t *testing.T) {
	in := `{
		"schemaVersion": "1.1",
		"status": "skeleton_proposed",
		"competencies": [{"id": "C1", "title": "Анализ"}],
		"header": {"title": "Курс", "totalHours": 16, "studyForm": "очная", "authors": [], "approval": {"orderNumber": null}},
		"curriculum": {
			"modules": [{
				"id": "M1", "title": "Основы", "assessmentForm": "зачёт", "weight": 1,
				"hours": {"lecture": 1, "practice": 1, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 2, "total": 2},
				"topics": [{
					"id": "T1.1", "title": "Тема", "outcomeIds": ["C1.K1"], "doubts": ["версия"],
					"hours": {"lecture": 1, "practice": 1, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 2, "total": 2},
					"_meta": {"locked": true, "version": 3, "skill": "skeleton", "dependsOn": ["C1.K1"]}
				}]
			}],
			"totals": {"lecture": 1, "practice": 1, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 2, "total": 2}
		},
		"finalAttestation": {"form": "зачёт", "description": "x",
			"hours": {"lecture": 0, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 1, "contact": 0, "total": 1}},
		"lectures": [{"id": "L1.1"}]
	}`

	_, want, got := roundTrip(t, in)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip changed the program\nwant: %v\n got: %v", want, got)
	}
}

func TestRoundTripOfSchemaExamples(t *testing.T) {
	for _, path := range []string{
		"../../../schema/example_program.json",
		"../../../schema/example_dpp_skeleton.json",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("schema example not found: %v", err)
		}

		_, want, got := roundTrip(t, string(data))
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: round trip changed the program", path)
		}
	}
}

func TestEditsAreWrittenAndRestIsKept(t *testing.T) {
	in := `{"id": "p1", "header": {"title": "Курс", "totalHours": 16, "studyForm": "очная"},
		"curriculum": {"modules": [{"id": "M1", "title": "Основы", "topics": [
			{"id": "T1.1", "title": "Тема", "hours": {"lecture": 1, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 1, "total": 1}}
		], "hours": {"lecture": 1, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 1, "total": 1}}],
		"totals": {"lecture": 1, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 1, "total": 1}}}`

	var p Program
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	p.V.Curriculum.V.Modules[0].V.Topics[0].V.Hours.Lecture = 4
	p.V.Curriculum.V.Modules[0].V.Topics[0].V.Hours.Total = 4

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var back struct {
		ID         string `json:"id"`
		Curriculum struct {
			Modules []struct {
				Title  string `json:"title"`
				Topics []struct {
					Title string `json:"title"`
					Hours Hours  `json:"hours"`
				} `json:"topics"`
			} `json:"modules"`
		} `json:"curriculum"`
	}
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}

	topic := back.Curriculum.Modules[0].Topics[0]
	if topic.Hours.Lecture != 4 || topic.Hours.Total != 4 {
		t.Errorf("edit was not written: %+v", topic.Hours)
	}
	if back.ID != "p1" || back.Curriculum.Modules[0].Title != "Основы" || topic.Title != "Тема" {
		t.Errorf("untouched fields were lost: %+v", back)
	}
}

func TestAbsentOptionalBlocksStayAbsent(t *testing.T) {
	var p Program
	if err := json.Unmarshal([]byte(`{"id": "p1", "status": "intake"}`), &p); err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"header", "curriculum", "finalAttestation", "request"} {
		if _, present := got[key]; present {
			t.Errorf("%q must not appear when it was absent", key)
		}
	}
}

func TestIsLocked(t *testing.T) {
	var p Program
	in := `{"curriculum": {"modules": [{"id": "M1", "topics": [
		{"id": "T1.1", "_meta": {"locked": true}},
		{"id": "T1.2", "_meta": {"locked": false}},
		{"id": "T1.3"}]}]}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}

	topics := p.V.Curriculum.V.Modules[0].V.Topics
	want := []bool{true, false, false}
	for i, topic := range topics {
		if got := topic.V.IsLocked(); got != want[i] {
			t.Errorf("%s: IsLocked = %v, want %v", topic.V.ID, got, want[i])
		}
	}
}

func TestRoundTripKeepsActivityFields(t *testing.T) {
	row := `{"lecture": 2, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 2, "total": 2}`
	in := `{"id": "p", "status": "content_ready", "curriculum": {"modules": [{"id": "M1", "hours": ` + row + `, "topics": [
		{"id": "T1.1", "hours": ` + row + `, "activities": [
			{"kind": "lecture", "hours": 2, "content": "Введение", "delivery": "onsite"}]}]}], "totals": ` + row + `}}`

	_, want, got := roundTrip(t, in)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("activity fields were lost\nwant: %v\n got: %v", want, got)
	}
}

func marshalToMap(t *testing.T, p Program) map[string]any {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestUnchangedFieldsNeverGainZeroValues(t *testing.T) {
	// A lecture with only an id: its typed fields (title, plan...) must not be
	// invented when the program is written back.
	var p Program
	if err := json.Unmarshal([]byte(`{"id": "p", "lectures": [{"id": "L1.1"}]}`), &p); err != nil {
		t.Fatal(err)
	}

	lecture := marshalToMap(t, p)["lectures"].([]any)[0].(map[string]any)
	if len(lecture) != 1 || lecture["id"] != "L1.1" {
		t.Errorf("lecture = %v, want only its id", lecture)
	}
}

func TestChangingOneFieldWritesOnlyThatField(t *testing.T) {
	var p Program
	in := `{"id": "p", "header": {"title": "Курс", "totalHours": 16, "studyForm": "очная", "authors": []}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	p.V.Header.V.TotalHours = 40

	header := marshalToMap(t, p)["header"].(map[string]any)
	if header["totalHours"] != 40.0 {
		t.Errorf("totalHours = %v, want 40", header["totalHours"])
	}
	if header["title"] != "Курс" || header["studyForm"] != "очная" || header["authors"] == nil {
		t.Errorf("untouched fields changed: %v", header)
	}
}

func TestAFieldTheViewDropsIsRemoved(t *testing.T) {
	var p Program
	in := `{"id": "p", "curriculum": {"modules": [{"id": "M1", "topics": [
		{"id": "T1.1", "title": "Тема", "hours": {}, "prerequisites": ["T0.1"]}]}], "totals": {}}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	p.V.Curriculum.V.Modules[0].V.Topics[0].V.Prerequisites = nil // omitempty: the view no longer encodes it

	topic := marshalToMap(t, p)["curriculum"].(map[string]any)["modules"].([]any)[0].(map[string]any)["topics"].([]any)[0].(map[string]any)
	if _, present := topic["prerequisites"]; present {
		t.Errorf("prerequisites should have been removed: %v", topic)
	}
	if topic["title"] != "Тема" {
		t.Errorf("other fields must stay: %v", topic)
	}
}

func TestRequiredListsAreWrittenEvenWhenEmpty(t *testing.T) {
	var p Program
	if err := json.Unmarshal([]byte(`{"id": "p", "status": "skeleton_approved"}`), &p); err != nil {
		t.Fatal(err)
	}
	p.V.Sources = []Node[Source]{}
	p.V.NormativeBasis = []string{}
	p.V.Lectures = []Node[Lecture]{}

	out := marshalToMap(t, p)
	for _, key := range []string{"sources", "normativeBasis", "lectures"} {
		list, present := out[key].([]any)
		if !present || len(list) != 0 {
			t.Errorf("%s = %v, want an empty list", key, out[key])
		}
	}
}

func TestNodeBuiltFromScratchWritesAllItsFields(t *testing.T) {
	p := Program{V: Root{Lectures: []Node[Lecture]{{V: Lecture{
		ID: "L1.1", TopicID: "T1.1", Title: "Лекция",
		Plan: []Node[LectureStep]{{V: LectureStep{Step: "Введение", Minutes: 45}}},
	}}}}}

	lecture := marshalToMap(t, p)["lectures"].([]any)[0].(map[string]any)
	if lecture["id"] != "L1.1" || lecture["topicId"] != "T1.1" || lecture["title"] != "Лекция" {
		t.Errorf("lecture = %v", lecture)
	}
	step := lecture["plan"].([]any)[0].(map[string]any)
	if step["minutes"] != 45.0 {
		t.Errorf("step = %v", step)
	}
}

func TestNestedChangeKeepsSiblingsAndUnknownFields(t *testing.T) {
	var p Program
	row := `{"lecture": 1, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 1, "total": 1}`
	in := `{"id": "p", "curriculum": {"modules": [{"id": "M1", "annotation": "заметка", "hours": ` + row + `, "topics": [
		{"id": "T1.1", "hours": ` + row + `, "doubts": ["x"]},
		{"id": "T1.2", "hours": ` + row + `}]}], "totals": ` + row + `}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	p.V.Curriculum.V.Modules[0].V.Topics[1].V.Hours.Lecture = 5

	module := marshalToMap(t, p)["curriculum"].(map[string]any)["modules"].([]any)[0].(map[string]any)
	topics := module["topics"].([]any)
	if topics[1].(map[string]any)["hours"].(map[string]any)["lecture"] != 5.0 {
		t.Errorf("nested change not written: %v", topics[1])
	}
	if topics[0].(map[string]any)["doubts"] == nil || module["annotation"] != "заметка" {
		t.Errorf("unknown fields were lost: %v", module)
	}
}
