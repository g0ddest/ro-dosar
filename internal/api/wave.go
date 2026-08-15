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

// buildWaveEstimate computes the wave-model queue estimate for an unsolved
// document from the category's cached stats alone. Returns nil when the
// document's cohort is absent from the stats.
func buildWaveEstimate(cat CategoryStatsResponse, docYear, docNumber int, now time.Time) *QueueResponse {
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

	q := float64(docNumber) / float64(cohort.Total)
	if q > 1 {
		q = 1
	}
	resp := &QueueResponse{
		CohortTotal: cohort.Total,
		Percentile:  math.Round(q*100) / 100,
	}

	nowFrac := nowFraction(now)
	censored := censoredYears(cat.Years)
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

	aStar, ok := envelopeAgeAt(env, target)
	if !ok {
		return resp
	}

	t := math.Max(0, (float64(docYear)+0.5+aStar)-nowFrac) * 12
	if t <= waveRateWindowMonth {
		r90 := 0
		for _, a := range cat.RecentActivity {
			if a.Year == docYear {
				r90 = a.Solved
				break
			}
		}
		if r90 > 0 {
			tRate := (target - cohortShare) * float64(cohort.Total) / (float64(r90) / 3.0)
			if tRate > t {
				t = tRate
			}
		}
	}

	minM := int(math.Max(1, math.Floor(waveRangeLow*t)))
	maxM := int(math.Ceil(waveRangeHigh * t))
	if maxM <= minM {
		maxM = minM + 1
	}
	resp.EstimatedMonthsMin = &minM
	resp.EstimatedMonthsMax = &maxM
	return resp
}
