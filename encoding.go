package cache

import (
	"bufio"
	"encoding/binary"
	"io"
	"time"
)

type Encoder struct {
	*bufio.Writer
	buf []byte
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		Writer: bufio.NewWriter(w),
		buf:    make([]byte, 8),
	}
}

func (e *Encoder) EncodeUint64(val uint64) error {
	binary.LittleEndian.PutUint64(e.buf, val)
	_, err := e.Write(e.buf)
	return err
}

func (e *Encoder) EncodeTime(val time.Time) error {
	return e.EncodeUint64(uint64(val.Unix()))
}

func (e *Encoder) EncodeBytes(val []byte) error {
	if err := e.EncodeUint64(uint64(len(val))); err != nil {
		return err
	}

	_, err := e.Write(val)
	return err
}

func (e *Encoder) EncodeNode(n *Node) error {
	if err := e.EncodeUint64(n.Hash); err != nil {
		return err
	}

	if err := e.EncodeTime(n.Expiration); err != nil {
		return err
	}

	if err := e.EncodeUint64(n.Access); err != nil {
		return err
	}

	if err := e.EncodeBytes(n.Key); err != nil {
		return err
	}

	if err := e.EncodeBytes(n.Value); err != nil {
		return err
	}

	return nil
}

type Decoder struct {
	*bufio.Reader
	buf []byte
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		Reader: bufio.NewReader(r),
		buf:    make([]byte, 8),
	}
}

func (d *Decoder) DecodeUint64() (uint64, error) {
	_, err := io.ReadFull(d, d.buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(d.buf), nil
}

func (d *Decoder) DecodeTime() (time.Time, error) {
	ts, err := d.DecodeUint64()
	if err != nil {
		return zero[time.Time](), err
	}
	return time.Unix(int64(ts), 0), nil
}

func (d *Decoder) DecodeBytes() ([]byte, error) {
	lenVal, err := d.DecodeUint64()
	if err != nil {
		return nil, err
	}
	data := make([]byte, lenVal)
	_, err = io.ReadFull(d, data)
	return data, err
}

func (d *Decoder) DecodeNodes() (*Node, error) {
	n := &Node{}

	hash, err := d.DecodeUint64()
	if err != nil {
		return nil, err
	}
	n.Hash = hash

	expiration, err := d.DecodeTime()
	if err != nil {
		return nil, err
	}
	n.Expiration = expiration

	access, err := d.DecodeUint64()
	if err != nil {
		return nil, err
	}
	n.Access = access

	n.Key, err = d.DecodeBytes()
	if err != nil {
		return nil, err
	}

	n.Value, err = d.DecodeBytes()
	if err != nil {
		return nil, err
	}
	return n, err
}

func (s *Store) Snapshot(w io.WriteSeeker) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Seek(0, io.SeekStart)
	wr := NewEncoder(w)

	wr.EncodeUint64(s.lenght)

	for v := s.evict.EvictNext; v != &s.evict; v = v.EvictNext {
		if err := wr.EncodeNode(v); err != nil {
			return err
		}
	}
	wr.Flush()
	return nil
}

func (s *Store) LoadSnapshot(r io.ReadSeeker) error {
	r.Seek(0, io.SeekStart)
	rr := NewDecoder(r)

	lenght, err := rr.DecodeUint64()
	if err != nil {
		return err
	}
	s.lenght = lenght

	k := 128
	for k < int(s.lenght) {
		k = k << 1
	}

	s.bucket = make([]Node, k)
	for range s.lenght {
		v, err := rr.DecodeNodes()
		if err != nil {
			return err
		}

		idx := v.Hash % uint64(len(s.bucket))

		bucket := &s.bucket[idx]
		lazyInitBucket(bucket)

		v.HashPrev = bucket
		v.HashNext = v.HashPrev.HashNext
		v.HashNext.HashPrev = v
		v.HashPrev.HashNext = v

		v.EvictPrev = &s.evict
		v.EvictNext = v.EvictPrev.EvictNext
		v.EvictNext.EvictPrev = v
		v.EvictPrev.EvictNext = v

		s.cost = s.cost + uint64(len(v.Key)) + uint64(len(v.Value))
	}
	return nil
}
