package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOption_SomeNone_IsSomeIsNone(t *testing.T) {
	t.Run("Some", func(t *testing.T) {
		o := Some(42)
		require.True(t, o.IsPresent())
		require.False(t, o.IsNone())
	})

	t.Run("None", func(t *testing.T) {
		o := None[int]()
		require.False(t, o.IsPresent())
		require.True(t, o.IsNone())
	})
}

func TestOption_Get(t *testing.T) {
	t.Run("Some", func(t *testing.T) {
		v, ok := Some(42).Get()
		require.Equal(t, 42, v)
		require.True(t, ok)
	})

	t.Run("None", func(t *testing.T) {
		v, ok := None[int]().Get()
		require.Equal(t, 0, v)
		require.False(t, ok)
	})
}

func TestOption_Unwrap(t *testing.T) {
	t.Run("Some", func(t *testing.T) {
		require.Equal(t, 42, Some(42).Unwrap())
	})

	t.Run("None panics", func(t *testing.T) {
		require.Panics(t, func() { _ = None[int]().Unwrap() })
	})
}

func TestOption_UnwrapOr(t *testing.T) {
	require.Equal(t, 42, Some(42).UnwrapOr(100))
	require.Equal(t, 100, None[int]().UnwrapOr(100))
}

func TestOption_UnwrapOrElse(t *testing.T) {
	t.Run("Some does not call fallback", func(t *testing.T) {
		calls := 0
		f := func() int { calls++; return 100 }

		require.Equal(t, 42, Some(42).UnwrapOrElse(f))
		require.Equal(t, 0, calls)
	})

	t.Run("None calls fallback once", func(t *testing.T) {
		calls := 0
		f := func() int { calls++; return 100 }

		require.Equal(t, 100, None[int]().UnwrapOrElse(f))
		require.Equal(t, 1, calls)
	})
}

func TestOption_UnwrapOrZero(t *testing.T) {
	require.Equal(t, 42, Some(42).UnwrapOrZero())
	require.Equal(t, 0, None[int]().UnwrapOrZero())
	require.Equal(t, "", None[string]().UnwrapOrZero())
}

func TestOption_Or(t *testing.T) {
	require.Equal(t, Some(1), Some(1).Or(Some(2)))
	require.Equal(t, Some(2), None[int]().Or(Some(2)))
	require.Equal(t, None[int](), None[int]().Or(None[int]()))
}

func TestOption_OrElse(t *testing.T) {
	t.Run("Some does not call fallback", func(t *testing.T) {
		calls := 0
		f := func() Option[int] { calls++; return Some(99) }

		require.Equal(t, Some(42), Some(42).OrElse(f))
		require.Equal(t, 0, calls)
	})

	t.Run("None calls fallback once", func(t *testing.T) {
		calls := 0
		f := func() Option[int] { calls++; return Some(99) }

		require.Equal(t, Some(99), None[int]().OrElse(f))
		require.Equal(t, 1, calls)
	})
}

func TestOption_And(t *testing.T) {
	require.Equal(t, Some(2), Some(1).And(Some(2)))
	require.Equal(t, None[int](), None[int]().And(Some(2)))
	require.Equal(t, None[int](), Some(1).And(None[int]()))
}

func TestOption_Ptr(t *testing.T) {
	t.Run("Some returns non-nil pointer to value", func(t *testing.T) {
		o := Some(42)
		p := o.Ptr()
		require.NotNil(t, p)
		require.Equal(t, 42, *p)
	})

	t.Run("None returns nil pointer", func(t *testing.T) {
		require.Nil(t, None[int]().Ptr())
	})
}

func TestFromPtr(t *testing.T) {
	t.Run("nil pointer -> None", func(t *testing.T) {
		var p *int
		require.Equal(t, None[int](), FromPtr(p))
	})

	t.Run("non-nil pointer -> Some", func(t *testing.T) {
		x := 42
		require.Equal(t, Some(42), FromPtr(&x))
	})
}

