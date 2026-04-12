package run

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"council/internal/model"
)

var itemTypeOrder = []string{
	model.ItemTypeClaim,
	model.ItemTypeRisk,
	model.ItemTypeRecommendation,
	model.ItemTypeQuestion,
}

func ExtractItems(outputs []model.AgentOutput) []model.Item {
	items := make([]model.Item, 0)
	itemIndex := map[string]int{}

	for _, output := range outputs {
		if strings.TrimSpace(output.Content) == "" {
			continue
		}

		for _, candidate := range extractCandidates(output.Content) {
			itemType, content, ok := classifyCandidate(candidate)
			if !ok {
				continue
			}

			normalized := itemType + "|" + normalizeItemContent(content)
			if normalized == itemType+"|" {
				continue
			}

			if index, ok := itemIndex[normalized]; ok {
				if !slices.Contains(items[index].SourceAgents, output.AgentName) {
					items[index].SourceAgents = append(items[index].SourceAgents, output.AgentName)
				}
				continue
			}

			items = append(items, model.Item{
				ID:           fmt.Sprintf("item-%03d", len(items)+1),
				Type:         itemType,
				Content:      content,
				SourceAgents: []string{output.AgentName},
				Status:       model.ItemStatusOpen,
			})
			itemIndex[normalized] = len(items) - 1
		}
	}

	return items
}

func extractCandidates(content string) []string {
	lines := strings.Split(content, "\n")
	candidates := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		trimmed = stripListMarker(trimmed)
		if trimmed == "" || strings.HasSuffix(trimmed, ":") {
			continue
		}

		for _, sentence := range splitCandidateLine(trimmed) {
			if shouldSkipCandidate(sentence) {
				continue
			}

			candidates = append(candidates, sentence)
		}
	}

	return candidates
}

func splitCandidateLine(line string) []string {
	line = cleanWhitespace(line)
	if line == "" {
		return nil
	}

	parts := make([]string, 0)
	start := 0
	for index, r := range line {
		if r != '.' && r != '!' && r != '?' && r != ';' {
			continue
		}

		part := sanitizeCandidate(line[start : index+1])
		if part != "" {
			parts = append(parts, part)
		}
		start = index + 1
	}

	tail := sanitizeCandidate(line[start:])
	if tail != "" {
		parts = append(parts, tail)
	}

	if len(parts) == 0 {
		return []string{sanitizeCandidate(line)}
	}

	return parts
}

func classifyCandidate(candidate string) (string, string, bool) {
	content := cleanWhitespace(candidate)
	if content == "" {
		return "", "", false
	}

	lower := strings.ToLower(content)

	switch {
	case strings.HasSuffix(content, "?") || hasAnyPrefix(lower, "question:", "open question:"):
		return model.ItemTypeQuestion, trimLabelPrefix(content, "question:", "open question:"), true
	case hasRiskMarker(lower):
		return model.ItemTypeRisk, trimLabelPrefix(content, "risk:", "risks:", "concern:", "concerns:", "failure mode:", "failure modes:", "regression:", "edge case:", "edge cases:", "hidden assumption:", "hidden assumptions:"), true
	case hasRecommendationMarker(lower):
		return model.ItemTypeRecommendation, trimLabelPrefix(content, "recommendation:", "recommendations:", "recommend:", "suggestion:", "suggestions:", "next step:", "next steps:"), true
	default:
		return model.ItemTypeClaim, trimLabelPrefix(content, "claim:", "claims:", "finding:", "findings:", "observation:", "observations:", "note:"), true
	}
}

func buildSynthesisPrompt(prompt string, artifacts []model.Artifact, outputs []model.AgentOutput, items []model.Item) string {
	var body strings.Builder
	body.WriteString("You are synthesizing multiple agent responses into one final answer.\n\n")
	body.WriteString(buildTaskContext(prompt, artifacts))
	body.WriteString("\n\n")

	if len(items) > 0 {
		body.WriteString("Normalized items:\n")
		for _, itemType := range itemTypeOrder {
			typedItems := filterItemsByType(items, itemType)
			if len(typedItems) == 0 {
				continue
			}

			body.WriteString("\n## ")
			body.WriteString(titleForItemType(itemType))
			body.WriteString("\n")
			for _, item := range typedItems {
				body.WriteString("- ")
				body.WriteString(item.Content)
				if len(item.SourceAgents) > 0 {
					body.WriteString(" [sources: ")
					body.WriteString(strings.Join(item.SourceAgents, ", "))
					body.WriteString("]")
				}
				body.WriteString("\n")
			}
		}
		body.WriteString("\nUse the normalized items first. Consult the raw agent outputs only if you need missing nuance or detail.\n")
	} else {
		body.WriteString("No normalized items were extracted reliably. Use the raw agent outputs below.\n")
	}

	body.WriteString("\nRaw agent outputs (fallback context):\n")
	for _, output := range outputs {
		body.WriteString("\n## ")
		body.WriteString(output.AgentName)
		body.WriteString("\n")
		body.WriteString(strings.TrimSpace(output.Content))
		body.WriteString("\n")
	}

	body.WriteString("\nProduce one final answer that follows any explicit output format, section order, heading text, or stopping rule from the original task exactly. If the original task does not specify an output format, use concise Markdown. Resolve disagreements where possible and note remaining uncertainty briefly when it matters.\n")

	return body.String()
}

