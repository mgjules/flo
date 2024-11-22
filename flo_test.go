package flo_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mgjules/flo"
	"github.com/stretchr/testify/require"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
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
		"flo",
		"Test Package Flo Description",
	)
	require.NoError(t, err)
	require.NotNil(t, f)

	t.Run("Add flo IO", func(t *testing.T) {
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
			"result",
			flo.ComponentIOTypeOUT,
			reflect.TypeFor[int](),
			f.ID,
		)
		require.NoError(t, err)
		require.NotNil(t, rInt)
		require.NoError(t, f.AddIO(rInt))

		rErr, err := flo.NewComponentIO(
			"err",
			flo.ComponentIOTypeOUT,
			reflect.TypeFor[error](),
			f.ID,
		)
		require.NoError(t, err)
		require.NotNil(t, rErr)
		require.NoError(t, f.AddIO(rErr))
	})

	compA, err := flo.NewComponent(
		"CompA",
		"githab.com/testuf/tera",
		"Test Comp A Label",
		"Test Comp A Description",
		(compA{val: 10}).AddVal,
	)
	require.NoError(t, err)
	require.NotNil(t, compA)
	require.NoError(t, f.AddComponent(compA))

	compB, err := flo.NewComponent(
		"CompB",
		"githab.com/testurrf/terb",
		"Test Comp B Label",
		"Test Comp B Description",
		compBFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compB)
	require.NoError(t, f.AddComponent(compB))

	compC, err := flo.NewComponent(
		"CompC",
		"githab.com/testuf/tera",
		"Test Comp C Label",
		"Test Comp C Description",
		compCFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compC)
	require.NoError(t, f.AddComponent(compC))

	compD, err := flo.NewComponent(
		"CompD",
		"githab.com/testam/taaar",
		"Test Comp D Label",
		"Test Comp D Description",
		compDFn,
	)
	require.NoError(t, err)
	require.NotNil(t, compD)
	require.NoError(t, f.AddComponent(compD))

	compE, err := flo.NewComponent(
		"CompE",
		"gitlub.com/testing/teag",
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

	src := &bytes.Buffer{}
	t.Run("Render", func(t *testing.T) {
		err = f.Render(context.Background(), src)
		require.NoError(t, err)
		require.Equal(t, `// Code generated by flo. Do not edit!

// Test Package Flo Description
package flo

import (
	taaar "githab.com/testam/taaar"
	tera "githab.com/testuf/tera"
	terb "githab.com/testurrf/terb"
	teag "gitlub.com/testing/teag"
)

func TestSync(ctx context.Context, in int) (int, error) {
	// Test Comp A Description
	ioff39613112342A272B0Edf2D60F8Cedd6Da8A1A0 := tera.CompA(ctx, in)

	// Test Comp D Description
	ioa94Cdb2B64820B08Fbac3Df6700F0418263458Cc := taaar.CompD()

	// Test Comp B Description
	iod8E895F4A10213A36E8626E91E455191C1886Cb0, err := terb.CompB(in, ioa94Cdb2B64820B08Fbac3Df6700F0418263458Cc)
	if err != nil {
		return 0, err
	}

	// Test Comp C Description
	ioaa5Ab25F0Cbe490A08347F8F66917A4Bd0899412, err := tera.CompC(ctx, ioff39613112342A272B0Edf2D60F8Cedd6Da8A1A0, iod8E895F4A10213A36E8626E91E455191C1886Cb0)
	if err != nil {
		return 0, err
	}

	// Test Comp E Description
	teag.CompE()

	return ioaa5Ab25F0Cbe490A08347F8F66917A4Bd0899412, nil
}
`, src.String())
	})

	t.Run("Execute", func(t *testing.T) {
		symbols := f.Symbols()
		require.Len(t, symbols, 4)

		i := interp.New(interp.Options{})

		require.NoError(t, i.Use(stdlib.Symbols))
		require.NoError(t, i.Use(symbols))
		i.ImportUsed()

		_, err := i.Eval(src.String())
		require.NoError(t, err)

		v, err := i.Eval("flo.TestSync")
		require.NoError(t, err)
		require.True(t, v.IsValid())
		require.True(t, v.CanInterface())

		testSync, ok := v.Interface().(func(context.Context, int) (int, error))
		require.True(t, ok)

		result, err := testSync(context.Background(), 2)
		require.NoError(t, err)
		require.Equal(t, 15, result)
	})

	// f.PrettyDump(os.Stdout)
	// t.FailNow()
}
