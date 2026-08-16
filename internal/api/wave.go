package api

import (
	"math"
	"sort"
	"time"
)

const (
	waveMinCohortSize   = 100
	waveCensorShareMax  = 0.5
	wavePhiIncrement    = 0.08
	wavePhiShareMin     = 0.5
	wavePhiMin          = 0.70
	wavePhiMax          = 0.95
	wavePhiDefault      = 0.85
	waveRateWindowMonth = 12.0
	waveRangeLow        = 0.8
	waveRangeHigh       = 1.3

	waveSpanMinCell      = 300
	waveSpanP90          = 0.95
	waveSpanRatioMax     = 1.6
	waveSpanRecentK      = 3
	waveSpanRatioDefault = 1.15
	waveCurveMinCell     = 50
	waveCurveCellShare   = 0.05
	waveCurveMinPoints   = 3
	waveRangeHighV21     = 1.5
	waveSpanMinAgeYears  = 3
)

type wavePoint struct {
	age   float64
	share float64
}

// nowFraction converts a time to a decimal year
func nowFraction(now time.Time) float64 {
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
	return float64(now.Year()) + now.Sub(yearStart).Seconds()/yearEnd.Sub(yearStart).Seconds()
}

// censoredYears finds registration years whose solutions predate the published
// archives: older than the best-covered reference year yet under 50% solved
func censoredYears(years []YearStatsResponse) map[int]bool {
	maxShare, maxShareYear := -1.0, 0
	for _, y := range years {
		if y.Total < waveMinCohortSize {
			continue
		}
		share := float64(y.Solved) / float64(y.Total)
		if share > maxShare {
			maxShare, maxShareYear = share, y.Year
		}
	}
	censored := map[int]bool{}
	if maxShareYear == 0 {
		return censored
	}
	for _, y := range years {
		if y.Total == 0 {
			continue
		}
		share := float64(y.Solved) / float64(y.Total)
		if y.Year < maxShareYear && share < waveCensorShareMax {
			censored[y.Year] = true
		}
	}
	return censored
}

// maturationEnvelope builds the monotone (age, share) envelope of the
// cross-sectional maturation curve, youngest cohorts first
func maturationEnvelope(years []YearStatsResponse, censored map[int]bool, nowFrac float64) []wavePoint {
	var pts []wavePoint
	for _, y := range years {
		if y.Total < waveMinCohortSize || censored[y.Year] {
			continue
		}
		pts = append(pts, wavePoint{
			age:   nowFrac - (float64(y.Year) + 0.5),
			share: float64(y.Solved) / float64(y.Total),
		})
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].age < pts[j].age })
	running := 0.0
	for i := range pts {
		if pts[i].share > running {
			running = pts[i].share
		}
		pts[i].share = running
	}
	return pts
}

// waveCoverage estimates phi, the share of a cohort the bulk wave covers:
// the plateau onset of the maturation envelope
func waveCoverage(env []wavePoint) float64 {
	for i := 0; i < len(env)-1; i++ {
		s := env[i].share
		if s >= wavePhiShareMin && env[i+1].share-s < wavePhiIncrement {
			return math.Max(wavePhiMin, math.Min(wavePhiMax, s))
		}
	}
	return wavePhiDefault
}

// envelopeAgeAt returns the age at which the envelope reaches the target share
func envelopeAgeAt(env []wavePoint, target float64) (float64, bool) {
	if len(env) == 0 || target > env[len(env)-1].share {
		return 0, false
	}
	prev := wavePoint{age: env[0].age, share: 0}
	for _, p := range env {
		if p.share >= target {
			if p.share == prev.share {
				return p.age, true
			}
			return prev.age + (p.age-prev.age)*(target-prev.share)/(p.share-prev.share), true
		}
		prev = p
	}
	return 0, false
}

// filterCells drops cells with implausible solution years (live data contains
// typo years like 8202) or a solution predating the registration
func filterCells(cells []CohortCellResponse, now time.Time) []CohortCellResponse {
	var out []CohortCellResponse
	for _, c := range cells {
		if c.SolYear >= 2001 && c.SolYear <= now.Year()+1 && c.SolYear >= c.RegYear {
			out = append(out, c)
		}
	}
	return out
}