func buildRoundPrompt(
	originalPrompt string,
	artifacts []model.Artifact,
	round int,
	agentName string,
	previousOutput *model.AgentOutput,
	items []model.Item,
	teamSize int,
) string {
	if round == 0 {
		return buildTaskContext(originalPrompt, artifacts)
	}

	var body strings.Builder
	body.WriteString("Critique/revise round ")
	body.WriteString(fmt.Sprintf("%d", round+1))
	body.WriteString(".\n\n")
	body.WriteString(buildTaskContext(originalPrompt, artifacts))
	body.WriteString("\n\n")

	if previousOutput != nil && strings.TrimSpace(previousOutput.Content) != "" {
		body.WriteString("Your previous answer:\n")
		body.WriteString(strings.TrimSpace(previousOutput.Content))
		body.WriteString("\n\n")
	}

	if len(items) > 0 {
		body.WriteString("Shared normalized items from the prior round:\n")
		body.WriteString(formatItemGroups(items))
		body.WriteString("\n")

		focusItems := buildFocusItems(agentName, items, teamSize)
		if len(focusItems) > 0 {
			body.WriteString("Focus items for critique:\n")
			for _, item := range focusItems {
				body.WriteString("- [")
				body.WriteString(item.Type)
				body.WriteString("] ")
				body.WriteString(item.Content)
				if len(item.SourceAgents) > 0 {
					body.WriteString(" [sources: ")
					body.WriteString(strings.Join(item.SourceAgents, ", "))
					body.WriteString("]")
				}
				body.WriteString("\n")
			}
			body.WriteString("\n")
		}
	} else {
		body.WriteString("No normalized items were extracted from the prior round. Improve the answer directly.\n\n")
	}

	body.WriteString("Instructions:\n")
	body.WriteString("1. Critique weak assumptions, gaps, and edge cases relevant to your role.\n")
	body.WriteString("2. Revise your answer using the normalized items above.\n")
	body.WriteString("3. Return one revised answer in the format required by the original task. If the original task does not specify a format, use Markdown. Do not return a transcript of the protocol.\n")

	return body.String()
}

func buildTaskContext(prompt string, artifacts []model.Artifact) string {
	var body strings.Builder
	body.WriteString("Original task:\n")
	body.WriteString(strings.TrimSpace(prompt))

	if len(artifacts) == 0 {
		return body.String()
	}

	body.WriteString("\n\nAttached local artifacts:\n")
	for _, artifact := range artifacts {
		body.WriteString("\n## ")
		body.WriteString(artifact.Path)
		body.WriteString("\n")
		body.WriteString("content type: ")
		body.WriteString(artifact.ContentType)
		body.WriteString("\n")
		body.WriteString("size: ")
		body.WriteString(fmt.Sprintf("%d", artifact.Size))
		body.WriteString(" bytes\n")
		body.WriteString("sha256: ")
		body.WriteString(artifact.SHA256)
		body.WriteString("\n")
		if artifact.Truncated {
			body.WriteString("content truncated: true\n")
		}
		body.WriteString("```text\n")
		body.WriteString(artifact.Content)
		if !strings.HasSuffix(artifact.Content, "\n") {
			body.WriteString("\n")
		}
		body.WriteString("```\n")
	}

	return strings.TrimRight(body.String(), "\n")
}

