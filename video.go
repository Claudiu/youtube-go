package youtube

import (
	"bufio"
	"errors"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	hostName     = "www.youtube.com"
	videoURL     = "/watch"
	videoInfoURL = "/get_video_info"
)

var (
	playerJsRe  = regexp.MustCompile(`"js"\s*:\s*"(.+?)"`)
	urlMapRe    = regexp.MustCompile(`encoded_fmt_stream_map"\s*:\s*"(.+?)"`)
	videoDescRe = regexp.MustCompile(`content="(.+?)"`)
)

type Video struct {
	Id          string
	Title       string
	Duration    int
	Description string
	ViewCount   int64
	Rating      float64
	Formats     []*VideoFormat
}

type VideoFormat struct {
	Itag    int
	Type    string
	Quality string
	URL     string
}

func GetVideo(id string) (*Video, error) {
	params := url.Values{"video_id": {id}, "el": {"detailpage"}}.Encode()
	videoURL := url.URL{Host: hostName, Path: videoInfoURL,
		Scheme: "https", RawQuery: params}

	data, err := getURLData(videoURL.String())
	if err != nil {
		return nil, err
	}

	vals, err := url.ParseQuery(string(data))
	if err != nil {
		return nil, err
	}

	title := vals.Get("title")
	if title == "" {
		return nil, errors.New("Could not get video title")
	}

	duration, err := strconv.Atoi(vals.Get("length_seconds"))
	if err != nil {
		return nil, errors.New("Could not get video duration")
	}

	viewCount, err := strconv.ParseInt(vals.Get("view_count"), 10, 64)
	if err != nil {
		return nil, errors.New("Could not get video view count")
	}

	rating, err := strconv.ParseFloat(vals.Get("avg_rating"), 64)
	if err != nil {
		return nil, errors.New("Could not get video rating")
	}

	video := &Video{
		Id:          id,
		Title:       title,
		Duration:    duration,
		Description: "",
		ViewCount:   viewCount,
		Rating:      rating,
	}

	return video, video.ReadFormats()
}

func (v *Video) ReadFormats() error {
	v.Formats = []*VideoFormat{}

	params := url.Values{"v": {v.Id}}.Encode()
	videoURL := url.URL{Host: hostName, Path: videoURL,
		Scheme: "https", RawQuery: params}

	data, err := getURLData(videoURL.String())
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()

		// Try and read the description of the video if not present
		if v.Description == "" {
			if strings.Contains(line, "<meta name=\"description\"") {
				matches := videoDescRe.FindStringSubmatch(line)
				if matches != nil && len(matches) > 1 {
					v.Description = html.UnescapeString(matches[1])
				}

				continue
			}
		}

		if !strings.Contains(line, "ytplayer.config") {
			continue
		}

		// Get player js url
		matches := playerJsRe.FindStringSubmatch(line)
		if matches == nil || len(matches) <= 1 {
			return errors.New("Could not find the url of the js player")
		}

		jsURL := strings.Replace(matches[1], "\\/", "/", -1)
		jsData, err := getURLData(strings.Replace(jsURL, "//", "https://", 1))
		if err != nil {
			return err
		}

		// Get format stream map
		matches = urlMapRe.FindStringSubmatch(line)
		if matches == nil || len(matches) <= 1 {
			return errors.New("Could not find format stream map")
		}

		fmtStreamMap := strings.Replace(matches[1], "\\u0026", "&", -1)
		v.Formats, err = parseFormats(fmtStreamMap, jsData)
		if err != nil {
			return err
		}
	}

	return nil
}
