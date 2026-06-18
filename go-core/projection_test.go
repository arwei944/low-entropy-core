//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

func TestProjection_FullRebuild(t *testing.T) {
	handler := func(state []byte, event EventEnvelope) ([]byte, error) {
		if state == nil {
			return event.EventData, nil
		}
		return append(state, event.EventData...), nil
	}

	proj := NewProjection(handler)

	input := ProjectionInput{
		AggregateID: "agg-1",
		Events: []EventEnvelope{
			{EventType: "A", EventData: []byte("hello"), Version: 1},
			{EventType: "B", EventData: []byte(" world"), Version: 2},
		},
		FromVersion:  0,
		CurrentState: nil,
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output.State) != "hello world" {
		t.Errorf("expected 'hello world', got %q", output.State)
	}
	if output.Version != 2 {
		t.Errorf("expected version 2, got %d", output.Version)
	}
	if output.EventsProcessed != 2 {
		t.Errorf("expected 2 events, got %d", output.EventsProcessed)
	}
}

func TestProjection_Incremental(t *testing.T) {
	handler := func(state []byte, event EventEnvelope) ([]byte, error) {
		if state == nil {
			return event.EventData, nil
		}
		return append(state, event.EventData...), nil
	}

	proj := NewProjection(handler)

	input := ProjectionInput{
		AggregateID: "agg-2",
		Events: []EventEnvelope{
			{EventType: "A", EventData: []byte("A"), Version: 1},
			{EventType: "B", EventData: []byte("B"), Version: 2},
			{EventType: "C", EventData: []byte("C"), Version: 3},
		},
		FromVersion:  2,
		CurrentState: []byte("A"),
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output.State) != "ABC" {
		t.Errorf("expected 'ABC', got %q", output.State)
	}
	if output.Version != 3 {
		t.Errorf("expected version 3, got %d", output.Version)
	}
	if output.EventsProcessed != 2 {
		t.Errorf("expected 2 events, got %d", output.EventsProcessed)
	}
}

func TestProjection_EmptyEvents(t *testing.T) {
	proj := NewProjection(func(state []byte, event EventEnvelope) ([]byte, error) {
		return state, nil
	})

	input := ProjectionInput{
		AggregateID:  "agg-3",
		Events:       nil,
		FromVersion:  0,
		CurrentState: []byte("existing"),
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output.State) != "existing" {
		t.Errorf("expected 'existing', got %q", output.State)
	}
	if output.Version != 0 {
		t.Errorf("expected version 0, got %d", output.Version)
	}
	if output.EventsProcessed != 0 {
		t.Errorf("expected 0 events, got %d", output.EventsProcessed)
	}
}

func TestProjection_AggregateID(t *testing.T) {
	proj := NewProjection(func(state []byte, event EventEnvelope) ([]byte, error) {
		return []byte("done"), nil
	})

	input := ProjectionInput{
		AggregateID: "my-aggregate",
		Events: []EventEnvelope{
			{EventType: "Created", Version: 1},
		},
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.AggregateID != "my-aggregate" {
		t.Errorf("expected 'my-aggregate', got %s", output.AggregateID)
	}
	if output.Version != 1 {
		t.Errorf("expected version 1, got %d", output.Version)
	}
	if output.EventsProcessed != 1 {
		t.Errorf("expected 1 event, got %d", output.EventsProcessed)
	}
}