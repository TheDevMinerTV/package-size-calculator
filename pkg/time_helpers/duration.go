package time_helpers

import (
	"fmt"
	"strings"
	"time"
)

const (
	Day   = time.Hour * 24
	Week  = Day * 7
	Month = Week * 4
)

func FormatDuration(d time.Duration) string {
	s := strings.Builder{}

	var (
		disableHours   = false
		disableMinutes = false
		disableSeconds = false
	)

	weeks := d / Week
	d -= weeks * Week

	days := d / Day
	d -= days * Day

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	if weeks > 0 {
		s.WriteString(fmt.Sprintf("%d", weeks))
		s.WriteString("w")
		disableHours = true
		disableMinutes = true
		disableSeconds = true
	}

	if days > 0 {
		s.WriteString(fmt.Sprintf("%d", days))
		s.WriteString("d")
		disableHours = true
		disableMinutes = true
		disableSeconds = true
	}

	if hours > 0 && !disableHours {
		s.WriteString(fmt.Sprintf("%d", hours))
		s.WriteString("h")
		disableSeconds = true
	}

	if minutes > 0 && !disableMinutes {
		s.WriteString(fmt.Sprintf("%d", minutes))
		s.WriteString("m")
	}

	if seconds > 0 && !disableSeconds {
		s.WriteString(fmt.Sprintf("%d", seconds))
		s.WriteString("s")
	}

	return s.String()
}
