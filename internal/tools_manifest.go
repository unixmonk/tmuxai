package internal

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const toolBulletPrefix = "- `"
const sectionHeaderPrefix = "## "

// ToolChange represents the outcome of a manifest update operation.
type ToolChange struct {
	Section     string
	Name        string
	Description string
	Action      string
}

// AddToolToManifest adds or updates a tool entry under the specified section.
// Returns true if the manifest was modified.
func AddToolToManifest(path, section, name, description string) (ToolChange, bool, error) {
	section = strings.TrimSpace(section)
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	if section == "" || name == "" {
		return ToolChange{}, false, errors.New("section and name are required")
	}

	content, err := readManifest(path)
	if err != nil {
		return ToolChange{}, false, err
	}

	lines := splitLines(content)
	header := sectionHeaderPrefix + section
	toolLine := formatToolLine(name, description)

	sectionStart, sectionEnd := findSectionBounds(lines, header)
	if sectionStart == -1 {
		lines = appendSection(lines, header)
		sectionStart, sectionEnd = findSectionBounds(lines, header)
	}

	for i := sectionStart + 1; i < sectionEnd; i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, toolBulletPrefix) {
			continue
		}
		currentName := extractToolName(line)
		if strings.EqualFold(currentName, name) {
			if line == toolLine {
				return ToolChange{Section: section, Name: name, Description: description, Action: "unchanged"}, false, nil
			}
			lines[i] = replaceWithIndent(lines[i], toolLine)
			return ToolChange{Section: section, Name: name, Description: description, Action: "updated"}, true, writeManifest(path, lines)
		}
	}

	insertIndex := sectionEnd
	if insertIndex > sectionStart+1 && strings.TrimSpace(lines[insertIndex-1]) != "" {
		lines = insertLine(lines, insertIndex, "")
		insertIndex++
	}
	lines = insertLine(lines, insertIndex, toolLine)
	return ToolChange{Section: section, Name: name, Description: description, Action: "added"}, true, writeManifest(path, lines)
}

// RemoveToolFromManifest removes a tool entry if present. Returns true when removed.
func RemoveToolFromManifest(path, section, name string) (ToolChange, bool, error) {
	section = strings.TrimSpace(section)
	name = strings.TrimSpace(name)
	if section == "" || name == "" {
		return ToolChange{}, false, errors.New("section and name are required")
	}

	content, err := readManifest(path)
	if err != nil {
		return ToolChange{}, false, err
	}

	lines := splitLines(content)
	header := sectionHeaderPrefix + section
	sectionStart, sectionEnd := findSectionBounds(lines, header)
	if sectionStart == -1 {
		return ToolChange{}, false, nil
	}

	for i := sectionStart + 1; i < sectionEnd; i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, toolBulletPrefix) {
			continue
		}
		currentName := extractToolName(line)
		if strings.EqualFold(currentName, name) {
			lines = append(lines[:i], lines[i+1:]...)
			lines = collapseExtraBlankLines(lines, sectionStart, sectionEnd-1)
			return ToolChange{Section: section, Name: name, Action: "removed"}, true, writeManifest(path, lines)
		}
	}

	return ToolChange{}, false, nil
}

// ListToolsInManifest returns the raw manifest contents.
func ListToolsInManifest(path string) (string, error) {
	content, err := readManifest(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(content, "\n") + "\n", nil
}

func readManifest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("tools manifest not found at %s", path)
	}
	return "", err
}

func writeManifest(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func splitLines(content string) []string {
	if content == "" {
		return []string{""}
	}
	return strings.Split(content, "\n")
}

func findSectionBounds(lines []string, header string) (int, int) {
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i
			for j := i + 1; j < len(lines); j++ {
				trim := strings.TrimSpace(lines[j])
				if strings.HasPrefix(trim, sectionHeaderPrefix) {
					end = j
					return start, end
				}
			}
			return start, end
		}
	}
	return -1, len(lines)
}

func appendSection(lines []string, header string) []string {
	trimmed := strings.TrimSpace(strings.Join(lines, "\n"))
	if trimmed != "" {
		lines = append(lines, "")
	}
	return append(lines, header, "")
}

func insertLine(lines []string, index int, value string) []string {
	if index < 0 {
		index = 0
	}
	if index >= len(lines) {
		return append(lines, value)
	}
	lines = append(lines[:index+1], lines[index:]...)
	lines[index] = value
	return lines
}

func replaceWithIndent(original, replacement string) string {
	leading := len(original) - len(strings.TrimLeft(original, " \t"))
	if leading <= 0 {
		return replacement
	}
	return strings.Repeat(" ", leading) + replacement
}

func extractToolName(line string) string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, toolBulletPrefix) {
		return ""
	}
	t = strings.TrimPrefix(t, toolBulletPrefix)
	idx := strings.Index(t, "`")
	if idx == -1 {
		return strings.TrimSpace(t)
	}
	return strings.TrimSpace(t[:idx])
}

func collapseExtraBlankLines(lines []string, sectionStart, sectionEnd int) []string {
	// sectionEnd is exclusive before removal; after removal it may shift.
	if sectionStart < 0 {
		return lines
	}
	// Clean repeated blank lines within section.
	limit := len(lines)
	prevBlank := false
	for i := sectionStart + 1; i < limit; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			if prevBlank {
				lines = append(lines[:i], lines[i+1:]...)
				limit--
				i--
				continue
			}
			prevBlank = true
		} else {
			prevBlank = false
			if strings.HasPrefix(strings.TrimSpace(lines[i]), sectionHeaderPrefix) {
				break
			}
		}
	}
	return lines
}

func formatToolLine(name, description string) string {
	if description == "" {
		description = "(no description)"
	}
	return fmt.Sprintf("- `%s` – %s", name, description)
}
