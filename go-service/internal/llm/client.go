package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Client talks to an OpenAI-compatible chat API behind a process-wide semaphore.
type Client struct {
	api       *openai.Client
	model     string
	maxTokens int
	callTO    time.Duration
	sem       chan struct{}
}

var (
	globalSem     chan struct{}
	globalSemSize int
	globalSemMu   sync.Mutex
)

// sharedSemaphore returns one process-wide semaphore of the given size.
func sharedSemaphore(size int) chan struct{} {
	globalSemMu.Lock()
	defer globalSemMu.Unlock()
	if globalSem == nil || globalSemSize != size {
		globalSem = make(chan struct{}, size)
		globalSemSize = size
	}
	return globalSem
}

// ResetSharedSemaphoreForTest clears the process-wide semaphore (tests only).
func ResetSharedSemaphoreForTest() {
	globalSemMu.Lock()
	defer globalSemMu.Unlock()
	globalSem = nil
	globalSemSize = 0
}

// NewClient builds an OpenAI-compatible client. cfg.Stub must be false.
func NewClient(cfg Config) *Client {
	ocfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		ocfg.BaseURL = cfg.BaseURL
	}
	return &Client{
		api:       openai.NewClientWithConfig(ocfg),
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
		callTO:    cfg.CallTimeout,
		sem:       sharedSemaphore(cfg.Concurrency),
	}
}

// ChatJSON sends a non-streaming chat completion and unmarshals JSON into dest.
// Bad JSON is retried once; a second failure returns ErrSchemaInvalid.
// 429/5xx use exponential backoff (up to 3 attempts), honouring ctx.
func (c *Client) ChatJSON(ctx context.Context, system, user string, dest any) error {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		Temperature: 0.2,
		MaxTokens:   c.maxTokens,
		Stream:      false,
	}

	var lastSchema error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		text, err := c.complete(ctx, req)
		if err != nil {
			return err
		}
		text = stripFences(text)
		if err := json.Unmarshal([]byte(text), dest); err != nil {
			lastSchema = fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Previous response was not valid JSON: " + err.Error() + ". Return JSON only, no markdown.",
			})
			continue
		}
		return nil
	}
	if lastSchema == nil {
		return ErrSchemaInvalid
	}
	return lastSchema
}

func (c *Client) complete(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	var lastErr error
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("%w: %v", ErrTimeout, err)
		}

		if err := c.acquire(ctx); err != nil {
			return "", err
		}

		callCtx, cancel := context.WithTimeout(ctx, c.callTO)
		resp, err := c.api.CreateChatCompletion(callCtx, req)
		cancel()
		c.release()

		if err != nil {
			// Context cancellation/deadline first; then retryable HTTP (429/5xx
			// including 504 Gateway Timeout) before string-matching "timeout".
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return "", fmt.Errorf("%w: %v", ErrTimeout, err)
			}
			if isRetryable(err) && attempt < 2 {
				lastErr = fmt.Errorf("%w: %v", ErrUpstream, err)
				if err := sleepCtx(ctx, backoff); err != nil {
					return "", fmt.Errorf("%w: %v", ErrTimeout, err)
				}
				backoff *= 2
				continue
			}
			if isTimeout(err) {
				return "", fmt.Errorf("%w: %v", ErrTimeout, err)
			}
			return "", fmt.Errorf("%w: %v", ErrUpstream, err)
		}
		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("%w: empty choices", ErrUpstream)
			if attempt < 2 {
				if err := sleepCtx(ctx, backoff); err != nil {
					return "", fmt.Errorf("%w: %v", ErrTimeout, err)
				}
				backoff *= 2
				continue
			}
			return "", lastErr
		}
		return strings.TrimSpace(resp.Choices[0].Message.Content), nil
	}
	if lastErr == nil {
		lastErr = ErrUpstream
	}
	return "", lastErr
}

func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrTimeout, ctx.Err())
	}
}

func (c *Client) release() {
	select {
	case <-c.sem:
	default:
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline")
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatusCode == http.StatusTooManyRequests || apiErr.HTTPStatusCode >= 500 {
			return true
		}
	}
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		if reqErr.HTTPStatusCode == http.StatusTooManyRequests || reqErr.HTTPStatusCode >= 500 {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "rate limit")
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}
