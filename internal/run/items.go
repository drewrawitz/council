package run

import (
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

func buildSynthesisPrompt(prompt string, outputs []model.AgentOutput, items []model.Item) string {
	var body strings.Builder
	body.WriteString("You are synthesizing multiple agent responses into one final answer.\n\n")
	body.WriteString("Original task:\n")
	body.WriteString(strings.TrimSpace(prompt))
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

	body.WriteString("\nProduce one concise final answer in Markdown. Resolve disagreements where possible and note remaining uncertainty briefly when it matters.\n")

	return body.String()
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

	for _, prefix := range []string{"system instructions:", "user task:", "original task:", "agent responses:", "tokens used"} {
		if lower == prefix {
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
	trimmed = strings.TrimSuffix(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if !strings.HasSuffix(trimmed, "?") {
		trimmed = strings.TrimRight(trimmed, ".!")
	}

	return cleanWhitespace(trimmed)
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
