package profile

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
)

type ValueType struct {
	Type string
	Unit string
}

type Function struct {
	ID         uint64
	Name       string
	SystemName string
	Filename   string
	StartLine  int64
}

type Mapping struct {
	ID      uint64
	File    string
	BuildID string
}

type Line struct {
	Function *Function
	Line     int64
	Column   int64
}

type Location struct {
	ID      uint64
	Address uint64
	Line    []Line
	Mapping *Mapping
}

type Sample struct {
	Location []*Location
	Value    []int64
	Label    map[string][]string
	NumLabel map[string][]int64
	NumUnit  map[string][]string
}

func (s *Sample) DiffBaseSample() bool {
	if s == nil || s.Label == nil {
		return false
	}
	_, ok := s.Label["pprof::base"]
	return ok
}

type Profile struct {
	SampleType        []*ValueType
	PeriodType        *ValueType
	Sample            []*Sample
	Mapping           []*Mapping
	Function          []*Function
	Location          []*Location
	Comments          []string
	TimeNanos         int64
	DurationNanos     int64
	Period            int64
	DefaultSampleType string
}

func (p *Profile) RemoveLabel(name string) {
	if p == nil {
		return
	}
	for _, s := range p.Sample {
		if s == nil || s.Label == nil {
			continue
		}
		delete(s.Label, name)
	}
}

func (p *Profile) ScaleN(ratios []float64) error {
	if p == nil {
		return nil
	}
	for _, s := range p.Sample {
		if s == nil {
			continue
		}
		if len(ratios) == 0 {
			continue
		}
		if len(s.Value) < len(ratios) {
			continue
		}
		for i, ratio := range ratios {
			if i >= len(s.Value) {
				break
			}
			s.Value[i] = int64(math.Round(float64(s.Value[i]) * ratio))
		}
	}
	return nil
}

func (p *Profile) Write(w io.Writer) error {
	if p == nil {
		_, err := w.Write([]byte("{}"))
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func (p *Profile) String() string {
	var buf bytes.Buffer
	_ = p.Write(&buf)
	return buf.String()
}
