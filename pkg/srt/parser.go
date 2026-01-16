package srt

import (
	"fmt"
	"regexp"
	"strings"

	"torrent-stream/internal/domain"
)

// ParseTimestamp converts SRT/VTT timestamp to seconds
func ParseTimestamp(ts string) float64 {
	// Format: 00:00:20,000 or 00:00:20.000
	ts = strings.Replace(ts, ",", ".", 1)
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0
	}
	h := 0.0
	m := 0.0
	s := 0.0
	fmt.Sscanf(parts[0], "%f", &h)
	fmt.Sscanf(parts[1], "%f", &m)
	fmt.Sscanf(parts[2], "%f", &s)
	return h*3600 + m*60 + s
}

// isNumeric checks if a string is purely numeric
func isNumeric(s string) bool {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return err == nil
}

// Parse converts SRT content to SubtitleCues
func Parse(srt []byte) []domain.SubtitleCue {
	lines := strings.Split(string(srt), "\n")
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
