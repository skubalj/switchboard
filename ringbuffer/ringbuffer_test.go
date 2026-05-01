package ringbuffer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInsert(t *testing.T) {
	buf := New[int](5)

	buf = buf.Append(1)
	require.EqualValues(t, []int{1}, buf.AsSlice())
	buf = buf.Append(2)
	require.EqualValues(t, []int{1, 2}, buf.AsSlice())
	buf = buf.Append(3)
	require.EqualValues(t, []int{1, 2, 3}, buf.AsSlice())
	buf = buf.Append(4)
	require.EqualValues(t, []int{1, 2, 3, 4}, buf.AsSlice())
	buf = buf.Append(5)
	require.EqualValues(t, []int{1, 2, 3, 4, 5}, buf.AsSlice())
	buf = buf.Append(6)
	require.EqualValues(t, []int{2, 3, 4, 5, 6}, buf.AsSlice())
	buf = buf.Append(7)
	require.EqualValues(t, []int{3, 4, 5, 6, 7}, buf.AsSlice())
	buf = buf.Append(8)
	require.EqualValues(t, []int{4, 5, 6, 7, 8}, buf.AsSlice())
	buf = buf.Append(9)
	require.EqualValues(t, []int{5, 6, 7, 8, 9}, buf.AsSlice())
	buf = buf.Append(10)
	require.EqualValues(t, []int{6, 7, 8, 9, 10}, buf.AsSlice())
	buf = buf.Append(11)
	require.EqualValues(t, []int{7, 8, 9, 10, 11}, buf.AsSlice())
	buf = buf.Append(12)
	require.EqualValues(t, []int{8, 9, 10, 11, 12}, buf.AsSlice())
	buf = buf.Append(13)
	require.EqualValues(t, []int{9, 10, 11, 12, 13}, buf.AsSlice())
	buf = buf.Append(14)
	require.EqualValues(t, []int{10, 11, 12, 13, 14}, buf.AsSlice())
	buf = buf.Append(15)
	require.EqualValues(t, []int{11, 12, 13, 14, 15}, buf.AsSlice())
}
