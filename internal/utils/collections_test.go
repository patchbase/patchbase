package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
	t.Run("maps ints to ints", func(t *testing.T) {
		in := []int{1, 2, 3}

		out := Map(in, func(v int) int {
			return v * 2
		})

		require.Len(t, out, 3)
		assert.Equal(t, []int{2, 4, 6}, out)
	})

	t.Run("maps strings to lengths", func(t *testing.T) {
		in := []string{"a", "bb", "ccc"}

		out := Map(in, func(s string) int {
			return len(s)
		})

		assert.Equal(t, []int{1, 2, 3}, out)
	})

	t.Run("empty input", func(t *testing.T) {
		out := Map([]int{}, func(v int) int { return v + 1 })
		assert.Empty(t, out)
	})
}

func TestMapErr(t *testing.T) {
	t.Run("maps ints to strings", func(t *testing.T) {
		in := []int{1, 2, 3}

		out, err := MapErr(in, func(v int) (string, error) {
			return strconv.Itoa(v), nil
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"1", "2", "3"}, out)
	})

	t.Run("returns error immediately", func(t *testing.T) {
		in := []int{1, 2, 3}

		out, err := MapErr(in, func(v int) (string, error) {
			if v == 2 {
				return "", assert.AnError
			}
			return strconv.Itoa(v), nil
		})

		assert.ErrorIs(t, err, assert.AnError)
		assert.Nil(t, out)
	})

	t.Run("empty input", func(t *testing.T) {
		out, err := MapErr([]int{}, func(v int) (string, error) {
			return strconv.Itoa(v), nil
		})

		require.NoError(t, err)
		assert.Empty(t, out)
	})
}
