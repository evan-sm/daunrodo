package downloader

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"daunrodo/internal/errs"
)

var (
	reInstagramReelCode = regexp.MustCompile(`"code"\s*:\s*"([^"]+)"`)
	reMPDDuration       = regexp.MustCompile(`(?i)^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?$`)
)

// IsInstagramReel reports whether raw looks like an Instagram Reel URL.
func IsInstagramReel(raw string) bool {
	_, err := InstagramReelShortcode(raw)

	return err == nil
}

// InstagramReelShortcode extracts the reel shortcode from a URL.
func InstagramReelShortcode(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return "", errs.ErrInstagramReelShortcode
	}

	host := strings.ToLower(parsed.Host)
	host = strings.TrimPrefix(host, "www.")

	if host != "instagram.com" && host != "instagr.am" {
		return "", errs.ErrInstagramReelShortcode
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		seg := strings.ToLower(parts[i])
		if seg == "reel" || seg == "reels" {
			code := parts[i+1]
			if code != "" && !strings.Contains(code, ".") {
				return code, nil
			}
		}
	}

	return "", errs.ErrInstagramReelShortcode
}

// dashAV holds selected DASH representation URLs and metadata.
type dashAV struct {
	VideoURL  string
	AudioURL  string
	Width     int
	Height    int
	Bandwidth int
	Duration  time.Duration
}

type mpdDoc struct {
	Duration string      `xml:"mediaPresentationDuration,attr"`
	Periods  []mpdPeriod `xml:"Period"`
}

type mpdPeriod struct {
	Sets []mpdAdaptationSet `xml:"AdaptationSet"`
}

type mpdAdaptationSet struct {
	ContentType string              `xml:"contentType,attr"`
	MimeType    string              `xml:"mimeType,attr"`
	Reps        []mpdRepresentation `xml:"Representation"`
}

type mpdRepresentation struct {
	Bandwidth int    `xml:"bandwidth,attr"`
	Width     int    `xml:"width,attr"`
	Height    int    `xml:"height,attr"`
	Codecs    string `xml:"codecs,attr"`
	MimeType  string `xml:"mimeType,attr"`
	BaseURL   string `xml:"BaseURL"`
}

// extractInstagramReelDASH finds the reel node for shortcode and picks best video+audio BaseURLs.
func extractInstagramReelDASH(page, shortcode string) (dashAV, error) {
	manifest, err := extractVideoDashManifest(page, shortcode)
	if err != nil {
		return dashAV{}, err
	}

	return pickBestFromMPD(manifest)
}

func extractVideoDashManifest(page, shortcode string) (string, error) {
	if shortcode == "" {
		return "", errs.ErrInstagramReelShortcode
	}

	reCode := regexp.MustCompile(`"code"\s*:\s*"` + regexp.QuoteMeta(shortcode) + `"`)

	loc := reCode.FindStringIndex(page)
	if loc == nil {
		return "", fmt.Errorf("%w: shortcode %s", errs.ErrInstagramReelManifest, shortcode)
	}

	rest := page[loc[1]:]
	if next := reInstagramReelCode.FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}

	idx := strings.Index(rest, `"video_dash_manifest"`)
	if idx < 0 {
		return "", fmt.Errorf("%w: shortcode %s", errs.ErrInstagramReelManifest, shortcode)
	}

	colon := strings.Index(rest[idx:], ":")
	if colon < 0 {
		return "", errs.ErrInstagramReelManifest
	}

	raw := strings.TrimLeft(rest[idx+colon+1:], " \n\r\t")

	var manifest string
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&manifest); err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrInstagramReelManifest, err)
	}

	if strings.TrimSpace(manifest) == "" {
		return "", errs.ErrInstagramReelManifest
	}

	return manifest, nil
}

func pickBestFromMPD(manifestXML string) (dashAV, error) {
	var doc mpdDoc
	if err := xml.Unmarshal([]byte(manifestXML), &doc); err != nil {
		return dashAV{}, fmt.Errorf("%w: %v", errs.ErrInstagramReelManifest, err)
	}

	var bestVideo, bestAudio *mpdRepresentation

	for _, period := range doc.Periods {
		for _, set := range period.Sets {
			for i := range set.Reps {
				rep := &set.Reps[i]
				switch {
				case isDASHAudio(set, rep):
					if bestAudio == nil || rep.Bandwidth > bestAudio.Bandwidth {
						bestAudio = rep
					}
				case isDASHVideo(set, rep):
					if bestVideo == nil || rep.Bandwidth > bestVideo.Bandwidth {
						bestVideo = rep
					}
				}
			}
		}
	}

	if bestVideo == nil || strings.TrimSpace(bestVideo.BaseURL) == "" {
		return dashAV{}, errs.ErrInstagramReelVideo
	}

	if bestAudio == nil || strings.TrimSpace(bestAudio.BaseURL) == "" {
		return dashAV{}, errs.ErrInstagramReelAudio
	}

	return dashAV{
		VideoURL:  bestVideo.BaseURL,
		AudioURL:  bestAudio.BaseURL,
		Width:     bestVideo.Width,
		Height:    bestVideo.Height,
		Bandwidth: bestVideo.Bandwidth,
		Duration:  parseMPDDuration(doc.Duration),
	}, nil
}

func isDASHAudio(set mpdAdaptationSet, rep *mpdRepresentation) bool {
	if strings.EqualFold(set.ContentType, "audio") {
		return true
	}

	mime := rep.MimeType
	if mime == "" {
		mime = set.MimeType
	}

	if strings.HasPrefix(strings.ToLower(mime), "audio/") {
		return true
	}

	return strings.HasPrefix(strings.ToLower(rep.Codecs), "mp4a")
}

func isDASHVideo(set mpdAdaptationSet, rep *mpdRepresentation) bool {
	if strings.EqualFold(set.ContentType, "video") {
		return true
	}

	mime := rep.MimeType
	if mime == "" {
		mime = set.MimeType
	}

	if strings.HasPrefix(strings.ToLower(mime), "video/") {
		return true
	}

	return rep.Height > 0 || rep.Width > 0
}

func parseMPDDuration(raw string) time.Duration {
	matches := reMPDDuration.FindStringSubmatch(strings.TrimSpace(raw))
	if matches == nil {
		return 0
	}

	var total time.Duration

	if matches[1] != "" {
		hours, _ := strconv.Atoi(matches[1])
		total += time.Duration(hours) * time.Hour
	}

	if matches[2] != "" {
		minutes, _ := strconv.Atoi(matches[2])
		total += time.Duration(minutes) * time.Minute
	}

	if matches[3] != "" {
		sec, err := strconv.ParseFloat(matches[3], 64)
		if err == nil {
			total += time.Duration(sec * float64(time.Second))
		}
	}

	return total
}
