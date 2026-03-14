package srt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"torrent-stream/internal/domain"
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
func Parse(srt []byte) []domain.SubtitleCue {
	content := string(srt)
	
	// Detect ASS format
	if strings.Contains(content, "[Script Info]") || strings.Contains(content, "ScriptType: v4.00") {
		return parseASS(content)
	}

	return parseSRT(content)
}

func parseSRT(content string) []domain.SubtitleCue {
	lines := strings.Split(content, "\n")
	var cues []domain.SubtitleCue

	// Regex for SRT timing
	reTiming := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}[,.]\d{3}) --> (\d{2}:\d{2}:\d{2}[,.]\d{3})`)
	reTags := regexp.MustCompile(`<[^>]*>`)

	var currentCue *domain.SubtitleCue

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if currentCue != nil {
				cues = append(cues, *currentCue)
				currentCue = nil
			}
			continue
		}

		// Skip index numbers if strictly numeric and short
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

			currentCue = &domain.SubtitleCue{
				Start: start,
				End:   end,
				Text:  "",
			}
		} else {
			if currentCue != nil {
				// Strip tags
				cleanLine := reTags.ReplaceAllString(line, "")
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

func parseASS(content string) []domain.SubtitleCue {
	var cues []domain.SubtitleCue
	lines := strings.Split(content, "\n")
	
	// Example ASS line:
	// Dialogue: 0,0:00:10.50,0:00:13.20,Default,,0,0,0,,Text goes here
	
	// Regex to match ASS dialogue lines and extract start, end, and text
	reDialogue := regexp.MustCompile(`^Dialogue:\s*[^,]+,\s*([^,]+)\s*,\s*([^,]+)\s*,.*?(?:,[^,]*){5},(.*)$`)
	// Regex to strip ASS override tags like {\an8} or {\c&H0000FF&}
	reOverrideTags := regexp.MustCompile(`\{[^}]*\}`)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Dialogue:") {
			continue
		}
		
		matches := reDialogue.FindStringSubmatch(line)
		if len(matches) == 4 {
			start := ParseTimestamp(matches[1])
			end := ParseTimestamp(matches[2])
			
			// Clean the text
			text := matches[3]
			text = reOverrideTags.ReplaceAllString(text, "")
			text = strings.ReplaceAll(text, "\\N", "\n")
			text = strings.ReplaceAll(text, "\\n", "\n")
			
			if strings.TrimSpace(text) != "" {
				cues = append(cues, domain.SubtitleCue{
					Start: start,
					End:   end,
					Text:  text,
				})
			}
		}
	}
	
	return cues
}
