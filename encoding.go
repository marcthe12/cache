package cache

import (
	"bufio"
	"encoding/binary"
	"io"
	"time"
)

type encoder struct {
	w   *bufio.Writer
	buf []byte
}

func newEncoder(w io.Writer) *encoder {
	return &encoder{
		w:   bufio.NewWriter(w),
		buf: make([]byte, 8),
	}
}

func (e *encoder) Flush() error {
	return e.w.Flush()
}

func (e *encoder) EncodeUint64(val uint64) error {
	binary.LittleEndian.PutUint64(e.buf, val)
	_, err := e.w.Write(e.buf)
	return err
}

func (e *encoder) EncodeTime(val time.Time) error {
	return e.EncodeUint64(uint64(val.Unix()))
}

func (e *encoder) EncodeBytes(val []byte) error {
	if err := e.EncodeUint64(uint64(len(val))); err != nil {
		return err
	}

	_, err := e.w.Write(val)
	return err
}

func (e *encoder) EncodeNode(n *node) error {
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

func (e *encoder) EncodeStore(s *store) error {
	if err := e.EncodeUint64(s.MaxCost); err != nil {
		return err
	}

	if err := e.EncodeUint64(uint64(s.Policy.Type)); err != nil {
		return err
	}

	if err := e.EncodeUint64(s.Length); err != nil {
		return err
	}

	for v := s.Evict.EvictNext; v != &s.Evict; v = v.EvictNext {
		if err := e.EncodeNode(v); err != nil {
			return err
		}
	}
	return nil
}

type decoder struct {
	r   *bufio.Reader
	buf []byte
}

func newDecoder(r io.Reader) *decoder {
	return &decoder{
		r:   bufio.NewReader(r),
		buf: make([]byte, 8),
	}
}

func (d *decoder) DecodeUint64() (uint64, error) {
	_, err := io.ReadFull(d.r, d.buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(d.buf), nil
}

func (d *decoder) DecodeTime() (time.Time, error) {
	ts, err := d.DecodeUint64()
	if err != nil {
		return zero[time.Time](), err
	}
	return time.Unix(int64(ts), 0), nil
}

func (d *decoder) DecodeBytes() ([]byte, error) {
	lenVal, err := d.DecodeUint64()
	if err != nil {
		return nil, err
	}
	data := make([]byte, lenVal)
	_, err = io.ReadFull(d.r, data)
	return data, err
}

func (d *decoder) DecodeNodes() (*node, error) {
	n := &node{}

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

func (d *decoder) DecodeStore(s *store) error {
	maxCost, err := d.DecodeUint64()
	if err != nil {
		return err
	}
	s.MaxCost = maxCost

	policy, err := d.DecodeUint64()
	if err != nil {
		return err
	}
	s.Policy.SetPolicy(EvictionPolicyType(policy))

	length, err := d.DecodeUint64()
	if err != nil {
		return err
	}
	s.Length = length

	k := 128
	for k < int(s.Length) {
		k = k << 1
	}

	s.Bucket = make([]node, k)
	for range s.Length {
		v, err := d.DecodeNodes()
		if err != nil {
			return err
		}

		idx := v.Hash % uint64(len(s.Bucket))

		bucket := &s.Bucket[idx]
		lazyInitBucket(bucket)

		v.HashPrev = bucket
		v.HashNext = v.HashPrev.HashNext
		v.HashNext.HashPrev = v
		v.HashPrev.HashNext = v

		v.EvictNext = &s.Evict
		v.EvictPrev = v.EvictNext.EvictPrev
		v.EvictNext.EvictPrev = v
		v.EvictPrev.EvictNext = v

		s.Cost = s.Cost + uint64(len(v.Key)) + uint64(len(v.Value))
	}
	return nil
}

func (s *store) Snapshot(w io.WriteSeeker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := w.Seek(0, io.SeekStart); err != nil {
		return err
	}

	wr := newEncoder(w)
	defer wr.Flush()

	return wr.EncodeStore(s)
}

func (s *store) LoadSnapshot(r io.ReadSeeker) error {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	d := newDecoder(r)

	return d.DecodeStore(s)
}
