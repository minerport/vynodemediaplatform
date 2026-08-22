package media

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var tvSxE = regexp.MustCompile(`(?i)(?:^|[ ._-])s(\d{1,2})e(\d{1,3})(?:e(\d{1,3}))?(?:[ ._-]|$)`)
var tvX = regexp.MustCompile(`(?i)(?:^|[ ._-])(\d{1,2})x(\d{1,3})(?:[ ._-]|$)`)
var yearPattern = regexp.MustCompile(`(?:^|[ ._(-])((?:19|20)\d{2})(?:[ ._)-]|$)`)
var qualityPattern = regexp.MustCompile(`(?i)[ ._-](?:480p|720p|1080p|1440p|2160p|4k|bluray|web[-_. ]?dl|remux).*$`)

func ParseFilename(name string) FilenameHints {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	h := FilenameHints{}
	cut := len(base)
	if m := tvSxE.FindStringSubmatchIndex(base); m != nil {
		h.SeasonNumber, _ = strconv.Atoi(base[m[2]:m[3]])
		h.EpisodeStart, _ = strconv.Atoi(base[m[4]:m[5]])
		if m[6] >= 0 {
			h.EpisodeEnd, _ = strconv.Atoi(base[m[6]:m[7]])
		} else {
			h.EpisodeEnd = h.EpisodeStart
		}
		cut = m[0]
	} else if m := tvX.FindStringSubmatchIndex(base); m != nil {
		h.SeasonNumber, _ = strconv.Atoi(base[m[2]:m[3]])
		h.EpisodeStart, _ = strconv.Atoi(base[m[4]:m[5]])
		h.EpisodeEnd = h.EpisodeStart
		cut = m[0]
	}
	if m := yearPattern.FindStringSubmatchIndex(base); m != nil {
		h.CandidateYear, _ = strconv.Atoi(base[m[2]:m[3]])
		if m[0] < cut {
			cut = m[0]
		}
	}
	title := qualityPattern.ReplaceAllString(base[:cut], "")
	title = strings.TrimSpace(strings.Join(strings.Fields(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(title)), " "))
	h.CandidateTitle = title
	return h
}

func Resolution(width, height int) string {
	switch {
	case width >= 3500 || height >= 2000:
		return "2160P"
	case width >= 2400 || height >= 1300:
		return "1440P"
	case width >= 1600 || height >= 900:
		return "1080P"
	case width >= 1100 || height >= 650:
		return "720P"
	case width > 0 && height > 0:
		return "SD"
	default:
		return "OTHER"
	}
}
