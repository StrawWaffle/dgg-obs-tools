package chat

import (
	"reflect"
	"testing"
)

func TestTextPartsFindsLinks(t *testing.T) {
	got := TextParts("see https://example.com/a?b=1, then")
	want := []Part{{Text: "see "}, {Link: "https://example.com/a?b=1"}, {Text: ", then"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestWordPartsFindsEmotesAsWholeWords(t *testing.T) {
	emotes := map[string]bool{"PEPE": true, "LUL": true}
	got := WordParts("PEPE hi PEPEs LUL https://x.y", func(word string) bool { return emotes[word] })
	want := []Part{
		{Emote: "PEPE"},
		{Text: " hi PEPEs "},
		{Emote: "LUL"},
		{Text: " "},
		{Link: "https://x.y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
