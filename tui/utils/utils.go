package utils

func ReorderDown[T any](rows []T, cursor int) bool {
	if cursor < 0 || len(rows)-1 <= cursor {
		return false
	}

	rows[cursor], rows[cursor+1] = rows[cursor+1], rows[cursor]
	return true
}

func ReorderUp[T any](rows []T, cursor int) bool {
	return ReorderDown(rows, cursor-1)
}
