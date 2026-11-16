package router

import "github.com/thomas/go-ai-gateway/internal/types"

// EstimateMessagesTokens approximates prompt token count (~4 chars/token + message framing).
func EstimateMessagesTokens(msgs []types.ChatMessage) int {
	chars := 0
	for _, m := range msgs {
		chars += len(m.Role) + len(m.Content) + len(m.Name) + 12
	}
	if chars == 0 {
		return 1
	}
	return (chars + 3) / 4
}

// EstimateTextTokens approximates token count for a completion string.
func EstimateTextTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4
}

// ComputeCostMicros computes cost in micro-dollars ($1 = 1,000,000 micros), rounding up any non-zero cost.
func ComputeCostMicros(spec ModelSpec, promptTokens, completionTokens int) int64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	raw := int64(promptTokens)*spec.InputMicrosPer1M + int64(completionTokens)*spec.OutputMicrosPer1M
	if raw <= 0 {
		return 0
	}
	return (raw + 999999) / 1000000
}

// EstimateReserveMicros calculates the worst-case micro-dollar reservation before calling a model.
func EstimateReserveMicros(spec ModelSpec, promptTokens, requestedMaxTokens int) int64 {
	maxOut := requestedMaxTokens
	if maxOut <= 0 {
		maxOut = spec.DefaultMaxTokens
	}
	return ComputeCostMicros(spec, promptTokens, maxOut)
}

