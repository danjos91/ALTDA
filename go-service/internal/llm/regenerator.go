package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	skel "github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// Regenerator is an LLM-backed skeleton.Regenerator.
type Regenerator struct {
	client *Client
}

func NewRegenerator(client *Client) skeleton.Regenerator {
	return &Regenerator{client: client}
}

type regenResponse struct {
	Modules []skel.Module `json:"modules"`
}

func (r *Regenerator) Regenerate(ctx context.Context, in skeleton.RegenerateInput) ([]skel.Module, error) {
	payload := map[string]any{
		"request":  in.Request,
		"comment":  in.Comment,
		"outcomes": in.Outcomes,
		"current":  in.Current,
		"targets":  in.Targets,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal input: %v", ErrUpstream, err)
	}
	user := "Пересобери модули из targets. JSON:\n" + string(body)

	var out regenResponse
	if err := r.client.ChatJSON(ctx, regenerateSystemPrompt, user, &out); err != nil {
		return nil, err
	}
	if len(out.Modules) == 0 {
		return nil, fmt.Errorf("%w: empty modules", ErrSchemaInvalid)
	}
	return out.Modules, nil
}
