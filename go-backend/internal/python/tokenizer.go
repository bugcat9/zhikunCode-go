package python

// TokenCountRequest mirrors POST /api/tokenizer/count in python-service.
type TokenCountRequest struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
}

type TokenCountResponse struct {
	Count int `json:"count"`
}
