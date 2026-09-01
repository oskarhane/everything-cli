package youtube

// SelectTrack picks the best caption track for the requested language: a
// non-generated track whose Lang equals lang wins, then a generated track
// whose Lang equals lang, then the first track in the list regardless of
// language. An empty list yields ErrNoCaptions.
func SelectTrack(tracks []Track, lang string) (Track, error) {
	if len(tracks) == 0 {
		return Track{}, ErrNoCaptions
	}
	for _, t := range tracks {
		if t.Lang == lang && !t.Generated {
			return t, nil
		}
	}
	for _, t := range tracks {
		if t.Lang == lang && t.Generated {
			return t, nil
		}
	}
	return tracks[0], nil
}