func TestOption_String(t *testing.T) {
	require.Equal(t, "Some(42)", Some(42).String())
	require.Equal(t, "None", None[int]().String())
}

func TestEqual(t *testing.T) {
	require.True(t, Equal(None[int](), None[int]()))
	require.False(t, Equal(Some(1), None[int]()))
	require.False(t, Equal(None[int](), Some(1)))
	require.True(t, Equal(Some(42), Some(42)))
	require.False(t, Equal(Some(42), Some(43)))
}

func TestMapOpt(t *testing.T) {
	t.Run("Some maps to Some", func(t *testing.T) {
		o := Some(21)
		mapped := o.Map(func(x int) int { return x * 2 })
		require.Equal(t, Some(42), mapped)
	})

	t.Run("None maps to None", func(t *testing.T) {
		o := None[int]()
		mapped := o.Map(func(x int) int { return x * 2 })
		require.Equal(t, None[int](), mapped)
	})
}

func TestCollectValues(t *testing.T) {
	options := []Option[int]{Some(1), None[int](), Some(2), Some(3), None[int]()}
	values := CollectValues(options)
	require.Equal(t, []int{1, 2, 3}, values)
}

func TestCoalesceOpt(t *testing.T) {
	t.Run("returns first Some", func(t *testing.T) {
		result := CoalesceOpt(None[int](), None[int](), Some(3), Some(4))
		require.Equal(t, Some(3), result)
	})

	t.Run("all None returns None", func(t *testing.T) {
		result := CoalesceOpt(None[int](), None[int](), None[int]())
		require.Equal(t, None[int](), result)
	})

	t.Run("no args returns None", func(t *testing.T) {
		result := CoalesceOpt[int]()
		require.Equal(t, None[int](), result)
	})

	t.Run("first arg is Some", func(t *testing.T) {
		result := CoalesceOpt(Some(1), Some(2))
		require.Equal(t, Some(1), result)
	})
}

func TestFlatMapOpt(t *testing.T) {
	half := func(x int) Option[int] {
		if x%2 == 0 {
			return Some(x / 2)
		}
		return None[int]()
	}

	t.Run("Some and f returns Some", func(t *testing.T) {
		result := Some(42).FlatMap(half)
		require.Equal(t, Some(21), result)
	})

	t.Run("Some and f returns None", func(t *testing.T) {
		result := Some(3).FlatMap(half)
		require.Equal(t, None[int](), result)
	})

	t.Run("None skips f", func(t *testing.T) {
		called := false
		f := func(x int) Option[int] { called = true; return Some(x) }
		result := None[int]().FlatMap(f)
		require.Equal(t, None[int](), result)
		require.False(t, called)
	})
}

func TestOption_Scan_String(t *testing.T) {
	t.Run("scan string value into Some", func(t *testing.T) {
		var opt Option[string]
		err := opt.Scan("hello")
		require.NoError(t, err)
		require.True(t, opt.IsPresent())
		require.Equal(t, "hello", opt.Unwrap())
	})

	t.Run("scan nil into None", func(t *testing.T) {
		var opt Option[string]
		err := opt.Scan(nil)
		require.NoError(t, err)
		require.True(t, opt.IsNone())
	})
}

func TestOption_Value_String(t *testing.T) {
	t.Run("Some returns string", func(t *testing.T) {
		opt := Some("hello")
		val, err := opt.Value()
		require.NoError(t, err)
		require.Equal(t, "hello", val)
	})

	t.Run("None returns nil", func(t *testing.T) {
		opt := None[string]()
		val, err := opt.Value()
		require.NoError(t, err)
		require.Nil(t, val)
	})
}

