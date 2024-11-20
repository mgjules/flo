package flo_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mgjules/flo"
	"github.com/stretchr/testify/require"
)

type compA struct {
	val int
}

func (t compA) AddVal(ctx context.Context, f1 int) int {
	return t.val + f1
}

func compBFn(f1 int, d1 bool) (int, error) {
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

func compDFn() bool {
	return true
}

func compEFn() {
	// So lonely.
}

func TestFlo(t *testing.T) {
	f, err := flo.NewFlo(
		"TestSync",
		"Test Flo Label",
		"Test Flo Description",
	)
	require.NoError(t, err)
	require.NotNil(t, f)

	pCtx, err := flo.NewComponentIO(
		"ctx",
		flo.ComponentIOTypeIN,
		reflect.TypeFor[context.Context](),
		f.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, pCtx)
	require.NoError(t, f.AddIO(pCtx))

	pIn, err := flo.NewComponentIO(
		"in",
		flo.ComponentIOTypeIN,
		reflect.TypeFor[int](),
		f.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, pIn)
	require.NoError(t, f.AddIO(pIn))

	rInt, err := flo.NewComponentIO(
		"",
		flo.ComponentIOTypeOUT,
		reflect.TypeFor[int](),
		f.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, rInt)
	require.NoError(t, f.AddIO(rInt))

	rErr, err := flo.NewComponentIO(
		"",
		flo.ComponentIOTypeOUT,
		reflect.TypeFor[error](),
		f.ID,
	)
	require.NoError(t, err)
	require.NotNil(t, rErr)
	require.NoError(t, f.AddIO(rErr))

	compA, err := flo.NewComponent(
		"CompA",
		"Test Comp A Label",
		"Test Comp A Description",
		(compA{val: 10}).AddVal,
	)
	require.NoError(t, err)
	require.NotNil(t, compA)
	require.NoError(t, f.AddComponent(compA))

	compB, err := flo.NewComponent(
		"CompB",
		"Test Comp B Label",
		"Test Comp B Description",
		compBFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compB)
	require.NoError(t, f.AddComponent(compB))

	compC, err := flo.NewComponent(
		"CompC",
		"Test Comp C Label",
		"Test Comp C Description",
		compCFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compC)
	require.NoError(t, f.AddComponent(compC))

	compD, err := flo.NewComponent(
		"CompD",
		"Test Comp D Label",
		"Test Comp D Description",
		compDFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compD)
	require.NoError(t, f.AddComponent(compD))

	compE, err := flo.NewComponent(
		"CompE",
		"Test Comp E Label",
		"Test Comp E Description",
		compEFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compE)
	require.NoError(t, f.AddComponent(compE))

	t.Run("Cannot add component twice", func(t *testing.T) {
		err = f.AddComponent(compC)
		require.ErrorContains(t, err, "already exists")
	})

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

			err = f.ConnectComponent(compD.ID, compD.IOs[0].ID, compB.ID, compB.IOs[1].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(compA.ID, compA.IOs[2].ID, compC.ID, compC.IOs[1].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(compB.ID, compB.IOs[2].ID, compC.ID, compC.IOs[2].ID)
			require.NoError(t, err)

			err = f.ConnectComponent(compC.ID, compC.IOs[3].ID, f.ID, f.IOs[2].ID)
			require.NoError(t, err)
		})

		t.Run("Cannot connect to an already connected component ingoing io", func(t *testing.T) {
			err = f.ConnectComponent(f.ID, f.IOs[1].ID, compC.ID, compC.IOs[2].ID)
			require.ErrorContains(t, err, "already has a connection")
		})
	})

	t.Run("Cannot delete component with connections", func(t *testing.T) {
		err = f.DeleteComponent(compA.ID)
		require.ErrorContains(t, err, "has connections")
	})

	t.Run("Render", func(t *testing.T) {
		buf := &bytes.Buffer{}
		err = f.Render(context.Background(), buf)
		require.NoError(t, err)
		require.Equal(t, "[Test Comp A Label][Test Comp D Label][Test Comp B Label][Test Comp C Label][Test Comp E Label]", buf.String())
	})

	// f.PrettyDump(os.Stdout)
	// t.FailNow()
}
