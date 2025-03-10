package main

import (
	"reflect"
	"testing"
)

func TestTrackFlags(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		s := trackFlags{}
		if err := s.Set("1-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := s.Set("1-2,1-3, 12-01, 12-134,13-*"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := s.Set("3"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		vacuum := struct{}{}
		expected := trackFlags{
			{DiscNumber: "1", TrackNumber: "1"}:    vacuum,
			{DiscNumber: "1", TrackNumber: "2"}:    vacuum,
			{DiscNumber: "1", TrackNumber: "3"}:    vacuum,
			{DiscNumber: "12", TrackNumber: "1"}:   vacuum,
			{DiscNumber: "12", TrackNumber: "134"}: vacuum,
			{DiscNumber: "13", TrackNumber: "*"}:   vacuum,
			{TrackNumber: "3"}:                     vacuum,
		}
		if !reflect.DeepEqual(s, expected) {
			t.Fatalf("expected %v, got %v", expected, s)
		}
	})

	t.Run("Invalid track number format", func(t *testing.T) {
		s := trackFlags{}
		if err := s.Set("1-1-1"); err == nil {
			t.Fatalf("expected error")
		}
	})
}