func TestOption_Scan_Int(t *testing.T) {
	t.Run("scan int value into Some", func(t *testing.T) {
		var opt Option[int]
		err := opt.Scan(42)
		require.NoError(t, err)
		require.True(t, opt.IsPresent())
		require.Equal(t, 42, opt.Unwrap())
	})

	t.Run("scan nil into None", func(t *testing.T) {
		var opt Option[int]
		err := opt.Scan(nil)
		require.NoError(t, err)
		require.True(t, opt.IsNone())
	})
}

func TestOption_Scan_Int32(t *testing.T) {
	t.Run("scan int value into Some", func(t *testing.T) {
		var opt Option[int32]
		err := opt.Scan(int64(42))
		require.NoError(t, err)
		require.True(t, opt.IsPresent())
		require.Equal(t, int32(42), opt.Unwrap())
	})

	t.Run("scan nil into None", func(t *testing.T) {
		var opt Option[int32]
		err := opt.Scan(nil)
		require.NoError(t, err)
		require.True(t, opt.IsNone())
	})
}

func TestOption_Value_Int(t *testing.T) {
	t.Run("Some returns int", func(t *testing.T) {
		opt := Some(42)
		val, err := opt.Value()
		require.NoError(t, err)
		require.Equal(t, 42, val)
	})

	t.Run("None returns nil", func(t *testing.T) {
		opt := None[int]()
		val, err := opt.Value()
		require.NoError(t, err)
		require.Nil(t, val)
	})
}

func TestOption_Non_Zero(t *testing.T) {
	t.Run("NonZeroOption with non-zero value returns Some", func(t *testing.T) {
		opt := NonZeroOption(42)
		require.True(t, opt.IsPresent())
		require.Equal(t, 42, opt.Unwrap())
		optStr := NonZeroOption("hello")
		require.True(t, optStr.IsPresent())
		require.Equal(t, "hello", optStr.Unwrap())
	})

	t.Run("NonZeroOption with zero value returns None", func(t *testing.T) {
		opt := NonZeroOption(0)
		require.True(t, opt.IsNone())
		optStr := NonZeroOption("")
		require.True(t, optStr.IsNone())
	})
}

func TestOption_Filter(t *testing.T) {
	isEven := func(x int) bool { return x%2 == 0 }

	t.Run("Some and predicate true returns Some", func(t *testing.T) {
		require.Equal(t, Some(42), Some(42).Filter(isEven))
	})

	t.Run("Some and predicate false returns None", func(t *testing.T) {
		require.Equal(t, None[int](), Some(41).Filter(isEven))
	})

	t.Run("None skips predicate and returns None", func(t *testing.T) {
		called := false
		f := func(x int) bool { called = true; return true }
		require.Equal(t, None[int](), None[int]().Filter(f))
		require.False(t, called)
	})
}

func TestOption_IsSomeAnd(t *testing.T) {
	isEven := func(x int) bool { return x%2 == 0 }

	t.Run("Some and predicate true returns true", func(t *testing.T) {
		require.True(t, Some(42).IsSomeAnd(isEven))
	})

	t.Run("Some and predicate false returns false", func(t *testing.T) {
		require.False(t, Some(41).IsSomeAnd(isEven))
	})

	t.Run("None skips predicate and returns false", func(t *testing.T) {
		called := false
		f := func(x int) bool { called = true; return true }
		require.False(t, None[int]().IsSomeAnd(f))
		require.False(t, called)
	})
}

func TestOption_IsNoneOrDefault(t *testing.T) {
	t.Run("None returns true", func(t *testing.T) {
		require.True(t, None[int]().IsNoneOrDefault())
		require.True(t, None[string]().IsNoneOrDefault())
	})

	t.Run("Some with zero value returns true", func(t *testing.T) {
		require.True(t, Some(0).IsNoneOrDefault())
		require.True(t, Some("").IsNoneOrDefault())
	})

	t.Run("Some with non-zero value returns false", func(t *testing.T) {
		require.False(t, Some(42).IsNoneOrDefault())
		require.False(t, Some("hello").IsNoneOrDefault())
	})
}
