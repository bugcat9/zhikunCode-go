package agent

import (
	"context"
	"fmt"
	"strings"
)

type Aggregator interface {
	Aggregate(ctx context.Context, req CoordinatorRequest, results []AgentResult) (string, error)
}

type SummaryAggregator struct{}

func NewSummaryAggregator() SummaryAggregator {
	return SummaryAggregator{}
}

func (SummaryAggregator) Aggregate(ctx context.Context, req CoordinatorRequest, results []AgentResult) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if len(results) == 0 {
		return "", ErrNoWorkerResults
	}

	var b strings.Builder
	title := strings.TrimSpace(req.Instruction)
	if title == "" {
		title = "multi-agent task"
	}

	b.WriteString("Coordinator summary for: ")
	b.WriteString(title)
	b.WriteString("\n\n")

	successes := 0
	failures := 0
	for _, result := range results {
		if result.Status == TaskStatusCompleted {
			successes++
			continue
		}
		failures++
	}

	b.WriteString(fmt.Sprintf("Workers completed: %d succeeded, %d failed.\n", successes, failures))

	for _, result := range results {
		b.WriteString("\n")
		b.WriteString("- ")
		b.WriteString(result.TaskID)
		if result.AgentID != "" {
			b.WriteString(" (")
			b.WriteString(result.AgentID)
			b.WriteString(")")
		}
		b.WriteString(": ")
		b.WriteString(string(result.Status))

		if strings.TrimSpace(result.Error) != "" {
			b.WriteString(" - ")
			b.WriteString(result.Error)
			continue
		}

		text := strings.TrimSpace(result.Text)
		if text != "" {
			b.WriteString("\n  ")
			b.WriteString(text)
		}
	}

	return b.String(), nil
}
