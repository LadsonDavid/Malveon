package extractor

import (
	"math/rand"
	"reflect"
	"testing"
)

// The hand-written scanner must find exactly what the regular expression it replaced found.
func TestNextWordAgreesWithTheRegularExpression(t *testing.T) {
	alphabet := []string{"a", "Z", "_", "$", "0", "9", " ", "\n", ".", "(", "é", "日", "-", "\x00", "\xff", "ab", "$x", "1a"}
	rng := rand.New(rand.NewSource(7))
	for n := 0; n < 3000; n++ {
		var s string
		for i := rng.Intn(40); i > 0; i-- {
			s += alphabet[rng.Intn(len(alphabet))]
		}
		var want, got [][]int
		for _, m := range wordRegexp.FindAllStringIndex(s, -1) {
			want = append(want, m)
		}
		for a, b := nextWord(s, 0); a >= 0; a, b = nextWord(s, b) {
			got = append(got, []int{a, b})
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("%q: regexp %v, scanner %v", s, want, got)
		}
	}
}

// The memo must hand out copies: one caller changing its slice cannot change another's.
func TestFuncRangesMemoGivesIndependentCopies(t *testing.T) {
	src := "function a() {\n}\nconst b = () => {\n}\n"
	first := jsFuncRanges(src)
	if len(first) == 0 {
		t.Fatal("expected functions")
	}
	first[0].name = "changed"
	second := jsFuncRanges(src)
	if second[0].name == "changed" {
		t.Fatal("the memo leaked a caller's change")
	}
	other := jsFuncRanges("function z() {\n}\n")
	if len(other) != 1 || other[0].name != "z" {
		t.Fatalf("a different file got the old answer: %+v", other)
	}
}
