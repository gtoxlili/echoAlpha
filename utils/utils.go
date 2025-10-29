package utils

import (
	"fmt"
	"unsafe"

	json "github.com/bytedance/sonic"
	"github.com/kaptinlin/jsonrepair"
	"github.com/openai/openai-go/v2"
	"github.com/samber/lo"
)

func ParseResult[T any](completion *openai.ChatCompletion) (T, error) {
	responseContent := completion.Choices[0].Message.Content
	repaired, err := jsonrepair.JSONRepair(responseContent)
	if err != nil {
		return lo.Empty[T](), fmt.Errorf("failed to repair JSON: %w", err)
	}

	var result T
	if err := json.Unmarshal(unsafe.Slice(unsafe.StringData(repaired), len(repaired)), &result); err != nil {
		return lo.Empty[T](), fmt.Errorf("failed to parse analysis result: %w", err)
	}
	return result, nil
}
