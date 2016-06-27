// Package api provides general types and functions for leanmanager
package api

import (
	"strings"
	"time"
)

// ConvertTime transforms a string typed by the user to the time.Time type
func ConvertTime(h string) (time.Time, error) {

	switch {
	case strings.Contains(h, "AM"):
		return time.Parse("15:04AM", h)
	case strings.Contains(h, "PM"):
		return time.Parse("15:04PM", h)
	case strings.Contains(h, "am"):
		return time.Parse("15:04am", h)
	case strings.Contains(h, "pm"):
		return time.Parse("15:04pm", h)
	default:
		return time.Parse("15:04", h)
	}
}
