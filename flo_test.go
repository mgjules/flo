package flo_test

import (
	"context"
	"testing"

	"github.com/mgjules/flo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	val int
}

func (t testStruct) AddVal(ctx context.Context, n int) int {
	return t.val + n
}

type testFloFn func(context.Context, int) (int, error)

func TestFlo(t *testing.T) {
	f, err := flo.NewFlo(lo.Empty[testFloFn]())
	require.NoError(t, err)
	require.NotNil(t, f)

	ts := testStruct{val: 10}
	root, err := flo.NewComponent(
		"Test Label",
		"Test Description",
		ts.AddVal,
	)
	require.NoError(t, err)
	require.NotNil(t, root)

	f.AddComponent(root)
}
