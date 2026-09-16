package extractor

import "testing"

func TestObjectLiteralKeys(t *testing.T) {
	cases := []struct {
		raw      string
		want     []string
		resolved bool
	}{
		{`{ amount: x, reason: y }`, []string{"amount", "reason"}, true},
		{`{ amount, reason }`, []string{"amount", "reason"}, true},
		{`{ 'amount': x }`, []string{"amount"}, true},
		{`{ amount: { min: 1 } }`, []string{"amount"}, true},
		{`{ ...defaults, amount: x }`, nil, false},
		{`payload`, nil, false},
		{`{}`, []string{}, true},
	}
	for _, tc := range cases {
		got, resolved := objectLiteralKeys(tc.raw)
		if resolved != tc.resolved {
			t.Errorf("objectLiteralKeys(%q) resolved = %v, want %v", tc.raw, resolved, tc.resolved)
			continue
		}
		if !resolved {
			continue
		}
		if !equalSets(got, tc.want) {
			t.Errorf("objectLiteralKeys(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestJSSentFieldsFromCallArgs(t *testing.T) {
	cases := []struct {
		name     string
		argsList string
		want     []string
		resolved bool
	}{
		{
			name:     "fetch with JSON.stringify",
			argsList: `"/refund", { method: 'POST', body: JSON.stringify({ amount: x, reason: y }) }`,
			want:     []string{"amount", "reason"},
			resolved: true,
		},
		{
			name:     "fetch with direct object body",
			argsList: `"/refund", { method: 'POST', body: { amount: x } }`,
			want:     []string{"amount"},
			resolved: true,
		},
		{
			name:     "axios positional body",
			argsList: `"/refund", { amount: x, reason: y }`,
			want:     []string{"amount", "reason"},
			resolved: true,
		},
		{
			name:     "GET with no body",
			argsList: `"/refund"`,
			resolved: false,
		},
		{
			name:     "body built from a variable",
			argsList: `"/refund", { method: 'POST', body: JSON.stringify(payload) }`,
			resolved: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, resolved := jsSentFieldsFromCallArgs(tc.argsList)
			if resolved != tc.resolved {
				t.Fatalf("resolved = %v, want %v (fields: %v)", resolved, tc.resolved, got)
			}
			if resolved && !equalSets(got, tc.want) {
				t.Errorf("fields = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJSReadFieldsFromHandler(t *testing.T) {
	cases := []struct {
		name     string
		argsList string
		want     []string
	}{
		{
			name:     "dot access",
			argsList: `"/refund", function (req, res) { const amount = req.body.amount; doRefund(amount, req.body.reason); }`,
			want:     []string{"amount", "reason"},
		},
		{
			name:     "bracket access",
			argsList: `"/refund", function (req, res) { const amount = req.body['amount']; }`,
			want:     []string{"amount"},
		},
		{
			name:     "destructure",
			argsList: `"/refund", function (req, res) { const { amount, reason } = req.body; }`,
			want:     []string{"amount", "reason"},
		},
		{
			name:     "no req.body access at all",
			argsList: `"/refund", function (req, res) { res.send("ok"); }`,
			want:     []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, resolved := jsReadFieldsFromHandler(tc.argsList)
			if !resolved {
				t.Fatalf("expected resolved, got unresolved")
			}
			if !equalSets(got, tc.want) {
				t.Errorf("fields = %v, want %v", got, tc.want)
			}
		})
	}
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, x := range a {
		seen[x] = true
	}
	for _, x := range b {
		if !seen[x] {
			return false
		}
	}
	return true
}
