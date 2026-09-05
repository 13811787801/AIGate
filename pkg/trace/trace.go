package trace

import "github.com/google/uuid"

// Ensure 若传入空则生成新的 trace id。
func Ensure(existing string) string {
	if existing != "" {
		return existing
	}
	return uuid.NewString()
}
