package parse

import "context"

const (
	ProgramTypeDPPPK = "DPP_PK"
	ProgramTypeDOOP  = "DOOP_GENERAL"
)

type Service interface {
	Parse(ctx context.Context, in Input) (*Result, error)
}

type Input struct {
	RawText        string
	Clarifications []Clarification
}

type Clarification struct {
	Param    string  `json:"param"`
	Question string  `json:"question"`
	Answer   *string `json:"answer"`
}

// Params mirrors $defs/requestParams of the schema. A pointer is nil (JSON
// null) when the request does not state the value and it cannot be inferred.
type Params struct {
	ProgramType           *string  `json:"programType"`
	Title                 *string  `json:"title"`
	Domain                *string  `json:"domain"`
	TotalHours            *float64 `json:"totalHours"`
	Audience              *string  `json:"audience"`
	EntryLevel            *string  `json:"entryLevel"`
	StudyForm             *string  `json:"studyForm"`
	DistanceTech          *bool    `json:"distanceTech"`
	DurationWeeks         *int     `json:"durationWeeks"`
	SessionHours          *float64 `json:"sessionHours"`
	SessionsPerWeek       *float64 `json:"sessionsPerWeek"`
	MinPracticeShare      *float64 `json:"minPracticeShare"`
	FinalAttestationForm  *string  `json:"finalAttestationForm"`
	IsInternship          bool     `json:"isInternship"`
	DeliverablesRequested []string `json:"deliverablesRequested"`
	Constraints           []string `json:"constraints"`
}

type Result struct {
	Params         Params
	Missing        []string // required params that are still nil
	Clarifications []Clarification
	GapsReport     []string
}
