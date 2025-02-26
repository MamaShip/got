package diff_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ysmood/got/lib/diff"
)

func TestReduce(t *testing.T) {
	eq := func(x, y string, e string) {
		t.Helper()

		out := diff.NewString(x).Reduce(diff.NewString(y)).String()
		if out != e {
			t.Error(out, "!=", e)
		}
	}

	eq("", "", "")
	eq("", "a", "")
	eq("a", "", "")
	eq("abc", "abc", "abc")
	eq("abc", "acb", "abc")
	eq("abc", "acbc", "abc")
	eq("abc", "xxx", "")
	eq("ac", "bc", "c")
}

func TestCommon(t *testing.T) {
	eq := func(x, y string, el, er int) {
		t.Helper()

		l, r := diff.NewString(x).Common(diff.NewString(y))

		if l != el || r != er {
			t.Error(l, r, "!=", el, er)
		}
	}

	eq("", "", 0, 0)
	eq("", "a", 0, 0)
	eq("a", "", 0, 0)
	eq("abc", "abc", 3, 0)
	eq("abc", "acb", 1, 0)
	eq("abc", "acbc", 1, 2)
	eq("abc", "xxx", 0, 0)
	eq("ac", "bc", 0, 1)
}

func TestLCSString(t *testing.T) {
	eq := func(x, y, expected string) {
		t.Helper()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		lcs := diff.NewString(x).LCS(ctx, diff.NewString(y))
		out := lcs.String()

		if out != expected {
			t.Error(out, "!=", expected)
		}
	}

	eq("", "", "")
	eq("abc", "acb", "ab")
	eq("abc", "acbc", "abc")
	eq("abc", "xxx", "")
	eq("ac", "bc", "c")
	eq("gac", "agcat", "ga")
	eq("agcat", "gac", "ac")

	x := bytes.Repeat([]byte("x"), 10000)
	y := bytes.Repeat([]byte("y"), 10000)
	eq(string(x), string(y), "")

	x[len(x)/2] = byte('a')
	y[len(y)/2] = byte('a')
	eq(string(x), string(y), "a")

	x[len(x)/2] = byte('y')
	y[len(y)/2] = byte('x')
	eq(string(x), string(y), "yx")
}

func TestText(t *testing.T) {
	g := setup(t)
	g.Len(diff.NewText("a"), 1)
	g.Len(diff.NewText("a\n"), 2)
	g.Len(diff.NewText("a\n\n"), 3)
	g.Len(diff.NewText("\na"), 2)
}

func TestLCSText(t *testing.T) {
	g := setup(t)
	eq := func(x, y, expected string) {
		t.Helper()

		x = strings.Join(strings.Split(x, ""), "\n")
		y = strings.Join(strings.Split(y, ""), "\n")
		expected = strings.Join(strings.Split(expected, ""), "\n")

		lcs := diff.NewText(x).LCS(context.Background(), diff.NewText(y))
		g.Eq(lcs.String(), expected)
	}

	eq("", "", "")
	eq("abc", "acb", "ab")
	eq("abc", "acbc", "abc")
	eq("abc", "xxx", "")
}
