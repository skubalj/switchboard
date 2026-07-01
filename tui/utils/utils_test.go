package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReorderDown(t *testing.T) {
	arr := []int{1, 2, 3, 4, 5}
	ok := ReorderDown(arr, -1)
	require.False(t, ok)
	require.Equal(t, []int{1, 2, 3, 4, 5}, arr)

	ok = ReorderDown(arr, 0)
	require.True(t, ok)
	require.Equal(t, []int{2, 1, 3, 4, 5}, arr)

	ok = ReorderDown(arr, 1)
	require.True(t, ok)
	require.Equal(t, []int{2, 3, 1, 4, 5}, arr)

	ok = ReorderDown(arr, 3)
	require.True(t, ok)
	require.Equal(t, []int{2, 3, 1, 5, 4}, arr)

	ok = ReorderDown(arr, 4)
	require.False(t, ok)
	require.Equal(t, []int{2, 3, 1, 5, 4}, arr)

	ok = ReorderDown(arr, 5)
	require.False(t, ok)
	require.Equal(t, []int{2, 3, 1, 5, 4}, arr)

	arr2 := []int{1}
	ok = ReorderDown(arr2, 0)
	require.False(t, ok)

	var arr3 []int
	ok = ReorderDown(arr3, 0)
	require.False(t, ok)
}
