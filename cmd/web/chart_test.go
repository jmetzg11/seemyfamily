package main

import (
	"testing"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

func buckets(start time.Time, counts ...int) []models.VisitorBucket {
	out := make([]models.VisitorBucket, len(counts))
	for i, count := range counts {
		out[i] = models.VisitorBucket{Start: start.AddDate(0, 0, i), Count: count}
	}
	return out
}

func TestBuildChartEmpty(t *testing.T) {
	c := buildChart(nil, 1)

	if c.Width != chartWidth || c.Height != chartHeight {
		t.Errorf("got %dx%d; want %dx%d", c.Width, c.Height, chartWidth, chartHeight)
	}
	if c.LabelY != chartHeight-chartBottom+20 {
		t.Errorf("got LabelY %d; want %d", c.LabelY, chartHeight-chartBottom+20)
	}
	if c.Max != 0 {
		t.Errorf("got Max %d; want 0", c.Max)
	}
	if len(c.Bars) != 0 {
		t.Errorf("got %d bars; want none — an empty range must not divide by zero", len(c.Bars))
	}
}

func TestBuildChartGeometry(t *testing.T) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	c := buildChart(buckets(start, 10, 5, 0, 10), 2)

	if c.Max != 10 {
		t.Fatalf("got Max %d; want 10", c.Max)
	}
	if len(c.Bars) != 4 {
		t.Fatalf("got %d bars; want 4", len(c.Bars))
	}

	want := []chartBar{
		{X: 2, Y: 10, Width: 176, Height: 170, Label: "Aug 1", Count: 10},
		{X: 182, Y: 95, Width: 176, Height: 85, Label: "", Count: 5},
		{X: 362, Y: 179, Width: 176, Height: 1, Label: "Aug 3", Count: 0},
		{X: 542, Y: 10, Width: 176, Height: 170, Label: "", Count: 10},
	}

	for i, w := range want {
		if c.Bars[i] != w {
			t.Errorf("bar %d = %+v; want %+v", i, c.Bars[i], w)
		}
	}
}

func TestBuildChartTallestBarFillsThePlot(t *testing.T) {
	c := buildChart(buckets(time.Now(), 3, 7), 1)

	plot := chartHeight - chartTop - chartBottom

	if c.Bars[1].Height != plot {
		t.Errorf("got height %d for the tallest bar; want the full plot %d", c.Bars[1].Height, plot)
	}
	if c.Bars[1].Y != chartTop {
		t.Errorf("got Y %d for the tallest bar; want %d", c.Bars[1].Y, chartTop)
	}
	if bottom := c.Bars[0].Y + c.Bars[0].Height; bottom != chartTop+plot {
		t.Errorf("got baseline %d; every bar sits on %d", bottom, chartTop+plot)
	}
}

func TestBuildChartNoVisitorsStillDrawsBars(t *testing.T) {
	c := buildChart(buckets(time.Now(), 0, 0, 0), 1)

	if c.Max != 0 {
		t.Fatalf("got Max %d; want 0", c.Max)
	}

	for i, b := range c.Bars {
		if b.Height != chartMinBarH {
			t.Errorf("bar %d has height %d; want the %dpx minimum so the axis stays visible", i, b.Height, chartMinBarH)
		}
	}
}

func TestBuildChartClampsNarrowBars(t *testing.T) {
	c := buildChart(buckets(time.Now(), make([]int, 400)...), 1)

	for i, b := range c.Bars {
		if b.Width != 1 {
			t.Fatalf("bar %d has width %d; want 1 — %d buckets across %dpx leaves no room for the gap", i, b.Width, len(c.Bars), chartWidth)
		}
	}
}

func TestBuildChartLabelsEveryNth(t *testing.T) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	c := buildChart(buckets(start, 1, 1, 1, 1, 1, 1, 1), 3)

	for i, b := range c.Bars {
		labelled := b.Label != ""
		if labelled != (i%3 == 0) {
			t.Errorf("bar %d label = %q; want labelled every 3rd", i, b.Label)
		}
	}
	if c.Bars[0].Label != "Aug 1" {
		t.Errorf("got label %q; want Aug 1", c.Bars[0].Label)
	}
}
