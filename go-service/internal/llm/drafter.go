package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	skel "github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

// Drafter is an LLM-backed skeleton.Drafter.
type Drafter struct {
	client *Client
}

func NewDrafter(client *Client) skeleton.Drafter {
	return &Drafter{client: client}
}

type draftResponse struct {
	Variants []skel.Draft `json:"variants"`
}

func (d *Drafter) Draft(ctx context.Context, req skeleton.Request) ([]skel.Draft, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %v", ErrUpstream, err)
	}
	user := "Параметры программы (JSON). Верни {\"variants\":[...]} без hours.\n" + string(body)

	var out draftResponse
	if err := d.client.ChatJSON(ctx, skeletonSystemPrompt, user, &out); err != nil {
		return nil, err
	}
	if len(out.Variants) == 0 {
		return nil, fmt.Errorf("%w: empty variants", ErrSchemaInvalid)
	}
	for i := range out.Variants {
		out.Variants[i].Normalize()
	}
	return out.Variants, nil
}
