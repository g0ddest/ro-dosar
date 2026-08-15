package api

import (
	"testing"
	"time"
)

var fixtureNow = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

func art11Fixture() CategoryStatsResponse {
	mk := func(year, total, solved int) YearStatsResponse {
		return YearStatsResponse{Year: year, Total: total, Solved: solved}
	}
	return CategoryStatsResponse{
		Category: StatsCategoryRef{Code: "ART_11"},
		Years: []YearStatsResponse{
			mk(2010, 94431, 3), mk(2011, 1, 1), mk(2012, 87009, 6), mk(2013, 51216, 7),
			mk(2014, 19949, 3), mk(2015, 79466, 78999), mk(2016, 111454, 110694),
			mk(2017, 92104, 90444), mk(2018, 93756, 85613), mk(2019, 96643, 82853),
			mk(2020, 20333, 16672), mk(2021, 38681, 29149), mk(2022, 31325, 19623),
			mk(2023, 38748, 7783), mk(2024, 45764, 1040), mk(2025, 18213, 4), mk(2026, 5115, 1),
		},
		RecentActivity: []YearActivityResponse{
			{Year: 2023, Solved: 3534}, {Year: 2024, Solved: 556},
		},
	}
}

func TestWave_HighPercentile2024(t *testing.T) {
	q := buildWaveEstimate(art11Fixture(), 2024, 39946, fixtureNow)
	if q == nil {
		t.Fatal("expected estimate")
	}
	if q.CohortTotal != 45764 || q.Percentile != 0.87 || q.WavePassed {
		t.Fatalf("wrong basics: %+v", q)
	}
	// phi = plateau at cohort 2021 share 29149/38681 = 0.7536 (first share >= 0.5
	// whose next increment < 0.08); target = 0.8729*0.7536 = 0.6578; crossing
	// between cohorts 2022 (0.6264) and 2021 (0.7536) gives t ~ 26.96 months
	// (> 12, no rate correction); range = [floor(0.8t), ceil(1.3t)] = [21, 36]
	if q.EstimatedMonthsMin == nil || q.EstimatedMonthsMax == nil {
		t.Fatal("expected months range")
	}
	if *q.EstimatedMonthsMin != 21 || *q.EstimatedMonthsMax != 36 {
		t.Errorf("expected [21, 36], got [%d, %d]", *q.EstimatedMonthsMin, *q.EstimatedMonthsMax)
	}
}

func TestWave_RateCorrectionLowPercentile(t *testing.T) {
	q := buildWaveEstimate(art11Fixture(), 2024, 5000, fixtureNow)
	if q == nil || q.WavePassed {
		t.Fatalf("expected estimate: %+v", q)
	}
	// tCurve ~ 4.0 months; rate floor with R90 = 556:
	// tRate = (0.0823 - 0.0227) * 45764 / (556/3) = 14.72 months, but the floor
	// saturates at waveRateWindowMonth (12) -> rateFloor = min(14.72, 12) = 12
	// -> t = max(4.0, 12) = 12 -> range [floor(0.8*12), ceil(1.3*12)] = [9, 16]
	if q.EstimatedMonthsMin == nil || *q.EstimatedMonthsMin != 9 || *q.EstimatedMonthsMax != 16 {
		t.Errorf("expected [9, 16], got %+v", q)
	}
}

func TestWave_WavePassed(t *testing.T) {
	// cohort 2022 is 62.6%% solved; q = 15000/31325 = 0.479, target = 0.361 < 0.626
	q := buildWaveEstimate(art11Fixture(), 2022, 15000, fixtureNow)
	if q == nil || !q.WavePassed {
		t.Fatalf("expected wavePassed: %+v", q)
	}
	if q.EstimatedMonthsMin != nil || q.EstimatedMonthsMax != nil {
		t.Error("wavePassed must carry no months")
	}
}

