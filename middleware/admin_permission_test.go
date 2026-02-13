package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchPath(t *testing.T) {
	tests := []struct {
		actual   string
		pattern  string
		expected bool
	}{
		{"/admin/expenses", "/admin/expenses", true},
		{"/admin/expenses/123", "/admin/expenses", false},
		{"/admin/users/123", "/admin/users/:id", true},
		{"/admin/users/1", "/admin/users/:id", true},
		{"/admin/users/abc", "/admin/users/:id", true},
		{"/admin/users/", "/admin/users/:id", false},
		{"/admin/users", "/admin/users/:id", false},
		{"/admin/categories/5", "/admin/categories/:id", true},
		{"/admin/ai-models/10", "/admin/ai-models/:id", true},
		{"/admin/wrong", "/admin/expenses", false},
		{"/admin/expenses/detailed-statistics", "/admin/expenses/detailed-statistics", true},
		{"/admin/expenses/detailed-statistics", "/admin/expenses/:id", true},
	}
	for _, tt := range tests {
		got := matchPath(tt.actual, tt.pattern)
		assert.Equalf(t, tt.expected, got, "matchPath(%q, %q)", tt.actual, tt.pattern)
	}
}
