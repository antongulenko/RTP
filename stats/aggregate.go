package stats

import (
	"bytes"
	"sort"
)

type namedResults struct {
	*Results
	name string
}

type AggregatedStats []namedResults

func (agg AggregatedStats) Len() int {
	return len(agg)
}
func (agg AggregatedStats) Less(i, j int) bool {
	return agg[i].name < agg[j].name
}
func (agg AggregatedStats) Swap(i, j int) {
	agg[i], agg[j] = agg[j], agg[i]
}

func (agg *AggregatedStats) Results(name string) *Results {
	for _, res := range *agg {
		if res.name == name {
			return res.Results
		}
	}
	results := NewResults()
	*agg = append(*agg, namedResults{results, name})
	sort.Sort(*agg)
	return results
}

func (agg *AggregatedStats) Aggregate(stats *Stats) {
	stats.Results = agg.Results(stats.Name)
}

func (agg AggregatedStats) String() string {
	var buf bytes.Buffer
	for _, stats := range agg {
		buf.WriteString(stats.name)
		buf.WriteString(": ")
		buf.WriteString(stats.String())
		buf.WriteString("\n")
	}
	return buf.String()
}

func (agg AggregatedStats) Flush(seconds uint) {
	for _, stats := range agg {
		stats.Flush(seconds)
	}
}

func (agg AggregatedStats) Start() {
	for _, res := range agg {
		res.start()
	}
}

func (agg AggregatedStats) Stop() {
	for _, res := range agg {
		res.stop()
	}
}
