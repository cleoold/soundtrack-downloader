package main

import (
	"reflect"
	"testing"
)

func TestTagFlags(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		s := tagFlags{}
		if err := s.Set("ALBUM=My Album"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := s.Set("date=2021"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := s.Set("ALBUMARTIST=My Artist "); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := s.Set("ARTIST=My Artist"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := s.Set("Genre=myType"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := tagFlags{
			"ALBUM":       "My Album",
			"DATE":        "2021",
			"ALBUMARTIST": "My Artist ",
			"ARTIST":      "My Artist",
			"GENRE":       "myType",
		}
		if !reflect.DeepEqual(s, expected) {
			t.Fatalf("expected %v, got %v", expected, s)
		}
	})

	t.Run("Invalid tag format", func(t *testing.T) {
		s := tagFlags{}
		if err := s.Set("ALBUM"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestOverrideFlags(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		s := overrideFlags{}
		if err := s.Set("ALBUM,artist,TRACKNUMBER"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		vacuum := struct{}{}
		expected := overrideFlags{"ALBUM": vacuum, "ARTIST": vacuum, "TRACKNUMBER": vacuum}
		if !reflect.DeepEqual(s, expected) {
			t.Fatalf("expected %v, got %v", expected, s)
		}
	})
}
