package cbor

import (
	"encoding/json"
	"io"
	"reflect"
)

type EncTagMode int
type DecTagMode int

const (
	EncTagRequired EncTagMode = 1
)

const (
	DecTagRequired DecTagMode = 1
)

type TagOptions struct {
	EncTag EncTagMode
	DecTag DecTagMode
}

type TagSet struct{}

func NewTagSet() TagSet { return TagSet{} }

func (TagSet) Add(_ TagOptions, _ reflect.Type, _ uint64) error { return nil }

type EncOptions struct{}
type DecOptions struct {
	MaxArrayElements int
	MaxMapPairs      int
}

func CoreDetEncOptions() EncOptions { return EncOptions{} }

type EncMode struct{}
type DecMode struct{}

func (EncOptions) EncModeWithTags(_ TagSet) (EncMode, error) { return EncMode{}, nil }
func (DecOptions) DecModeWithTags(_ TagSet) (DecMode, error) { return DecMode{}, nil }

type Encoder struct{ enc *json.Encoder }
type Decoder struct{ dec *json.Decoder }

func (m EncMode) NewEncoder(w io.Writer) *Encoder { return &Encoder{enc: json.NewEncoder(w)} }
func (m DecMode) NewDecoder(r io.Reader) *Decoder { return &Decoder{dec: json.NewDecoder(r)} }

func (e *Encoder) Encode(v any) error { return e.enc.Encode(v) }
func (d *Decoder) Decode(v any) error { return d.dec.Decode(v) }
