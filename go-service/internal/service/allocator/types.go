package allocator

type Kind string

const (
	KindLecture   Kind = "lecture"
	KindPractice  Kind = "practice"
	KindLab       Kind = "lab"
	KindSelfStudy Kind = "selfStudy"
)

type Topic struct {
	ID                string
	Weight            float64
	Kinds             []Kind
	PracticeShareHint *float64 // optional: share of the topic's contact work that is practice/lab

	// Fixed keeps the topic exactly as it is (a person locked it). The topic
	// takes part in the sums but is never reallocated.
	Fixed *Hours
}

type Module struct {
	ID      string
	Weight  float64
	IsIntro bool
	Topics  []Topic
}

type Input struct {
	TotalHours       float64
	Modules          []Module
	StudyForm        string // очная | очно-заочная | заочная
	IsInternship     bool   // all hours become selfStudy
	MinPracticeShare *float64

	// FixedAttestation keeps the final attestation at these hours instead of
	// computing them.
	FixedAttestation *float64
}

// Hours follows $defs/hours of the schema: 0.5-hour steps,
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

type TopicResult struct {
	ID    string
	Hours Hours
}

type ModuleResult struct {
	ID     string
	Hours  Hours
	Topics []TopicResult
}

type Result struct {
	Modules          []ModuleResult
	FinalAttestation Hours
	Totals           Hours
	Warnings         []string
}

// Config holds the tunable proportions. Calibrate them against the АЦР samples.
type Config struct {
	// Final attestation hours = share of the total, clamped to [Min, Max].
	FinalAttestationShare float64
	FinalAttestationMin   float64
	FinalAttestationMax   float64

	// MinTopicHours is the smallest allocation a topic may get.
	MinTopicHours float64

	// Share of a topic's non-selfStudy hours that is practice/lab.
	DefaultPracticeShare float64
	IntroPracticeShare   float64

	// Share of a topic's hours that is selfStudy, by study form. Only applies
	// to topics that list selfStudy among their kinds.
	SelfStudyShare map[string]float64
}

func DefaultConfig() Config {
	return Config{
		FinalAttestationShare: 0.05,
		FinalAttestationMin:   1,
		FinalAttestationMax:   8,
		MinTopicHours:         1,
		DefaultPracticeShare:  0.6,
		IntroPracticeShare:    0.3,
		SelfStudyShare: map[string]float64{
			"очная":        0.10,
			"очно-заочная": 0.25,
			"заочная":      0.50,
		},
	}
}
