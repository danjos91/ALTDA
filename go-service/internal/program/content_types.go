package program

// Competency is read for its outcome IDs and profstandard links.
type Competency struct {
	ID                string                   `json:"id"`
	ProfStandardLinks []Node[ProfStandardLink] `json:"profStandardLinks,omitempty"`
	Knowledge         []Node[Outcome]          `json:"knowledge,omitempty"`
	Skills            []Node[Outcome]          `json:"skills,omitempty"`
	Abilities         []Node[Outcome]          `json:"abilities,omitempty"`
}

type Outcome struct {
	ID   string `json:"id"`
	Text string `json:"text,omitempty"`
}

type ProfStandardLink struct {
	RegistryRef       string `json:"registryRef"`
	LaborFunctionCode string `json:"laborFunctionCode"`
}

// OutcomeIDs lists the knowledge, skill and ability outcomes in that order.
func (c Competency) OutcomeIDs() []string {
	var ids []string
	for _, group := range [][]Node[Outcome]{c.Knowledge, c.Skills, c.Abilities} {
		for _, o := range group {
			ids = append(ids, o.V.ID)
		}
	}
	return ids
}

// Dpp is the block of a ДПП ПК program.
type Dpp struct {
	ProfStandards []Node[ProfStandard] `json:"profStandards,omitempty"`
}

type ProfStandard struct {
	RegistryRef    string                `json:"registryRef"`
	LaborFunctions []Node[LaborFunction] `json:"laborFunctions,omitempty"`
}

type LaborFunction struct {
	Code string `json:"code"`
}

// Calendar is the training schedule.
type Calendar struct {
	Sessions []Node[Session] `json:"sessions,omitempty"`
}

type Session struct {
	Number           int      `json:"number"`
	Week             *int     `json:"week,omitempty"`
	Hours            float64  `json:"hours"`
	TopicIDs         []string `json:"topicIds"`
	FinalAttestation *bool    `json:"finalAttestation,omitempty"`
}

type Lecture struct {
	ID             string              `json:"id"`
	TopicID        string              `json:"topicId"`
	SessionNumbers []int               `json:"sessionNumbers,omitempty"`
	Title          string              `json:"title"`
	Goal           string              `json:"goal,omitempty"`
	Plan           []Node[LectureStep] `json:"plan"`
	KeyTerms       []string            `json:"keyTerms,omitempty"`
	OutcomeIDs     []string            `json:"outcomeIds,omitempty"`
	SourceIDs      []string            `json:"sourceIds,omitempty"`
	ReferenceIDs   []string            `json:"referenceIds,omitempty"`
	Meta           *Node[Meta]         `json:"_meta,omitempty"`
}

type LectureStep struct {
	Step    string `json:"step"`
	Minutes int    `json:"minutes"`
}

type Assessment struct {
	Tests        []Node[Test]        `json:"tests,omitempty"`
	SeminarTasks []Node[SeminarTask] `json:"seminarTasks,omitempty"`
}

type Test struct {
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	Purpose       string           `json:"purpose"`
	ModuleID      *string          `json:"moduleId,omitempty"`
	PassThreshold *float64         `json:"passThreshold,omitempty"`
	Questions     []Node[Question] `json:"questions"`
	Meta          *Node[Meta]      `json:"_meta,omitempty"`
}

type Question struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"` // single | multiple | truefalse | short
	Text        string         `json:"text"`
	Options     []Node[Option] `json:"options,omitempty"`
	Correct     []string       `json:"correct"`
	Explanation string         `json:"explanation,omitempty"`
	OutcomeIDs  []string       `json:"outcomeIds"`
	TopicIDs    []string       `json:"topicIds,omitempty"`
	Difficulty  string         `json:"difficulty"`
	Points      *int           `json:"points,omitempty"`
}

type Option struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Feedback string `json:"feedback,omitempty"`
}

type SeminarTask struct {
	ID            string            `json:"id"`
	TopicID       string            `json:"topicId"`
	Title         string            `json:"title"`
	Format        string            `json:"format,omitempty"`
	Description   string            `json:"description"`
	Deliverable   string            `json:"deliverable"`
	DurationHours float64           `json:"durationHours"`
	OutcomeIDs    []string          `json:"outcomeIds"`
	Criteria      []Node[Criterion] `json:"criteria"`
	MaxScore      int               `json:"maxScore"`
	PassThreshold *int              `json:"passThreshold,omitempty"`
	Meta          *Node[Meta]       `json:"_meta,omitempty"`
}

type Criterion struct {
	Text      string `json:"text"`
	MaxPoints int    `json:"maxPoints"`
}

type Source struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Citation string   `json:"citation"`
	Verified bool     `json:"verified"`
	TopicIDs []string `json:"topicIds,omitempty"`
}

type Tool struct {
	Name          string   `json:"name"`
	Version       string   `json:"version,omitempty"`
	UsedInTopics  []string `json:"usedInTopics,omitempty"`
	Status        string   `json:"status,omitempty"` // current | outdated | unknown
	LatestVersion string   `json:"latestVersion,omitempty"`
}

type TemplateSection struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Status   string `json:"status"` // filled | placeholder | missing
	JSONPath string `json:"jsonPath,omitempty"`
}
