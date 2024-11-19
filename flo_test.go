package flo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mgjules/flo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type floFn func(ctx context.Context, in int) (int, error)

type compA struct {
	val int
}

func (t compA) AddVal(ctx context.Context, f1 int) int {
	return t.val + f1
}

func compBFn(f1 int) (int, error) {
	if f1 < 0 {
		return 0, errors.New("f1 is less than zero")
	}

	return f1 + 1, nil
}

func compCFn(ctx context.Context, a1 int, b1 int) (int, error) {
	if a1 < 0 || b1 < 0 {
		return 0, errors.New("a1 or b1 is less than zero")
	}

	return a1 + b1, nil
}

func TestFlo(t *testing.T) {
	f, err := flo.NewFlo(lo.Empty[floFn]())
	require.NoError(t, err)
	require.NotNil(t, f)

	compA, err := flo.NewComponent(
		"Test Comp A Label",
		"Test Comp A Description",
		(compA{val: 10}).AddVal,
	)
	require.NoError(t, err)
	require.NotNil(t, compA)
	f.AddComponent(compA)

	compB, err := flo.NewComponent(
		"Test Comp B Label",
		"Test Comp B Description",
		compBFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compB)
	f.AddComponent(compB)

	compC, err := flo.NewComponent(
		"Test Comp C Label",
		"Test Comp C Description",
		compCFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compC)
	f.AddComponent(compC)

	t.Run("Connect flos & components", func(t *testing.T) {
		t.Run("Cannot connect to self", func(t *testing.T) {
			err = f.ConnectComponent(compA.ID, compA.IOs[2].ID, compA.ID, compA.IOs[1].ID)
			require.ErrorContains(t, err, "cannot connect to itself")

			err = f.ConnectComponent(f.ID, f.IOs[2].ID, f.ID, f.IOs[1].ID)
			require.ErrorContains(t, err, "cannot connect to itself")
		})

		t.Run("Cannot connect wrong io types", func(t *testing.T) {
			err = f.ConnectComponent(f.ID, f.IOs[0].ID, compA.ID, compA.IOs[1].ID)
			require.ErrorContains(t, err, "cannot be assigned to")
		})

		t.Run("Cannot connect flo outgoing io as type out instead of in", func(t *testing.T) {
			err = f.ConnectComponent(f.ID, f.IOs[3].ID, compA.ID, compA.IOs[1].ID)
			require.ErrorContains(t, err, "is not of type in")
		})

		t.Run("Cannot connect component outgoing io as type in to component out", func(t *testing.T) {
			err = f.ConnectComponent(compB.ID, compB.IOs[0].ID, compA.ID, compA.IOs[2].ID)
			require.ErrorContains(t, err, "is not of type out")
		})

		t.Run("Successful connections", func(t *testing.T) {
			err = f.ConnectComponent(f.ID, f.IOs[0].ID, compC.ID, compC.IOs[0].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(f.ID, f.IOs[0].ID, compA.ID, compA.IOs[0].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(f.ID, f.IOs[1].ID, compA.ID, compA.IOs[1].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(f.ID, f.IOs[1].ID, compB.ID, compB.IOs[0].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(compA.ID, compA.IOs[2].ID, compC.ID, compC.IOs[1].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(compB.ID, compB.IOs[1].ID, compC.ID, compC.IOs[2].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(compC.ID, compC.IOs[3].ID, f.ID, f.IOs[2].ID)
			require.NoError(t, err)
		})

		t.Run("Cannot connect to an already connected component ingoing io", func(t *testing.T) {
			err = f.ConnectComponent(f.ID, f.IOs[1].ID, compC.ID, compC.IOs[2].ID)
			require.ErrorContains(t, err, "already has a connection")
		})
	})
}