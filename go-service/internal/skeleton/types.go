// Package skeleton holds the types of the skeleton step: a Draft is what a
// drafter (template or LLM) proposes, with weights and no hours.
// It mirrors $defs/skeletonDraft of the schema.
package skeleton

type Draft struct {
	Key              string           `json:"key"`
	Rationale        string           `json:"rationale"`
	Pros             []string         `json:"pros"`
	Cons             []string         `json:"cons"`
	Goal             string           `json:"goal"`
	Competencies     []Competency     `json:"competencies"`
	Modules          []Module         `json:"modules"`
	FinalAttestation FinalAttestation `json:"finalAttestation"`
}

type Competency struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Kind              string             `json:"kind,omitempty"` // новая | совершенствуемая
	ActivityType      *string            `json:"activityType"`
	ProfStandardLinks []ProfStandardLink `json:"profStandardLinks"`
	Knowledge         []Outcome          `json:"knowledge"`
	Skills            []Outcome          `json:"skills"`
	Abilities         []Outcome          `json:"abilities"`
}

// ProfStandardLink carries registry IDs only; a drafter must never invent one.
type ProfStandardLink struct {
	RegistryRef       string `json:"registryRef"`
	LaborFunctionCode string `json:"laborFunctionCode"`
}

type Outcome struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Module struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	IsIntro bool    `json:"isIntro"`
	Weight  float64 `json:"weight"` // share of all modules; the weights add up to 1
	Topics  []Topic `json:"topics"`
}

type Topic struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Weight            float64  `json:"weight"` // share inside the module; adds up to 1
	Kinds             []string `json:"kinds"`  // lecture | practice | lab | selfStudy
	PracticeShareHint *float64 `json:"practiceShareHint,omitempty"`
	OutcomeIDs        []string `json:"outcomeIds"`
	Prerequisites     []string `json:"prerequisites"`
}

type FinalAttestation struct {
	Form        string   `json:"form"`
	Description string   `json:"description"`
	OutcomeIDs  []string `json:"outcomeIds"`
}

// OutcomeIDs lists every outcome the competencies define.
func (d Draft) OutcomeIDs() []string {
	var ids []string
	for _, c := range d.Competencies {
		for _, group := range [][]Outcome{c.Knowledge, c.Skills, c.Abilities} {
			for _, o := range group {
				ids = append(ids, o.ID)
			}
		}
	}
	return ids
}

// Normalize makes every list non-nil, so it encodes as [] and not null.
func (d *Draft) Normalize() {
	d.Pros = nonNil(d.Pros)
	d.Cons = nonNil(d.Cons)
	d.FinalAttestation.OutcomeIDs = nonNil(d.FinalAttestation.OutcomeIDs)
	for i := range d.Competencies {
		c := &d.Competencies[i]
		if c.ProfStandardLinks == nil {
			c.ProfStandardLinks = []ProfStandardLink{}
		}
		c.Knowledge = nonNilOutcomes(c.Knowledge)
		c.Skills = nonNilOutcomes(c.Skills)
		c.Abilities = nonNilOutcomes(c.Abilities)
	}
	for mi := range d.Modules {
		for ti := range d.Modules[mi].Topics {
			t := &d.Modules[mi].Topics[ti]
			t.Kinds = nonNil(t.Kinds)
			t.OutcomeIDs = nonNil(t.OutcomeIDs)
			t.Prerequisites = nonNil(t.Prerequisites)
		}
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilOutcomes(o []Outcome) []Outcome {
	if o == nil {
		return []Outcome{}
	}
	return o
}
