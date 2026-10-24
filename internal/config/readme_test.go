package config

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReadme_yamlBlocks keeps the README's examples valid: each one must be a complete config the parser accepts.
func TestReadme_yamlBlocks(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	require.NoError(t, err)
	blocks := regexp.MustCompile("(?s)```yaml\n(.*?)```").FindAllStringSubmatch(string(raw), -1)
	require.NotEmpty(t, blocks)

	for i, b := range blocks {
		t.Run(fmt.Sprintf("block %d", i), func(t *testing.T) {
			_, err := Parse([]byte(b[1]))

			require.NoError(t, err)
		})
	}
}
