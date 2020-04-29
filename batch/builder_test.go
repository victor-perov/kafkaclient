package batch

import (
	"testing"

	"github.com/mkocikowski/libkafka"
	"github.com/mkocikowski/libkafka/batch"
	"github.com/mkocikowski/libkafka/compression"
	"github.com/mkocikowski/libkafka/record"
)

func TestUnitBuilderStartStop(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor: &compression.Nop{},
		MinRecords: 1,
		NumWorkers: 1,
	}
	records := make(chan []*libkafka.Record)
	batches := builder.Start(records)
	records <- []*libkafka.Record{record.New(nil, []byte("foo"))}
	b := <-batches
	if b.NumRecords != 1 {
		t.Fatal(b.NumRecords)
	}
	if b.UncompressedBytes != b.BatchLengthBytes {
		t.Fatal(b.UncompressedBytes, b.BatchLengthBytes)
	}
	close(records)
	if _, ok := <-batches; ok {
		t.Fatal("expected output to be closed")
	}
}

// setting MinRecords=1 but calling Add with 2 records. expect a single batch of 2 records
func TestUnitBuilderBigBatch(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor: &compression.Nop{},
		MinRecords: 1,
		NumWorkers: 1,
	}
	records := make(chan []*libkafka.Record)
	batches := builder.Start(records)
	records <- []*libkafka.Record{
		record.New(nil, []byte("foo")),
		record.New(nil, []byte("bar")),
	}
	if b := <-batches; b.NumRecords != 2 {
		t.Fatal(b, b.NumRecords)
	}
}

// setting MinRecords=2 but calling Add with 1 record. then closing the builder to "flush" expect a
// batch with only 1 record
func TestUnitBuilderSmallBatchFlush(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor: &compression.Nop{},
		MinRecords: 2,
		NumWorkers: 1,
	}
	records := make(chan []*libkafka.Record)
	batches := builder.Start(records)
	records <- []*libkafka.Record{record.New(nil, []byte("foo"))}
	close(records)
	if b := <-batches; b.NumRecords != 1 {
		t.Fatal(b, b.NumRecords)
	}
}

func TestUnitBuilderEmptySets(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor: &compression.Nop{},
		MinRecords: 1,
		NumWorkers: 1,
	}
	records := make(chan []*libkafka.Record)
	batches := builder.Start(records)
	records <- []*libkafka.Record{}
	records <- []*libkafka.Record{}
	close(records)
	if b := <-batches; b != nil {
		t.Fatalf("%+v", b)
	}
}

func TestUnitBuilderNilRecords(t *testing.T) {
	builder := &SequentialBuilder{
		Compressor: &compression.Nop{},
		MinRecords: 1,
		NumWorkers: 1,
	}
	records := make(chan []*libkafka.Record)
	batches := builder.Start(records)
	records <- []*libkafka.Record{}
	records <- []*libkafka.Record{nil, nil}
	close(records)
	if b := <-batches; b.BuildError != batch.ErrNilRecord {
		t.Fatal(b.BuildError)
	}
}
