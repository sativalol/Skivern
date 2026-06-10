//go:build !windows && !linux
package spotify

func GetSpotifyTrack() string {
	return ""
}
