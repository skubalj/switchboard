package config

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_trimIter(t *testing.T) {
	file := "line1\n" +
		" line2\n \n" +
		"line3\r\n" +
		" line4\r  \r\n"

	expected := []string{
		"line1",
		"line2",
		"line3",
		"line4",
	}

	lines := slices.Collect(trimIter(strings.Lines(file)))
	require.Equal(t, expected, lines)
}
