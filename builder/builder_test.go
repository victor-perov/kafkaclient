package builder

import (
	"testing"

	"github.com/mkocikowski/libkafka/compression"
)

type recordT struct { // implements Record
	k []byte
	v []byte
}

func (r *recordT) Key() []byte   { return r.k }
func (r *recordT) Value() []byte { return r.v }

//

func TestUnitBuilderStartStop(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  1,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{&recordT{nil, []byte("foo")}}
	b := <-batches
	if b.NumRecords != 1 {
		t.Fatal(b.NumRecords)
	}
	if b.UncompressedBytes != b.BatchLengthBytes {
		t.Fatal(b.UncompressedBytes, b.BatchLengthBytes)
	}
	if b.Partition != -1 { // NopPartitioner
		t.Fatal(b.Partition)
	}
	close(records)
	if _, ok := <-batches; ok {
		t.Fatal("expected output to be closed")
	}
}

// setting MinRecords=1 but calling Add with 2 records. expect 2 batches of 1 record
func TestUnitBuilderBigSet(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  1,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{
		&recordT{nil, []byte("foo")},
		&recordT{nil, []byte("bar")},
	}
	if b := <-batches; b.NumRecords != 1 {
		t.Fatal(b, b.NumRecords)
	}
	if b := <-batches; b.NumRecords != 1 {
		t.Fatal(b, b.NumRecords)
	}
}

// setting MinRecords=1 but calling Add with 2 records. expect 2 batches of 1 record
func TestUnitBuilderPartitioned(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  1,
		NumWorkers:  1,
		Partitioner: &HashPartitioner{numPartitions: 10},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{
		&recordT{[]byte("foo"), []byte("foo")},
	}
	if b := <-batches; b.Partition != 3 {
		t.Fatal(b, b.Partition)
	}
}

// setting MinRecords=1 but MinUncompressedBytes=4. then adding 2 separate payloads, but the first
// payload is only 3 bytes. expecting the 2 payloads to get combined
func TestUnitBuilderFlushOnBytes(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:           &compression.Nop{},
		MinRecords:           1,
		MinUncompressedBytes: 4,
		NumWorkers:           1,
		Partitioner:          &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{&recordT{nil, []byte("foo")}}
	records <- []Record{&recordT{nil, []byte("bar")}}
	if b := <-batches; b.NumRecords != 2 {
		t.Fatal(b, b.NumRecords)
	}
}

// setting MinRecords=2 but calling Add with 1 record. then closing the builder to "flush" expect a
// batch with only 1 record
func TestUnitBuilderSmallBatchFlush(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  2,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{&recordT{nil, []byte("foo")}}
	close(records)
	if b := <-batches; b.NumRecords != 1 {
		t.Fatal(b, b.NumRecords)
	}
}

func TestUnitBuilderEmptySets(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  1,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{}
	records <- []Record{}
	close(records)
	if b := <-batches; b != nil {
		t.Fatalf("%+v", b)
	}
}

// expect nil records to be skipped
func TestUnitBuilderNilRecords(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  1,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{}
	records <- []Record{nil, nil}
	close(records)
	if b := <-batches; b != nil {
		t.Fatalf("%+v", b)
	}
}

func TestUnitBuilderFlush(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  10,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	records <- []Record{
		&recordT{nil, []byte("foo")},
		&recordT{nil, []byte("bar")},
	}
	builder.Flush(0) // 0 flushes everything
	if b := <-batches; b.NumRecords != 2 {
		t.Fatal(b, b.NumRecords)
	}
}

/*
mik:batch$ go test . -bench=Builder -run=xxx -cpu=1,4,8
goos: linux
goarch: amd64
pkg: github.com/mkocikowski/kafkaclient/batch
BenchmarkBuilder           33391             30090 ns/op
BenchmarkBuilder-4         37789             33003 ns/op
BenchmarkBuilder-8         36254             34245 ns/op
PASS
ok      github.com/mkocikowski/kafkaclient/batch        4.523s
*/
func BenchmarkBuilder(b *testing.B) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  100,
		NumWorkers:  1,
		Partitioner: &NopPartitioner{},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	go func() {
		for _ = range batches {
		}
	}()
	r := []Record{}
	for i := 0; i < 100; i++ {
		r = append(r, &recordT{[]byte{uint8(i)}, []byte("foo")})
	}
	for i := 0; i < b.N; i++ {
		records <- r
	}
}

/*
mik:batch$ go test . -bench=BuilderPartitioned -run=xxx -cpu=1,4,8
goos: linux
goarch: amd64
pkg: github.com/mkocikowski/kafkaclient/batch
BenchmarkBuilderPartitioned                31688             35553 ns/op
BenchmarkBuilderPartitioned-4              33046             38427 ns/op
BenchmarkBuilderPartitioned-8              29829             39553 ns/op
PASS
ok      github.com/mkocikowski/kafkaclient/batch        4.745s
*/
func BenchmarkBuilderPartitioned(b *testing.B) {
	builder := &SequentialBuilder{
		Compressor:  &compression.Nop{},
		MinRecords:  100,
		NumWorkers:  1,
		Partitioner: &HashPartitioner{numPartitions: 100},
	}
	records := make(chan []Record)
	batches := builder.Start(records)
	go func() {
		for _ = range batches {
		}
	}()
	r := []Record{}
	for i := 0; i < 100; i++ {
		r = append(r, &recordT{[]byte{uint8(i)}, []byte("foo")})
	}
	for i := 0; i < b.N; i++ {
		records <- r
	}
}
