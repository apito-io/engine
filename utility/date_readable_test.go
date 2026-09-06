package utility

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatReadableDate(t *testing.T) {
	require.Equal(t, "6 October 2026", FormatReadableDate("2026-10-06T12:25:43Z", ""))
	require.Equal(t, "13 September 2026", FormatReadableDate("2026-09-13T12:25:43Z", ""))
	require.Equal(t, "1 January 2026", FormatReadableDate("2026-01-01", ""))
	require.Equal(t, "on Day 30", FormatReadableDate("", "on Day 30"))
	require.Equal(t, "not-a-date", FormatReadableDate("not-a-date", "fallback"))
}