// cohortSpans estimates each cohort's real document-number span from the
// matrix p90s; returns valid spans and the median span/total ratio of the
// most recent cohorts that have one. Cohorts younger than
// waveSpanMinAgeYears are skipped entirely: their wave hasn't reached the
// high numbers yet, so the span would systematically underestimate.
func cohortSpans(years []YearStatsResponse, censored map[int]bool, cells []CohortCellResponse, now time.Time) (map[int]float64, float64) {
	totals := map[int]int{}
	for _, y := range years {
		totals[y.Year] = y.Total
	}

	maxP90 := map[int]int{}
	for _, c := range cells {
		if c.Count < waveSpanMinCell {
			continue
		}
		if censored[c.RegYear] || totals[c.RegYear] < waveMinCohortSize {
			continue
		}
		if c.RegYear > now.Year()-waveSpanMinAgeYears {
			continue
		}
		if c.P90 > maxP90[c.RegYear] {
			maxP90[c.RegYear] = c.P90
		}
	}

	spans := map[int]float64{}
	var validYears []int
	ratios := map[int]float64{}
	for year, p90 := range maxP90 {
		span := float64(p90) / waveSpanP90
		ratio := span / float64(totals[year])
		if ratio >= 1.0 && ratio <= waveSpanRatioMax {
			spans[year] = span
			ratios[year] = ratio
			validYears = append(validYears, year)
		}
	}

	ratioMedian := waveSpanRatioDefault
	if len(validYears) > 0 {
		sort.Sort(sort.Reverse(sort.IntSlice(validYears)))
		recent := validYears
		if len(recent) > waveSpanRecentK {
			recent = recent[:waveSpanRecentK]
		}
		vals := make([]float64, 0, len(recent))
		for _, y := range recent {
			vals = append(vals, ratios[y])
		}
		sort.Float64s(vals)
		ratioMedian = vals[len(vals)/2]
	}

	return spans, ratioMedian
}

// empiricalCurve pools span-corrected wave-phase points (q of a cell's p50,
// cohort age at the cell's solution year) from cohorts that show at least
// two monotone wave points; the running maximum keeps the pooled curve
// conservative across cohorts
func empiricalCurve(years []YearStatsResponse, censored map[int]bool, cells []CohortCellResponse, spans map[int]float64, ratioMedian float64) []wavePoint {
	totals := map[int]int{}
	for _, y := range years {
		totals[y.Year] = y.Total
	}

	byReg := map[int][]CohortCellResponse{}
	for _, c := range cells {
		byReg[c.RegYear] = append(byReg[c.RegYear], c)
	}

	var pooled []wavePoint
	for reg, regCells := range byReg {
		total := totals[reg]
		if censored[reg] || total < waveMinCohortSize {
			continue
		}
		span, ok := spans[reg]
		if !ok {
			span = float64(total) * ratioMedian
		}

		sort.Slice(regCells, func(i, j int) bool { return regCells[i].SolYear < regCells[j].SolYear })
		threshold := waveCurveCellShare * float64(total)
		if threshold < waveCurveMinCell {
			threshold = waveCurveMinCell
		}

		var pts []wavePoint
		prevQ := -1.0
		for _, c := range regCells {
			if float64(c.Count) < threshold {
				continue
			}
			q := float64(c.P50) / span
			if q <= prevQ {
				break
			}
			pts = append(pts, wavePoint{age: float64(c.SolYear - reg), share: q})
			prevQ = q
		}
		if len(pts) >= 2 {
			pooled = append(pooled, pts...)
		}
	}

	sort.Slice(pooled, func(i, j int) bool {
		if pooled[i].share != pooled[j].share {
			return pooled[i].share < pooled[j].share
		}
		return pooled[i].age < pooled[j].age
	})
	running := 0.0
	for i := range pooled {
		if pooled[i].age > running {
			running = pooled[i].age
		}
		pooled[i].age = running
	}
	return pooled
}

