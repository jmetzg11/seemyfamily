package main

import (
	"seemyfamily.jmetzg11/internal/models"
)

const (
	chartWidth   = 720
	chartHeight  = 220
	chartTop     = 10
	chartBottom  = 40
	chartBarGap  = 4
	chartMinBarH = 1
)

type chartBar struct {
	X      int
	Y      int
	Width  int
	Height int
	Label  string
	Count  int
}

type chart struct {
	Width  int
	Height int
	LabelY int
	Max    int
	Bars   []chartBar
}

func buildChart(buckets []models.VisitorBucket, labelEvery int) chart {
	c := chart{
		Width:  chartWidth,
		Height: chartHeight,
		LabelY: chartHeight - chartBottom + 20,
	}

	if len(buckets) == 0 {
		return c
	}

	for _, b := range buckets {
		if b.Count > c.Max {
			c.Max = b.Count
		}
	}

	plot := chartHeight - chartTop - chartBottom
	slot := chartWidth / len(buckets)

	width := slot - chartBarGap
	if width < 1 {
		width = 1
	}

	for i, b := range buckets {
		height := 0
		if c.Max > 0 {
			height = b.Count * plot / c.Max
		}
		if height < chartMinBarH {
			height = chartMinBarH
		}

		label := ""
		if i%labelEvery == 0 {
			label = b.Start.Format("Jan 2")
		}

		c.Bars = append(c.Bars, chartBar{
			X:      i*slot + chartBarGap/2,
			Y:      chartTop + plot - height,
			Width:  width,
			Height: height,
			Label:  label,
			Count:  b.Count,
		})
	}

	return c
}
