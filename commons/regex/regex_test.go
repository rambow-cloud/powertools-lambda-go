package regex

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/dlclark/regexp2/v2"
)

func TestNodeReplacements(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Pattern, Flags        string
			Input, Format, Result []rune
			LastIndex, FinalIndex int
		}
		Invalid []struct{ Pattern, Flags string }
	}
	data, err := os.ReadFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, item := range fixture.Cases {
		r, err := Compile(item.Pattern, item.Flags, Options{})
		if err != nil {
			t.Fatalf("case %d compile %q/%s: %v", i, item.Pattern, item.Flags, err)
		}
		r.SetLastIndex(item.LastIndex)
		value, err := r.Replace(encodeRunes(item.Input), encodeRunes(item.Format))
		actual := inputRunes(value, false)
		if err != nil || !slices.Equal(actual, item.Result) || r.LastIndex() != item.FinalIndex {
			t.Errorf("case %d %q/%s input %U format %U: got %U index %d, want %U index %d, err %v", i, item.Pattern, item.Flags, item.Input, item.Format, actual, r.LastIndex(), item.Result, item.FinalIndex, err)
		}
	}
	for _, item := range fixture.Invalid {
		if _, err := Compile(item.Pattern, item.Flags, Options{}); err == nil {
			t.Errorf("accepted invalid %q/%s", item.Pattern, item.Flags)
		}
	}
}

func TestConcurrentReplacementAndStatelessSearch(t *testing.T) {
	r, err := Compile(`(?<word>\p{Letter}+)`, "gu", Options{})
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 64 {
		workers.Go(func() {
			value, err := r.Replacer("[$<word>]")("αβ one")
			if err != nil || value != "[αβ] [one]" {
				t.Errorf("replacement %q: %v", value, err)
			}
			if matched, err := r.MatchString("αβ"); err != nil || !matched {
				t.Errorf("match %v: %v", matched, err)
			}
		})
	}
	workers.Wait()
	if r.LastIndex() != 0 {
		t.Fatal("global replacement did not reset state")
	}
	r.SetLastIndex(42)
	r.MatchString("αβ")
	if r.LastIndex() != 42 {
		t.Fatal("stateless search changed state")
	}
}

func TestOperationalFailures(t *testing.T) {
	if _, err := Compile("a", "", Options{MatchTimeout: -1}); err == nil {
		t.Fatal("negative timeout accepted")
	}
	r, err := Compile("^(a|aa)+$", "g", Options{MaxBacktrackingStackSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	r.SetLastIndex(9)
	_, err = r.Replace(strings.Repeat("a", 16)+"!", "x")
	if !errors.Is(err, regexp2.ErrBacktrackingStackLimit) {
		t.Fatalf("lost engine failure: %v", err)
	}
	if r.LastIndex() != 0 {
		t.Fatal("failed global replacement retained state")
	}
}
