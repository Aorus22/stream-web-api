package ranger

import (
	"strconv"
	"strings"
)

func ParseByteRange(rangeHeader string) (start int64, end int64, ok bool) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, -1, false
	}
	parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(parts) != 2 {
		return 0, -1, false
	}
	s, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || s < 0 {
		return 0, -1, false
	}
	e := int64(-1)
	if parts[1] != "" {
		if parsed, err := strconv.ParseInt(parts[1], 10, 64); err == nil && parsed >= s {
			e = parsed
		}
	}
	return s, e, true
}

func ParseContentRange(cr string) (start int64, end int64, total int64, ok bool) {
	if !strings.HasPrefix(cr, "bytes ") {
		return 0, 0, 0, false
	}
	cr = strings.TrimPrefix(cr, "bytes ")
	parts := strings.Split(cr, "/")
	if len(parts) != 2 {
		return 0, 0, 0, false
	}
	rangePart := parts[0]
	totalPart := parts[1]

	re := strings.Split(rangePart, "-")
	if len(re) != 2 {
		return 0, 0, 0, false
	}
	s, err := strconv.ParseInt(strings.TrimSpace(re[0]), 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	e, err := strconv.ParseInt(strings.TrimSpace(re[1]), 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}

	t := int64(0)
	if totalPart != "*" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(totalPart), 10, 64); err == nil {
			t = parsed
		}
	}
	return s, e, t, true
}
