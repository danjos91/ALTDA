package parse

import (
	parseservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/parse"
)

type parseRequest struct {
	SchemaVersion  string                       `json:"schemaVersion"`
	RawText        string                       `json:"rawText"`
	Clarifications []parseservice.Clarification `json:"clarifications"`
}

type parseResponse struct {
	SchemaVersion  string                       `json:"schemaVersion"`
	Params         parseservice.Params          `json:"params"`
	Missing        []string                     `json:"missing"`
	Clarifications []parseservice.Clarification `json:"clarifications"`
	GapsReport     []string                     `json:"gapsReport"`
}
