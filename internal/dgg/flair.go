package dgg

// colouredFlairs is the source order of https://cdn.destiny.gg/flairs/flairs.css.
// Every rule there is `.user.<flair>` at one specificity, so the colour a name
// gets is the last matching rule in the file, not the first. Reversing this or
// stopping at the first match miscolours anyone holding more than one flair.
//
// Flairs with no colour rule of their own are left out on purpose. A colour
// added to web/src/watchers/flairs.css needs its place here too.
var colouredFlairs = []string{
	"moderator",
	"flair125",
	"flair11",
	"bot",
	"subscriber",
	"flair9",
	"flair13",
	"flair32",
	"flair1",
	"flair22",
	"flair3",
	"flair24",
	"flair8",
	"flair26",
	"flair12",
	"flair7",
	"flair42",
	"flair33",
	"flair18",
	"admin",
	"flair17",
}

// ResolveFlair names the flair a username takes its colour from, or "" when
// none of the ones held colour it.
func ResolveFlair(features []string) string {
	held := make(map[string]bool, len(features))
	for _, feature := range features {
		held[feature] = true
	}
	winner := ""
	for _, flair := range colouredFlairs {
		if held[flair] {
			winner = flair
		}
	}
	return winner
}
