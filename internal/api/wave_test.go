package api

import (
	"encoding/json"
	"testing"
	"time"

	"ro-dosar/internal/repository"
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
	q := buildWaveEstimate(art11Fixture(), nil, 2024, 39946, fixtureNow)
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
	q := buildWaveEstimate(art11Fixture(), nil, 2024, 5000, fixtureNow)
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
	q := buildWaveEstimate(art11Fixture(), nil, 2022, 15000, fixtureNow)
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
	q := buildWaveEstimate(cat, nil, 2026, 100, fixtureNow)
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
	q := buildWaveEstimate(cat, nil, 2024, 1000, fixtureNow)
	if q == nil || q.WavePassed || q.EstimatedMonthsMin != nil {
		t.Errorf("expected percentile-only: %+v", q)
	}
}

func TestWave_UnknownCohort(t *testing.T) {
	if q := buildWaveEstimate(art11Fixture(), nil, 2005, 100, fixtureNow); q != nil {
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
	full := buildWaveEstimate(art11Fixture(), nil, 2024, 39946, fixtureNow)
	trimmed := art11Fixture()
	var kept []YearStatsResponse
	for _, y := range trimmed.Years {
		if y.Year >= 2015 {
			kept = append(kept, y)
		}
	}
	trimmed.Years = kept
	cut := buildWaveEstimate(trimmed, nil, 2024, 39946, fixtureNow)
	if *full.EstimatedMonthsMin != *cut.EstimatedMonthsMin || *full.EstimatedMonthsMax != *cut.EstimatedMonthsMax {
		t.Errorf("censoring must neutralize pre-2015 phantoms: full=%+v cut=%+v", full, cut)
	}
}

func TestWave_MonotoneAcrossRateBoundary(t *testing.T) {
	lo := buildWaveEstimate(art11Fixture(), nil, 2024, 12199, fixtureNow)
	hi := buildWaveEstimate(art11Fixture(), nil, 2024, 12222, fixtureNow)
	if lo == nil || hi == nil || lo.EstimatedMonthsMax == nil || hi.EstimatedMonthsMax == nil {
		t.Fatalf("expected estimates: %+v %+v", lo, hi)
	}
	if *hi.EstimatedMonthsMin < *lo.EstimatedMonthsMin || *hi.EstimatedMonthsMax < *lo.EstimatedMonthsMax {
		t.Errorf("estimate must be monotone in the number: lo=%+v hi=%+v", lo, hi)
	}
}

func TestWave_CensoredCohortWavePassed(t *testing.T) {
	q := buildWaveEstimate(art11Fixture(), nil, 2013, 100, fixtureNow)
	if q == nil || !q.WavePassed {
		t.Fatalf("censored-cohort dossier must report wavePassed: %+v", q)
	}
	if q.EstimatedMonthsMin != nil || q.EstimatedMonthsMax != nil {
		t.Error("wavePassed must carry no months")
	}
}

func TestWave_CurrentYearProjectedDenominator(t *testing.T) {
	// fixtureNow is mid-August 2026: elapsed ~0.62, so cohort 2026's partial
	// total (5115) is projected to ~8250 and q for No. 500 drops from ~0.098
	// to ~0.06; displayed cohortTotal stays the actual 5115
	q := buildWaveEstimate(art11Fixture(), nil, 2026, 500, fixtureNow)
	if q == nil || q.CohortTotal != 5115 {
		t.Fatalf("wrong cohortTotal: %+v", q)
	}
	if q.Percentile > 0.07 {
		t.Errorf("expected projected percentile ~0.06, got %v", q.Percentile)
	}
}

func TestWave_SmallCohortPercentileOnly(t *testing.T) {
	cat := art11Fixture()
	cat.Years = append(cat.Years, YearStatsResponse{Year: 2009, Total: 40, Solved: 5})
	q := buildWaveEstimate(cat, nil, 2009, 20, fixtureNow)
	if q == nil || q.CohortTotal != 40 {
		t.Fatalf("expected percentile-only for small cohort: %+v", q)
	}
	if q.WavePassed || q.EstimatedMonthsMin != nil || q.EstimatedMonthsMax != nil {
		t.Errorf("small cohort must carry no estimate: %+v", q)
	}
}

func TestWave_ZeroPaceFloor(t *testing.T) {
	// cohort 2025 has no recent activity (r90 == 0) and a tiny tCurve target:
	// the rate floor saturates at 12 months -> [9, 16]
	q := buildWaveEstimate(art11Fixture(), nil, 2025, 100, fixtureNow)
	if q == nil || q.WavePassed || q.EstimatedMonthsMin == nil {
		t.Fatalf("expected estimate: %+v", q)
	}
	if *q.EstimatedMonthsMin != 9 || *q.EstimatedMonthsMax != 16 {
		t.Errorf("expected zero-pace floor [9, 16], got [%d, %d]", *q.EstimatedMonthsMin, *q.EstimatedMonthsMax)
	}
}

// art11Cells is the production cohort matrix of 2026-08-15 (research fixture);
// counts/p50/p90 are the live values, two dirty solYear rows included
func art11Cells() []CohortCellResponse {
	c := func(reg, sol, n, p50, p90 int) CohortCellResponse {
		return CohortCellResponse{RegYear: reg, SolYear: sol, Count: n, P50: p50, P90: p90}
	}
	return []CohortCellResponse{
		c(2018, 2019, 282, 31811, 72793), c(2018, 2020, 40078, 24816, 47839),
		c(2018, 2021, 35403, 71895, 93729), c(2018, 2022, 5472, 71166, 94995),
		c(2018, 2023, 1452, 73492, 97719), c(2018, 2024, 1369, 65950, 94455),
		c(2018, 2025, 1143, 51222, 93531), c(2018, 2026, 401, 52796, 96178),
		c(2019, 2021, 7873, 7499, 15036), c(2019, 2022, 44175, 41470, 74620),
		c(2019, 2023, 19834, 86440, 100391), c(2019, 2024, 6504, 70905, 98232),
		c(2019, 2025, 3616, 73514, 97490), c(2019, 2026, 804, 66706, 95653),
		c(2021, 2023, 3681, 9594, 15405), c(2021, 2024, 14971, 26858, 41869),
		c(2021, 2025, 5559, 27446, 41640), c(2021, 2026, 4858, 20570, 36526),
		c(2022, 2023, 106, 5880, 16887), c(2022, 2024, 1464, 4082, 30288),
		c(2022, 2025, 15462, 17826, 32849), c(2022, 2026, 2587, 33224, 39107),
		c(2023, 2024, 107, 6362, 14091), c(2023, 2025, 1262, 15259, 38333),
		c(2023, 2026, 6407, 9629, 39055),
		c(2024, 2026, 1040, 11000, 21323),
		// dirty solution-year typos observed live — must be filtered out;
		// the 2023/8202 one is deliberately influential: unfiltered it gives
		// cohort 2023 a second wave point with age 6179 and explodes the curve
		c(2023, 8202, 5000, 30000, 41000), c(2018, 2230, 1, 52734, 52734),
	}
}

func TestWaveV21_FlagshipCalibrated(t *testing.T) {
	// Derivation (all per the spec's algorithm, fixtureNow = 2026-08-15):
	// spans (maxP90 of cells n>=300, /0.95): 2018 ratio 1.097, 2019 1.093,
	// 2021 1.139, 2022 1.314, 2023 1.061 (valid); 2024 own ratio 0.490 -> invalid;
	// ratioMedian over 3 most recent valid {2021,2022,2023} = 1.1394;
	// span(2024) = 45764*1.1394 = 52142.5 -> q = 39946/52142.5 = 0.7661 -> percentile 0.77.
	// curve (cells n >= max(50, 5% of total), monotone-q walk, cohorts with >=2 pts):
	// 2018 (0.241,2)(0.699,3); 2019 (0.071,2)(0.392,3)(0.818,4);
	// 2021 (0.218,2)(0.609,3)(0.623,4); 2022 (0.433,3)(0.807,4); 2023 gives 1 pt -> excluded.
	// pooled+running-max age at q=0.7661 -> 4.0; t = (2024.5+4.0-2026.6205)*12 = 22.55 mo;
	// rate floor (q*phi=0.577 share target, R90=556) -> min(137,12)=12, t stays 22.55;
	// v2.1 band: [floor(0.8t), ceil(1.5t)] = [18, 34].
	q := buildWaveEstimate(art11Fixture(), art11Cells(), 2024, 39946, fixtureNow)
	if q == nil || q.WavePassed {
		t.Fatalf("expected estimate: %+v", q)
	}
	if q.Percentile != 0.77 {
		t.Errorf("expected span-corrected percentile 0.77, got %v", q.Percentile)
	}
	if q.EstimatedMonthsMin == nil || *q.EstimatedMonthsMin != 18 || *q.EstimatedMonthsMax != 34 {
		t.Errorf("expected [18, 34], got %+v", q)
	}
}

func TestWaveV21_NilCellsReproducesV2(t *testing.T) {
	q := buildWaveEstimate(art11Fixture(), nil, 2024, 39946, fixtureNow)
	if q == nil || q.Percentile != 0.87 ||
		q.EstimatedMonthsMin == nil || *q.EstimatedMonthsMin != 21 || *q.EstimatedMonthsMax != 36 {
		t.Errorf("nil cells must reproduce v2 exactly ([21, 36], percentile 0.87): %+v", q)
	}
}

func TestWaveV21_DirtyCellsHarmless(t *testing.T) {
	clean := buildWaveEstimate(art11Fixture(), art11Cells()[:len(art11Cells())-2], 2024, 39946, fixtureNow)
	dirty := buildWaveEstimate(art11Fixture(), art11Cells(), 2024, 39946, fixtureNow)
	if *clean.EstimatedMonthsMin != *dirty.EstimatedMonthsMin || *clean.EstimatedMonthsMax != *dirty.EstimatedMonthsMax {
		t.Errorf("dirty solYear cells must be filtered: clean=%+v dirty=%+v", clean, dirty)
	}
}

func TestWaveV21_ShortCurveFallsBackToEnvelope(t *testing.T) {
	// only cohort 2023's cells: one wave point -> curve too short -> v2 envelope
	// path (with span-corrected q) must be used; band multiplier stays 1.3.
	var subset []CohortCellResponse
	for _, c := range art11Cells() {
		if c.RegYear == 2023 {
			subset = append(subset, c)
		}
	}
	q := buildWaveEstimate(art11Fixture(), subset, 2024, 39946, fixtureNow)
	if q == nil || q.EstimatedMonthsMin == nil {
		t.Fatalf("expected estimate: %+v", q)
	}
	// span(2024) = 45764 * 1.0610 (2023's ratio is the only valid one) = 48557;
	// q = 0.8226 -> target 0.6199; envelope crossing between cohorts 2022 (0.6264)...
	// target < 0.6264 crossing between 2023 (0.2009, age 3.12) and 2022 (0.6264, age 4.12):
	// frac = (0.6199-0.2009)/0.4256 = 0.9845 -> aStar-age(2024) = 1.9845+...
	// tCurve = ((2023+0.5+ (age interp)) ... = 12*(interp offset) — derive numerically:
	// aStar = 3.12055+0.9845 = 4.105; t = (2024.5+... use aStar-age(2024)=4.105-2.1206=1.985 yr = 23.82 mo
	// band 1.3: [floor(19.05), ceil(30.96)] = [19, 31]
	if *q.EstimatedMonthsMin != 19 || *q.EstimatedMonthsMax != 31 {
		t.Errorf("expected envelope-path [19, 31], got [%d, %d]", *q.EstimatedMonthsMin, *q.EstimatedMonthsMax)
	}
}

func TestGetDocument_WaveUsesCohortCells(t *testing.T) {
	docRepo := newQueueDocument(t, "39946/RD/2024", time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), "ART_11", false)
	fix := art11Fixture()
	statsRepo := &MockStatsRepository{}
	for _, y := range fix.Years {
		statsRepo.yearly = append(statsRepo.yearly, repository.CategoryYearStats{Category: "ART_11", Year: y.Year, Total: y.Total, Solved: y.Solved})
	}
	for _, a := range fix.RecentActivity {
		statsRepo.activity = append(statsRepo.activity, repository.CategoryYearActivity{Category: "ART_11", Year: a.Year, Solved: a.Solved})
	}
	for _, c := range art11Cells() {
		statsRepo.cohorts = append(statsRepo.cohorts, repository.CohortCell{Category: "ART_11", RegYear: c.RegYear, SolYear: c.SolYear, Count: c.Count, P50: c.P50, P90: c.P90})
	}
	handler := NewHandler(docRepo, NewMockAppointmentRepository(), statsRepo,
		&MockAuditRepository{}, &MockOrdinRepository{}, &MockOathRepository{})

	rec := doDocumentRequest(t, handler, "39946", "RD", "2024")

	var resp DocumentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Queue == nil || resp.Queue.EstimatedMonthsMin == nil ||
		*resp.Queue.EstimatedMonthsMin != 18 || *resp.Queue.EstimatedMonthsMax != 34 {
		t.Errorf("expected calibrated [18, 34] through the handler, got %+v", resp.Queue)
	}
}