func TestWave_InsufficientCurve(t *testing.T) {
	cat := CategoryStatsResponse{
		Category: StatsCategoryRef{Code: "ART_8_2"},
		Years: []YearStatsResponse{
			{Year: 2025, Total: 200, Solved: 0},
			{Year: 2026, Total: 150, Solved: 0},
		},
	}
	q := buildWaveEstimate(cat, 2026, 100, fixtureNow)
	if q == nil {
		t.Fatal("expected percentile-only response")
	}
	if q.WavePassed || q.EstimatedMonthsMin != nil || q.EstimatedMonthsMax != nil {
		t.Errorf("expected no estimate fields: %+v", q)
	}
	if q.CohortTotal != 150 {
		t.Errorf("wrong cohortTotal: %d", q.CohortTotal)
	}
}

func TestWave_TargetBeyondEnvelope(t *testing.T) {
	// increments all >= 0.08 -> phi defaults to 0.85; envelope max share 0.6;
	// q = 1.0 -> target 0.85 > 0.6 -> months unavailable
	cat := CategoryStatsResponse{
		Category: StatsCategoryRef{Code: "ART_10"},
		Years: []YearStatsResponse{
			{Year: 2022, Total: 1000, Solved: 600},
			{Year: 2023, Total: 1000, Solved: 300},
			{Year: 2024, Total: 1000, Solved: 100},
		},
	}
	q := buildWaveEstimate(cat, 2024, 1000, fixtureNow)
	if q == nil || q.WavePassed || q.EstimatedMonthsMin != nil {
		t.Errorf("expected percentile-only: %+v", q)
	}
}

func TestWave_UnknownCohort(t *testing.T) {
	if q := buildWaveEstimate(art11Fixture(), 2005, 100, fixtureNow); q != nil {
		t.Errorf("expected nil for unknown cohort, got %+v", q)
	}
}

func TestWave_CensoringExcludesOldYears(t *testing.T) {
	// with censoring, the envelope starts at cohort 2015 (share 0.994); if the
	// censored 2010-2014 cohorts (share ~0) entered the envelope, the running
	// maximum would be unchanged BUT phi/interp would see extra low points at
	// high ages breaking monotonicity of ages->shares; assert the estimate for
	// the high-percentile dossier is unchanged when censored years are removed
	// from the fixture entirely
	full := buildWaveEstimate(art11Fixture(), 2024, 39946, fixtureNow)
	trimmed := art11Fixture()
	var kept []YearStatsResponse
	for _, y := range trimmed.Years {
		if y.Year >= 2015 {
			kept = append(kept, y)
		}
	}
	trimmed.Years = kept
	cut := buildWaveEstimate(trimmed, 2024, 39946, fixtureNow)
	if *full.EstimatedMonthsMin != *cut.EstimatedMonthsMin || *full.EstimatedMonthsMax != *cut.EstimatedMonthsMax {
		t.Errorf("censoring must neutralize pre-2015 phantoms: full=%+v cut=%+v", full, cut)
	}
}

func TestWave_MonotoneAcrossRateBoundary(t *testing.T) {
	lo := buildWaveEstimate(art11Fixture(), 2024, 12199, fixtureNow)
	hi := buildWaveEstimate(art11Fixture(), 2024, 12222, fixtureNow)
	if lo == nil || hi == nil || lo.EstimatedMonthsMax == nil || hi.EstimatedMonthsMax == nil {
		t.Fatalf("expected estimates: %+v %+v", lo, hi)
	}
	if *hi.EstimatedMonthsMin < *lo.EstimatedMonthsMin || *hi.EstimatedMonthsMax < *lo.EstimatedMonthsMax {
		t.Errorf("estimate must be monotone in the number: lo=%+v hi=%+v", lo, hi)
	}
}

func TestWave_CensoredCohortWavePassed(t *testing.T) {
	q := buildWaveEstimate(art11Fixture(), 2013, 100, fixtureNow)
	if q == nil || !q.WavePassed {
		t.Fatalf("censored-cohort dossier must report wavePassed: %+v", q)
	}
	if q.EstimatedMonthsMin != nil || q.EstimatedMonthsMax != nil {
		t.Error("wavePassed must carry no months")
	}
}
