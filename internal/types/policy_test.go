package types

import "testing"

func TestTierRank(t *testing.T) {
	cases := []struct {
		tier ModelTier
		want int
	}{
		{TierFast, 1},
		{TierBalanced, 2},
		{TierReasoning, 3},
		{ModelTier("unknown"), 0},
	}
	for _, tc := range cases {
		if got := TierRank(tc.tier); got != tc.want {
			t.Errorf("TierRank(%q) = %d, want %d", tc.tier, got, tc.want)
		}
	}
}
