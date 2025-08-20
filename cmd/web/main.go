package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

func main() {
	client_id := os.Getenv("SPOTIFY_CLIENT_ID")
	client_secret := os.Getenv("SPOTIFY_CLIENT_SECRET")

	if client_id == "" || client_secret == "" {
		fmt.Println("Missing SPOTIFY_CLIENT_ID or SPOTIFY_CLIENT_SECRET in env")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, expiresIn, err := getAccessToken(ctx, client_id, client_secret)
	if err != nil {
		fmt.Println("Token fetch failed:", err)
		return
	}

	fmt.Printf("token length: %d\n", len(token))
	fmt.Printf("expires_in (sec): %d\n", expiresIn)

	if err := callSpotifySearchParsed(ctx, token, "Daft Punk", 5); err != nil {
		fmt.Println("API call failed:", err)
	}

	bpm, stepLen := BPMEstimateSimple(1.75, 2.68224)
	fmt.Printf("Estimated cadence: %.0f spm (step length: %.2f m)\n", bpm, stepLen)

}

func getAccessToken(ctx context.Context, clientID, clientSecret string) (string, int, error) {
	basic := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret)) //base64 per Spotify

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	//make the POST request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basic)

	//send POST request through network
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	//print errors if there is one
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return "", 0, fmt.Errorf("status %s: %s", resp.Status, buf.String())
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", 0, err
	}
	return payload.AccessToken, payload.ExpiresIn, nil
}

// token = Access token, term = seach term, limit = # of results
func callSpotifySearchParsed(ctx context.Context, token, term string, limit int) error {
	type artist struct {
		Name string `json:"name"`
	}
	type track struct {
		Name    string   `json:"name"`
		Artists []artist `json:"artists"`
	}
	type tracksPage struct {
		Items []track `json:"items"`
	}
	type searchResp struct {
		Tracks tracksPage `json:"tracks"`
	}

	//build the URL for the endpoint
	baseURL := "https://api.spotify.com/v1/search"
	q := url.Values{}
	q.Set("q", term)
	q.Set("type", "track")
	q.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("search failed: %s: %s", resp.Status, buf.String())
	}

	var out searchResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}

	if len(out.Tracks.Items) == 0 {
		fmt.Println("No tracks found.")
		return nil
	}

	for i, t := range out.Tracks.Items {
		artist := "Unknown"
		if len(t.Artists) > 0 {
			artist = t.Artists[0].Name
		}
		fmt.Printf("%2d) %s — %s\n", i+1, artist, t.Name)
	}
	return nil
}

// Function to estimate the BPM or steps per minute.
func BPMEstimateSimple(height, speed float64) (float64, float64) {
	L := 0.414 * height //Stride length

	//Account for longer steps when running faster, around ~5mph or ~2.2m/s
	if speed > 2.2 {
		scale := 1.0 + 0.25*((speed-2.2)/1.8)
		if scale > 1.25 {
			scale = 1.25
		}
		L = L * scale
	}

	//cap the stride length to 55% of height
	maxL := 0.55 * height
	if L > maxL {
		L = maxL
	}

	//calculate BPM
	bpm := (speed / L) * 60.0
	return bpm, L
}
