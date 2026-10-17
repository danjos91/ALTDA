package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
)

// Generator is an LLM-backed content.Generator (one topic per call).
type Generator struct {
	client *Client
}

func NewGenerator(client *Client) content.Generator {
	return &Generator{client: client}
}

func (g *Generator) Placeholder() bool { return false }

type topicResponse struct {
	Activities []program.Activity `json:"activities"`
	Lecture    *program.Lecture   `json:"lecture"`
	Questions  []topicQuestion    `json:"questions"`
	Task       *topicTask         `json:"task"`
}

type topicQuestion struct {
	Type        string           `json:"type"`
	Text        string           `json:"text"`
	Options     []program.Option `json:"options"`
	Correct     []string         `json:"correct"`
	Explanation string           `json:"explanation"`
	OutcomeIDs  []string         `json:"outcomeIds"`
	Difficulty  string           `json:"difficulty"`
}

type topicTask struct {
	Title         string              `json:"title"`
	Description   string              `json:"description"`
	Deliverable   string              `json:"deliverable"`
	DurationHours float64             `json:"durationHours"`
	OutcomeIDs    []string            `json:"outcomeIds"`
	Criteria      []program.Criterion `json:"criteria"`
	MaxScore      int                 `json:"maxScore"`
}

func (g *Generator) Topic(ctx context.Context, in content.TopicInput) (content.TopicContent, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return content.TopicContent{}, fmt.Errorf("%w: marshal topic: %v", ErrUpstream, err)
	}
	user := "Заполни содержание темы. JSON входа:\n" + string(body)

	var raw topicResponse
	if err := g.client.ChatJSON(ctx, topicSystemPrompt, user, &raw); err != nil {
		return content.TopicContent{}, err
	}
	if err := validateTopicResponse(in, raw); err != nil {
		return content.TopicContent{}, err
	}

	out := content.TopicContent{
		Activities: raw.Activities,
		Lecture:    raw.Lecture,
	}
	for _, q := range raw.Questions {
		out.Questions = append(out.Questions, program.Question{
			Type:        q.Type,
			Text:        q.Text,
			Options:     nodesOf(q.Options),
			Correct:     q.Correct,
			Explanation: q.Explanation,
			OutcomeIDs:  q.OutcomeIDs,
			Difficulty:  q.Difficulty,
		})
	}
	if raw.Task != nil {
		out.Task = &program.SeminarTask{
			Title:         raw.Task.Title,
			Description:   raw.Task.Description,
			Deliverable:   raw.Task.Deliverable,
			DurationHours: raw.Task.DurationHours,
			OutcomeIDs:    raw.Task.OutcomeIDs,
			Criteria:      criterionNodes(raw.Task.Criteria),
			MaxScore:      raw.Task.MaxScore,
		}
	}
	return out, nil
}

func validateTopicResponse(in content.TopicInput, raw topicResponse) error {
	if len(raw.Activities) == 0 {
		h := in.Topic.Hours
		if h.Lecture+h.Practice+h.Lab+h.SelfStudy+h.Assessment > 0 {
			return fmt.Errorf("%w: missing activities", ErrSchemaInvalid)
		}
	}
	for _, a := range raw.Activities {
		if a.Kind == "" || a.Content == "" {
			return fmt.Errorf("%w: activity missing kind/content", ErrSchemaInvalid)
		}
	}
	return nil
}

func nodesOf(opts []program.Option) []program.Node[program.Option] {
	out := make([]program.Node[program.Option], 0, len(opts))
	for _, o := range opts {
		out = append(out, program.Node[program.Option]{V: o})
	}
	return out
}

func criterionNodes(c []program.Criterion) []program.Node[program.Criterion] {
	out := make([]program.Node[program.Criterion], 0, len(c))
	for _, x := range c {
		out = append(out, program.Node[program.Criterion]{V: x})
	}
	return out
}
