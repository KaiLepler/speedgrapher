package seo

import (
	"context"
	"testing"
)

func TestScoreImpactsAndSeverity(t *testing.T) {
	for _, keyword := range []string{"", "test"} {
		_, res, err := Handler(context.Background(), nil, SEOParams{HTML: "<html><body></body></html>", Keyword: keyword})
		if err != nil {
			t.Fatal(err)
		}
		deductions := 0
		for _, check := range res.Checks {
			deductions += check.ScoreImpact
			if (check.Name == "Title Tag" || check.Name == "Meta Description") && check.Status != "fail" {
				t.Fatalf("missing metadata downgraded: %+v", check)
			}
		}
		want := 100 - deductions
		if want < 0 {
			want = 0
		}
		if res.Score != want {
			t.Fatalf("score %d does not reconcile with deductions %d", res.Score, deductions)
		}
	}
}
