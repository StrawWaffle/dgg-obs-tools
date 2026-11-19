package settings

import (
	"testing"

	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
)

func TestValidateKeepsYouTubeVideoCase(t *testing.T) {
	s := Default()
	s.Channels = []dgg.Channel{
		{Platform: "kick", ID: "Destiny"},
		{Platform: "kick", ID: "destiny"},
		{Platform: "youtube", ID: "P7Xv5DdqL0k"},
		{Platform: "youtube", ID: "@Destiny"},
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []dgg.Channel{
		{Platform: "kick", ID: "destiny"},
		{Platform: "youtube", ID: "P7Xv5DdqL0k"},
		{Platform: "youtube", ID: "@destiny"},
	}
	if len(s.Channels) != len(want) {
		t.Fatalf("got %v", s.Channels)
	}
	for i := range want {
		if s.Channels[i] != want[i] {
			t.Fatalf("got %v", s.Channels)
		}
	}
}

func TestValidateRefusesAYouTubeNameThatIsNeither(t *testing.T) {
	s := Default()
	s.Channels = []dgg.Channel{{Platform: "youtube", ID: "destiny"}}
	if s.Validate() == nil {
		t.Fatal("accepted a bare name for YouTube")
	}
}