// curveAgeAt interpolates the empirical wave curve at position q; outside
// the observed range it clamps to the boundary ages (the exception tail is
// not number-ordered — extrapolating the wave line would be false precision)
func curveAgeAt(curve []wavePoint, q float64) float64 {
	if q <= curve[0].share {
		return curve[0].age
	}
	for i := 1; i < len(curve); i++ {
		if curve[i].share >= q {
			q0, a0 := curve[i-1].share, curve[i-1].age
			q1, a1 := curve[i].share, curve[i].age
			if q1 == q0 {
				return a1
			}
			return a0 + (a1-a0)*(q-q0)/(q1-q0)
		}
	}
	return curve[len(curve)-1].age
}

// buildWaveEstimate computes the wave-model queue estimate for an unsolved
// document from the category's cached stats, optionally calibrated by the
// cohort matrix cells for the document's category. Returns nil when the
// document's cohort is absent from the stats.
func buildWaveEstimate(cat CategoryStatsResponse, cells []CohortCellResponse, docYear, docNumber int, now time.Time) *QueueResponse {
	var cohort *YearStatsResponse
	for i := range cat.Years {
		if cat.Years[i].Year == docYear {
			cohort = &cat.Years[i]
			break
		}
	}
	if cohort == nil || cohort.Total == 0 {
		return nil
	}

	totalEff := float64(cohort.Total)
	nowFrac := nowFraction(now)
	if docYear == now.Year() {
		elapsed := nowFrac - float64(now.Year())
		if elapsed < 0.25 {
			elapsed = 0.25
		}
		totalEff = float64(cohort.Total) / elapsed
	}

	cells = filterCells(cells, now)
	censored := censoredYears(cat.Years)

	span := totalEff
	var spans map[int]float64
	var ratioMedian float64
	if len(cells) > 0 {
		spans, ratioMedian = cohortSpans(cat.Years, censored, cells, now)
		if s, ok := spans[docYear]; ok {
			span = s
		} else {
			span = totalEff * ratioMedian
		}
	}

	q := float64(docNumber) / span
	if q > 1 {
		q = 1
	}
	resp := &QueueResponse{
		CohortTotal: cohort.Total,
		Percentile:  math.Round(q*100) / 100,
	}

	if cohort.Total < waveMinCohortSize {
		return resp
	}

	if censored[docYear] {
		resp.WavePassed = true
		return resp
	}
	env := maturationEnvelope(cat.Years, censored, nowFrac)
	if len(env) < 3 {
		return resp
	}

	phi := waveCoverage(env)
	target := q * phi

	cohortShare := float64(cohort.Solved) / float64(cohort.Total)
	if cohortShare >= target {
		resp.WavePassed = true
		return resp
	}

	var curve []wavePoint
	if len(cells) > 0 {
		curve = empiricalCurve(cat.Years, censored, cells, spans, ratioMedian)
	}

	var aStar float64
	usedEmpirical := false
	if len(curve) >= waveCurveMinPoints {
		aStar = curveAgeAt(curve, q)
		usedEmpirical = true
	} else {
		var ok bool
		aStar, ok = envelopeAgeAt(env, target)
		if !ok {
			return resp
		}
	}

	t := math.Max(0, (float64(docYear)+0.5+aStar)-nowFrac) * 12

	r90 := 0
	for _, a := range cat.RecentActivity {
		if a.Year == docYear {
			r90 = a.Solved
			break
		}
	}
	rateFloor := waveRateWindowMonth
	if r90 > 0 {
		tRate := (target - cohortShare) * float64(cohort.Total) / (float64(r90) / 3.0)
		rateFloor = math.Min(tRate, waveRateWindowMonth)
	}
	if rateFloor > t {
		t = rateFloor
	}

	high := waveRangeHigh
	if usedEmpirical {
		high = waveRangeHighV21
	}
	minM := int(math.Max(1, math.Floor(waveRangeLow*t)))
	maxM := int(math.Ceil(high * t))
	if maxM <= minM {
		maxM = minM + 1
	}
	resp.EstimatedMonthsMin = &minM
	resp.EstimatedMonthsMax = &maxM
	return resp
}
