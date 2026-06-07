package python

import "context"

// TokenCountRequest mirrors POST /api/tokenizer/count in python-service.
type TokenCountRequest struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
}

type TokenCountResponse struct {
	TokenCount int     `json:"token_count"`
	Count      int     `json:"count"`
	Model      string  `json:"model,omitempty"`
	ElapsedMS  float64 `json:"elapsed_ms,omitempty"`
}

func CountTokens(ctx context.Context, req TokenCountRequest) (TokenCountResponse, error) {
	client := NewClient("")
	return client.CountTokens(ctx, req)
}

func (r *TokenCountResponse) normalize() {
	if r.TokenCount == 0 && r.Count != 0 {
		r.TokenCount = r.Count
	}
	r.Count = r.TokenCount
}
