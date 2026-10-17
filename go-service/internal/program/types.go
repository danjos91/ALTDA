package program

// Program is the root of program.json.
type Program = Node[Root]

// Root lists the top-level blocks Go reads. Optional blocks are pointers:
// nil means the key is absent from the program.
type Root struct {
	Status           string                  `json:"status,omitempty"`
	ProgramType      string                  `json:"programType,omitempty"`
	Request          *Node[Request]          `json:"request,omitempty"`
	Header           *Node[Header]           `json:"header,omitempty"`
	Competencies     []Node[Competency]      `json:"competencies,omitempty"`
	Dpp              *Node[Dpp]              `json:"dpp,omitempty"`
	Curriculum       *Node[Curriculum]       `json:"curriculum,omitempty"`
	FinalAttestation *Node[FinalAttestation] `json:"finalAttestation,omitempty"`
	Calendar         *Node[Calendar]         `json:"calendar,omitempty"`
	Assessment       *Node[Assessment]       `json:"assessment,omitempty"`
	Tools            []Node[Tool]            `json:"tools,omitempty"`

	// Required from content_ready on, so no omitempty: when Go sets one of them
	// (even to an empty list) it is written as [].
	TemplateSections []Node[TemplateSection] `json:"templateSections"`
	Lectures         []Node[Lecture]         `json:"lectures"`
	Sources          []Node[Source]          `json:"sources"`
	NormativeBasis   []string                `json:"normativeBasis"`
}

type Request struct {
	Params      *Node[Params]      `json:"params,omitempty"`
	Attachments []Node[Attachment] `json:"attachments,omitempty"`
}

// Attachment is a reference supplied by the methodologist.
type Attachment struct {
	ID string `json:"id"`
}

// Params is the part of request.params Go reads. A nil pointer means the
// request did not state the value (JSON null or absent).
type Params struct {
	ProgramType          *string  `json:"programType,omitempty"`
	Title                *string  `json:"title,omitempty"`
	Domain               *string  `json:"domain,omitempty"`
	TotalHours           *float64 `json:"totalHours,omitempty"`
	Audience             *string  `json:"audience,omitempty"`
	EntryLevel           *string  `json:"entryLevel,omitempty"`
	StudyForm            *string  `json:"studyForm,omitempty"`
	FinalAttestationForm *string  `json:"finalAttestationForm,omitempty"`
	MinPracticeShare     *float64 `json:"minPracticeShare,omitempty"`
	IsInternship         *bool    `json:"isInternship,omitempty"`
	SessionHours         *float64 `json:"sessionHours,omitempty"`
	SessionsPerWeek      *float64 `json:"sessionsPerWeek,omitempty"`
}

type Header struct {
	Title       string  `json:"title,omitempty"`
	ItDirection string  `json:"itDirection,omitempty"`
	TotalHours  float64 `json:"totalHours"`
	StudyForm   string  `json:"studyForm"`
}

type Curriculum struct {
	Modules []Node[Module] `json:"modules"`
	Totals  Hours          `json:"totals"`
}

type Module struct {
	ID      string        `json:"id"`
	Title   string        `json:"title,omitempty"`
	IsIntro *bool         `json:"isIntro,omitempty"`
	Weight  *float64      `json:"weight,omitempty"`
	Hours   Hours         `json:"hours"`
	Topics  []Node[Topic] `json:"topics"`
	Meta    *Node[Meta]   `json:"_meta,omitempty"`
}

type Topic struct {
	ID            string      `json:"id"`
	Title         string      `json:"title,omitempty"`
	OutcomeIDs    []string    `json:"outcomeIds,omitempty"`
	Prerequisites []string    `json:"prerequisites,omitempty"`
	ReferenceIDs  []string    `json:"referenceIds,omitempty"`
	Weight        *float64    `json:"weight,omitempty"`
	Hours         Hours       `json:"hours"`
	Meta          *Node[Meta] `json:"_meta,omitempty"`

	// Activities is filled by the content step. Each is a Node so the fields
	// Go does not read (content, delivery) survive a round trip.
	Activities []Node[Activity] `json:"activities,omitempty"`
}

// Activity is one work kind of a topic. Kind "control" counts as assessment.
type Activity struct {
	Kind     string  `json:"kind"`
	Hours    float64 `json:"hours"`
	Content  string  `json:"content"`
	Delivery string  `json:"delivery,omitempty"`
}

type FinalAttestation struct {
	Form       string      `json:"form,omitempty"`
	OutcomeIDs []string    `json:"outcomeIds,omitempty"`
	Hours      Hours       `json:"hours"`
	Meta       *Node[Meta] `json:"_meta,omitempty"`
}

// Meta is the provenance block of a node ($defs/nodeMeta).
type Meta struct {
	Locked *bool `json:"locked,omitempty"`
}

// Hours follows $defs/hours: 0.5-hour steps,
// contact = lecture+practice+lab, total = contact+selfStudy+assessment.
type Hours struct {
	Lecture    float64 `json:"lecture"`
	Practice   float64 `json:"practice"`
	Lab        float64 `json:"lab"`
	SelfStudy  float64 `json:"selfStudy"`
	Assessment float64 `json:"assessment"`
	Contact    float64 `json:"contact"`
	Total      float64 `json:"total"`
}

// IsLocked reports whether a person locked the node (_meta.locked = true).
func (m *Module) IsLocked() bool { return isLocked(m.Meta) }
func (t *Topic) IsLocked() bool  { return isLocked(t.Meta) }

func (f *FinalAttestation) IsLocked() bool { return isLocked(f.Meta) }

func isLocked(meta *Node[Meta]) bool {
	return meta != nil && meta.V.Locked != nil && *meta.V.Locked
}
