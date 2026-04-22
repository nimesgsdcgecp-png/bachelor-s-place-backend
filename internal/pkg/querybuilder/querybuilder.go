// Package querybuilder provides a lightweight helper for constructing
// parameterized SQL queries with dynamic WHERE clauses for use with pgx/v5.
//
// This is NOT an ORM. It is a small utility that manages the $1, $2...
// argument counter automatically so callers don't risk off-by-one errors
// when building queries with multiple optional filters.
//
// Use this ONLY for endpoints with many optional filters (e.g., map search).
// For simple queries, write raw SQL strings directly.
//
// Usage:
//
//	qb := querybuilder.New("SELECT id, title, rent_amount FROM properties")
//	qb.Where("deleted_at IS NULL")
//	qb.Where("status = 'verified'")
//	qb.WhereParam("ST_DWithin(location, ST_MakePoint($?, $?)::geography, $?)", lng, lat, radius)
//	if filter.PriceMin > 0 {
//	    qb.WhereParam("rent_amount >= $?", filter.PriceMin)
//	}
//	qb.OrderBy("distance_metres ASC")
//	qb.Limit(20).Offset(0)
//	sql, args := qb.Build()
//	rows, err := pool.Query(ctx, sql, args...)
package querybuilder

import (
	"fmt"
	"strings"
)

// Builder constructs a parameterized pgx SQL query with dynamic conditions.
type Builder struct {
	base       string
	conditions []string
	args       []interface{}
	argIndex   int
	orderBy    string
	limit      *int
	offset     *int
}

// New creates a Builder with the given SELECT ... FROM ... base string.
// Do NOT include a WHERE clause in the base.
func New(base string) *Builder {
	return &Builder{base: base, argIndex: 1}
}

// Where appends a condition with no bound parameters (e.g., "deleted_at IS NULL").
func (b *Builder) Where(condition string) *Builder {
	b.conditions = append(b.conditions, condition)
	return b
}

// WhereParam appends a condition that contains one or more $? placeholders.
// Each $? is replaced in order with the next available $N index.
// Pass the corresponding argument values as variadic args.
//
// Example:
//
//	qb.WhereParam("ST_DWithin(location, ST_MakePoint($?, $?)::geography, $?)", lng, lat, radius)
func (b *Builder) WhereParam(condition string, args ...interface{}) *Builder {
	for _, arg := range args {
		placeholder := fmt.Sprintf("$%d", b.argIndex)
		condition = strings.Replace(condition, "$?", placeholder, 1)
		b.args = append(b.args, arg)
		b.argIndex++
	}
	b.conditions = append(b.conditions, condition)
	return b
}

// OrderBy sets the ORDER BY clause. Subsequent calls overwrite the previous value.
func (b *Builder) OrderBy(clause string) *Builder {
	b.orderBy = clause
	return b
}

// Limit sets the LIMIT value as a bound parameter.
func (b *Builder) Limit(n int) *Builder {
	b.limit = &n
	return b
}

// Offset sets the OFFSET value as a bound parameter.
func (b *Builder) Offset(n int) *Builder {
	b.offset = &n
	return b
}

// Build assembles the final SQL string and the ordered argument slice.
// The returned sql and args are ready for pgxpool.Pool.Query(ctx, sql, args...).
func (b *Builder) Build() (string, []interface{}) {
	q := b.base

	if len(b.conditions) > 0 {
		q += " WHERE " + strings.Join(b.conditions, " AND ")
	}
	if b.orderBy != "" {
		q += " ORDER BY " + b.orderBy
	}
	if b.limit != nil {
		q += fmt.Sprintf(" LIMIT $%d", b.argIndex)
		b.args = append(b.args, *b.limit)
		b.argIndex++
	}
	if b.offset != nil {
		q += fmt.Sprintf(" OFFSET $%d", b.argIndex)
		b.args = append(b.args, *b.offset)
		b.argIndex++
	}

	return q, b.args
}
