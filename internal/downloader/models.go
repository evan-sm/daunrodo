package downloader

// ResultJSON represents the JSON output from yt-dlp.
type ResultJSON struct {
	Type               string    `json:"_type"`
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	Ext                string    `json:"ext"`
	Timestamp          int       `json:"timestamp"`
	Duration           float64   `json:"duration"`
	Channel            string    `json:"channel"`
	Uploader           string    `json:"uploader"`
	UploaderID         string    `json:"uploader_id"`
	ViewCount          int64     `json:"view_count"`
	LikeCount          int64     `json:"like_count"`
	CommentCount       int64     `json:"comment_count"`
	PostExtractor      any       `json:"__post_extractor"`
	Entries            []Entries `json:"entries"`
	Thumbnail          string    `json:"thumbnail"`
	WebpageURL         string    `json:"webpage_url"`
	OriginalURL        string    `json:"original_url"`
	WebpageURLBasename string    `json:"webpage_url_basename"`
	WebpageURLDomain   string    `json:"webpage_url_domain"`
	Extractor          string    `json:"extractor"`
	ExtractorKey       string    `json:"extractor_key"`
	UploadDate         string    `json:"upload_date"`
	ReleaseYear        any       `json:"release_year"`
	PlaylistCount      int       `json:"playlist_count"`
	Epoch              int       `json:"epoch"`
	Version            Version   `json:"_version"`
	VCodec             string    `json:"vcodec"`
	ACodec             string    `json:"acodec"`
	Width              int       `json:"width"`
	Height             int       `json:"height"`
	Filesize           int64     `json:"filesize"`
	Format             string    `json:"format"`
	Filename           string    `json:"filename"`
}

// GetThumbnail returns the thumbnail URL from the result.
func (res *ResultJSON) GetThumbnail() string {
	if len(res.Entries) > 0 {
		return res.Entries[0].Thumbnail
	}

	if res.Thumbnail != "" {
		return res.Thumbnail
	}

	return ""
}

// GetViewCount returns the view count as an integer.
// func (res *ResultJSON) GetViewCount() int {
// 	switch val := res.ViewCount.(type) {
// 	case int:
// 		return val
// 	case float64:
// 		return int(val)
// 	case string:
// 		i, _ := strconv.Atoi(val)

// 		return i
// 	default:
// 		return 0
// 	}
// }

// Entries represents an entry in a playlist or multiple entries result.
type Entries struct {
	ID                 string  `json:"id"`
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	Timestamp          int     `json:"timestamp"`
	Channel            string  `json:"channel"`
	Uploader           string  `json:"uploader"`
	UploaderID         string  `json:"uploader_id"`
	ViewCount          any     `json:"view_count"`
	LikeCount          int     `json:"like_count"`
	CommentCount       int     `json:"comment_count"`
	Duration           int     `json:"duration"`
	PlaylistCount      int     `json:"playlist_count"`
	Playlist           string  `json:"playlist"`
	PlaylistID         string  `json:"playlist_id"`
	PlaylistTitle      string  `json:"playlist_title"`
	PlaylistUploader   string  `json:"playlist_uploader"`
	PlaylistUploaderID string  `json:"playlist_uploader_id"`
	PlaylistChannel    string  `json:"playlist_channel"`
	PlaylistChannelID  any     `json:"playlist_channel_id"`
	PlaylistWebpageURL string  `json:"playlist_webpage_url"`
	NEntries           int     `json:"n_entries"`
	WebpageURL         string  `json:"webpage_url"`
	WebpageURLBasename string  `json:"webpage_url_basename"`
	WebpageURLDomain   string  `json:"webpage_url_domain"`
	PlaylistIndex      int     `json:"playlist_index"`
	LastPlaylistIndex  int     `json:"__last_playlist_index"`
	Extractor          string  `json:"extractor"`
	ExtractorKey       string  `json:"extractor_key"`
	PlaylistAutonumber int     `json:"playlist_autonumber"`
	Thumbnail          string  `json:"thumbnail"`
	DisplayID          string  `json:"display_id"`
	Fulltitle          string  `json:"fulltitle"`
	DurationString     string  `json:"duration_string"`
	UploadDate         string  `json:"upload_date"`
	ReleaseYear        any     `json:"release_year"`
	RequestedSubtitles any     `json:"requested_subtitles"`
	HasDrm             any     `json:"_has_drm"`
	Epoch              int     `json:"epoch"`
	Format             string  `json:"format"`
	FormatID           string  `json:"format_id"`
	Ext                string  `json:"ext"`
	Protocol           string  `json:"protocol"`
	Language           any     `json:"language"`
	FormatNote         string  `json:"format_note"`
	FilesizeApprox     int     `json:"filesize_approx"`
	Tbr                float64 `json:"tbr"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	Resolution         string  `json:"resolution"`
	Fps                any     `json:"fps"`
	DynamicRange       string  `json:"dynamic_range"`
	Vcodec             string  `json:"vcodec"`
	Vbr                float64 `json:"vbr"`
	StretchedRatio     any     `json:"stretched_ratio"`
	AspectRatio        float64 `json:"aspect_ratio"`
	Acodec             string  `json:"acodec"`
	Abr                float64 `json:"abr"`
	Asr                int     `json:"asr"`
	AudioChannels      any     `json:"audio_channels"`
}

// Version represents the version information of yt-dlp.
type Version struct {
	Version        string `json:"version"`
	CurrentGitHead any    `json:"current_git_head"`
	ReleaseGitHead string `json:"release_git_head"`
	Repository     string `json:"repository"`
}