func filterItemsByType(items []model.Item, itemType string) []model.Item {
	filtered := make([]model.Item, 0)
	for _, item := range items {
		if item.Type == itemType {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func titleForItemType(itemType string) string {
	switch itemType {
	case model.ItemTypeClaim:
		return "Claims"
	case model.ItemTypeRisk:
		return "Risks"
	case model.ItemTypeRecommendation:
		return "Recommendations"
	case model.ItemTypeQuestion:
		return "Questions"
	default:
		return "Items"
	}
}

func buildFocusItems(agentName string, items []model.Item, teamSize int) []model.Item {
	focusItems := make([]model.Item, 0)
	for _, item := range items {
		if item.Type == model.ItemTypeQuestion {
			focusItems = append(focusItems, item)
			continue
		}

		if !slices.Contains(item.SourceAgents, agentName) {
			focusItems = append(focusItems, item)
			continue
		}

		if teamSize > 0 && len(item.SourceAgents) < teamSize {
			focusItems = append(focusItems, item)
		}
	}

	return focusItems
}

func formatItemGroups(items []model.Item) string {
	var body strings.Builder
	for _, itemType := range itemTypeOrder {
		typedItems := filterItemsByType(items, itemType)
		if len(typedItems) == 0 {
			continue
		}

		body.WriteString("\n## ")
		body.WriteString(titleForItemType(itemType))
		body.WriteString("\n")
		for _, item := range typedItems {
			body.WriteString("- ")
			body.WriteString(item.Content)
			if len(item.SourceAgents) > 0 {
				body.WriteString(" [sources: ")
				body.WriteString(strings.Join(item.SourceAgents, ", "))
				body.WriteString("]")
			}
			body.WriteString("\n")
		}
	}

	return strings.TrimLeft(body.String(), "\n")
}

func findAgentOutput(outputs []model.AgentOutput, agentName string) *model.AgentOutput {
	for index := range outputs {
		if outputs[index].AgentName == agentName {
			return &outputs[index]
		}
	}

	return nil
}

func buildRoundSummary(round int, items []model.Item) model.RoundSummary {
	return model.RoundSummary{
		Round:     round + 1,
		ItemCount: len(items),
		ItemHash:  hashItems(items),
	}
}

func hashItems(items []model.Item) string {
	entries := make([]string, 0, len(items))
	for _, item := range items {
		entries = append(entries, item.Type+"|"+normalizeItemContent(item.Content))
	}

	slices.Sort(entries)
	hash := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(hash[:])
}

func stripListMarker(value string) string {
	trimmed := strings.TrimSpace(value)
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}

	if len(trimmed) >= 3 && unicode.IsDigit(rune(trimmed[0])) {
		separator := trimmed[1]
		if (separator == '.' || separator == ')') && trimmed[2] == ' ' {
			return strings.TrimSpace(trimmed[3:])
		}
	}

	return trimmed
}

func sanitizeCandidate(value string) string {
	trimmed := cleanWhitespace(value)
	trimmed = strings.Trim(trimmed, "\"'`")
	trimmed = strings.TrimSpace(trimmed)
	return trimmed
}

func cleanWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func shouldSkipCandidate(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" || len(lower) < 8 {
		return true
	}

	if strings.HasPrefix(lower, "mock response from") {
		return true
	}

	for _, prefix := range []string{
		"system instructions:",
		"user task:",
		"original task:",
		"attached local artifacts:",
		"agent responses:",
		"tokens used",
		"your previous answer:",
		"shared normalized items from the prior round:",
		"focus items for critique:",
		"instructions:",
		"content type:",
		"charset=",
		"size:",
		"sha256:",
		"content truncated:",
	} {
		if lower == prefix || strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	for _, marker := range []string{
		"critique/revise round ",
		"revise your answer using the normalized items above",
		"return one revised answer in the format required by the original task",
		"if the original task does not specify a format",
		"do not return a transcript of the protocol",
		"critique weak assumptions, gaps, and edge cases relevant to your role",
	} {
		if strings.HasPrefix(lower, marker) {
			return true
		}
	}

	return false
}

func hasRiskMarker(value string) bool {
	if hasAnyPrefix(value, "risk:", "risks:", "concern:", "concerns:", "failure mode:", "failure modes:", "regression:", "edge case:", "edge cases:", "hidden assumption:", "hidden assumptions:") {
		return true
	}

	for _, marker := range []string{"risk is", "risks are", "failure mode", "regression", "edge case", "hidden assumption", "hidden assumptions"} {
		if strings.Contains(value, marker) {
			return true
		}
	}

	return false
}

func hasRecommendationMarker(value string) bool {
	if hasAnyPrefix(value, "recommendation:", "recommendations:", "recommend:", "suggestion:", "suggestions:", "next step:", "next steps:") {
		return true
	}

	return hasAnyPrefix(value, "should ", "consider ", "recommend ", "avoid ", "prefer ", "add ", "implement ", "use ", "keep ", "need to ", "must ", "try ", "make ")
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	return false
}

func trimLabelPrefix(value string, prefixes ...string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			trimmed = strings.TrimSpace(trimmed[len(prefix):])
			break
		}
	}

	trimmed = strings.TrimSpace(trimmed)
	trimmed = stripSourceSuffix(trimmed)
	trimmed = strings.TrimSuffix(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if !strings.HasSuffix(trimmed, "?") {
		trimmed = strings.TrimRight(trimmed, ".!")
	}

	return cleanWhitespace(trimmed)
}

func stripSourceSuffix(value string) string {
	marker := strings.Index(strings.ToLower(value), " [sources:")
	if marker == -1 {
		return value
	}

	return strings.TrimSpace(value[:marker])
}

func normalizeItemContent(value string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(cleanWhitespace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			normalized.WriteRune(r)
		case unicode.IsSpace(r):
			normalized.WriteRune(' ')
		}
	}

	return cleanWhitespace(normalized.String())
}
