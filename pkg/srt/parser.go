package srt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"stream-web-api/internal/domain/model"
)

// ParseTimestamp converts SRT/VTT/ASS timestamp to seconds
func ParseTimestamp(ts string) float64 {
	// Format could be:
	// SRT/VTT: 00:00:20,000 or 00:00:20.000
	// ASS: 0:00:20.00 (single digit hour)
	
	ts = strings.Replace(ts, ",", ".", 1)
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0
	}
	
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	s, _ := strconv.ParseFloat(parts[2], 64)
	
	return h*3600 + m*60 + s
}

// isNumeric checks if a string is purely numeric
func isNumeric(s string) bool {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return err == nil
}

// Parse converts SRT, VTT, or ASS content to SubtitleCues
func Parse(srt []byte) []model.SubtitleCue {
	content := string(srt)
	
	// Detect ASS format
	if strings.Contains(content, "[Script Info]") || strings.Contains(content, "ScriptType: v4.00") {
		return parseASS(content)
	}

	return parseSRT(content)
}

func parseSRT(content string) []model.SubtitleCue {
	lines := strings.Split(content, "\n")
	var cues []model.SubtitleCue

	reTiming := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}[,.]\d{3}) --> (\d{2}:\d{2}:\d{2}[,.]\d{3})`)
	reHtmlTags := regexp.MustCompile(`<[^>]*>`)
	reAssTags := regexp.MustCompile(`\{[^}]*\}`)
	reAlignTag := regexp.MustCompile(`\\an(\d)`)

	var currentCue *model.SubtitleCue

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if currentCue != nil {
				cues = append(cues, *currentCue)
				currentCue = nil
			}
			continue
		}

		if isNumeric(line) && len(line) < 5 && currentCue == nil {
			continue
		}

		if reTiming.MatchString(line) {
			if currentCue != nil {
				cues = append(cues, *currentCue)
			}

			matches := reTiming.FindStringSubmatch(line)
			start := ParseTimestamp(matches[1])
			end := ParseTimestamp(matches[2])

			currentCue = &model.SubtitleCue{
				Start: start,
				End:   end,
				Text:  "",
			}
		} else {
			if currentCue != nil {
				if currentCue.Position == "" {
					alignMatch := reAlignTag.FindStringSubmatch(line)
					if len(alignMatch) == 2 {
						n, _ := strconv.Atoi(alignMatch[1])
						switch {
						case n >= 1 && n <= 3:
							currentCue.Position = "bottom"
						case n >= 4 && n <= 6:
							currentCue.Position = "middle"
						case n >= 7 && n <= 9:
							currentCue.Position = "top"
						}
					}
				}
				cleanLine := reAssTags.ReplaceAllString(line, "")
				cleanLine = reHtmlTags.ReplaceAllString(cleanLine, "")
				if currentCue.Text != "" {
					currentCue.Text += "\n" + cleanLine
				} else {
					currentCue.Text = cleanLine
				}
			}
		}
	}
	if currentCue != nil {
		cues = append(cues, *currentCue)
	}

	return cues
}

func parseASS(content string) []model.SubtitleCue {
	var cues []model.SubtitleCue
	lines := strings.Split(content, "\n")
	
	// Example ASS line:
	// Dialogue: 0,0:00:10.50,0:00:13.20,Default,,0,0,0,,Text goes here
	
	// Regex to match ASS dialogue lines and extract start, end, and text
	reDialogue := regexp.MustCompile(`^Dialogue:\s*[^,]+,\s*([^,]+)\s*,\s*([^,]+)\s*,.*?(?:,[^,]*){5},(.*)$`)
	reOverrideTags := regexp.MustCompile(`\{[^}]*\}`)
	reAlignTag := regexp.MustCompile(`\\an(\d)`)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Dialogue:") {
			continue
		}
		
		matches := reDialogue.FindStringSubmatch(line)
		if len(matches) == 4 {
			start := ParseTimestamp(matches[1])
			end := ParseTimestamp(matches[2])
			
			text := matches[3]

			position := ""
			alignMatch := reAlignTag.FindStringSubmatch(text)
			if len(alignMatch) == 2 {
				n, _ := strconv.Atoi(alignMatch[1])
				switch {
				case n >= 1 && n <= 3:
					position = "bottom"
				case n >= 4 && n <= 6:
					position = "middle"
				case n >= 7 && n <= 9:
					position = "top"
				}
			}

			text = reOverrideTags.ReplaceAllString(text, "")
			text = strings.ReplaceAll(text, "\\N", "\n")
			text = strings.ReplaceAll(text, "\\n", "\n")
			
			if strings.TrimSpace(text) != "" {
				cues = append(cues, model.SubtitleCue{
					Start:    start,
					End:      end,
					Text:     text,
					Position: position,
				})
			}
		}
	}
	
	return cues
}
