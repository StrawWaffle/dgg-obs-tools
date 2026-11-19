package settings

import (
	"slices"
	"testing"

	"github.com/StrawWaffle/dgg-obs-tools/internal/dgg"
)

func TestValidateTidiesEachList(t *testing.T) {
	s := Default()
	s.Embeds = []dgg.Channel{
		{Platform: "kick", ID: "Destiny"},
		{Platform: "kick", ID: "destiny"},
		{Platform: "youtube", ID: "P7Xv5DdqL0k"},
		{Platform: "youtube", ID: "@Destiny"},
	}
	s.KickChats = []string{"Destiny", "destiny", "anythingelse"}
	s.YouTubeChats = []string{"@Destiny", "P7Xv5DdqL0k"}
	s.TwitchChats = []string{"xQc", "xqc"}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	wantEmbeds := []dgg.Channel{
		{Platform: "kick", ID: "destiny"},
		{Platform: "youtube", ID: "P7Xv5DdqL0k"},
		{Platform: "youtube", ID: "@destiny"},
	}
	if !slices.Equal(s.Embeds, wantEmbeds) {
		t.Fatalf("embeds %v", s.Embeds)
	}
	if !slices.Equal(s.KickChats, []string{"destiny", "anythingelse"}) {
		t.Fatalf("kick %v", s.KickChats)
	}
	if !slices.Equal(s.TwitchChats, []string{"xqc"}) {
		t.Fatalf("twitch %v", s.TwitchChats)
	}
	// A video id keeps its case, because YouTube's ids are case sensitive.
	if !slices.Equal(s.YouTubeChats, []string{"@destiny", "P7Xv5DdqL0k"}) {
		t.Fatalf("youtube %v", s.YouTubeChats)
	}
}

func TestValidateRefusesWhatItCannotPlace(t *testing.T) {
	for _, mutate := range []func(*Settings){
		func(s *Settings) { s.Embeds = []dgg.Channel{{Platform: "youtube", ID: "destiny"}} },
		func(s *Settings) { s.KickChats = []string{"kick.com/destiny"} },
		func(s *Settings) { s.YouTubeChats = []string{"destiny"} },
	} {
		s := Default()
		mutate(&s)
		if s.Validate() == nil {
			t.Errorf("accepted %+v", s)
		}
	}
}
