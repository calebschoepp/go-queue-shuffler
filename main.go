package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/zmb3/spotify"
)

var redirectURL = "http://localhost:8080/callback"
var auth = spotify.Authenticator{}
var state = "abc"
var client spotify.Client

// This is a big no no.
var clientID = ""
var secretKey = ""

var sentinelTrackID = spotify.ID("4uLU6hMCjMI75M1A2tKUQC") // Never gonna give you up

func main() {
	// the redirect URL must be an exact match of a URL you've registered for your application
	// scopes determine which permissions the user is prompted to authorize
	auth = spotify.NewAuthenticator(
		redirectURL,
		spotify.ScopeUserReadCurrentlyPlaying,
		spotify.ScopeUserReadPlaybackState,
		spotify.ScopeUserModifyPlaybackState,
		spotify.ScopeStreaming,
	)
	auth.SetAuthInfo(clientID, secretKey)
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/callback", redirectHandler)
	http.HandleFunc("/home", homeHandler)
	http.HandleFunc("/shuffle", shuffleHandler)
	http.ListenAndServe(":8080", nil)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<a href=\"%s\">Get Token!</a>\n", auth.AuthURL(state))
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	// use the same state string here that you used to generate the URL
	token, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusNotFound)
		return
	}
	// create a client using the specified token
	client = auth.NewClient(token)

	http.Redirect(w, r, "/home", 303)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	html := "<html><form action=\"/shuffle\" method=\"post\"><input type=\"submit\" name=\"shuffle\" value=\"Shuffle\" /></form></html>"
	fmt.Fprintf(w, html)
}

func shuffleHandler(w http.ResponseWriter, r *http.Request) {
	// Get current song and seek position
	fmt.Println("Fetching player state")
	playerState, err := client.PlayerState()
	if err != nil {
		reportError(w, err, "Failed to get player state")
		return
	}
	currentSongID := playerState.CurrentlyPlaying.Item.ID
	currentSongPosition := playerState.CurrentlyPlaying.Progress
	currentSongIsPaused := playerState.CurrentlyPlaying.Playing

	// Pause the player
	fmt.Println("Pausing the player")
	err = client.Pause()
	if err != nil {
		reportError(w, err, "Failed to pause player")
		return
	}

	// Add sentienl song to the queue
	fmt.Println("Queueing sentinel song")
	err = client.QueueSong(sentinelTrackID)
	if err != nil {
		reportError(w, err, "Failed to queue sentinel song")
		return
	}

	// Log and skip songs until sentinel song is found
	i := 0
	var queuedSongs []spotify.ID
	for {
		fmt.Printf("Processing next song (i=%d): ", i)

		// Skip to next song
		err = client.Next()
		if err != nil {
			reportError(w, err, fmt.Sprintf("Failed to skip to next song (i=%d)", i))
			return
		}

		// Pause the player
		err = client.Pause()
		if err != nil {
			reportError(w, err, "Failed to pause player")
			return
		}

		// Grab next song
		playerState, err = client.PlayerState()
		if err != nil {
			reportError(w, err, fmt.Sprintf("Failed to get player state (i=%d)", i))
			return
		}

		// Check if the sentinel song was found
		fmt.Println(playerState.CurrentlyPlaying.Item.Name)
		if playerState.CurrentlyPlaying.Item.ID == sentinelTrackID {
			break
		}

		// Add the song id to a list that will be shuffled
		queuedSongs = append(queuedSongs, playerState.CurrentlyPlaying.Item.ID)
		i++
	}

	// Shuffle queued songs
	fmt.Println("Shuffling queued songs")
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(queuedSongs), func(i, j int) { queuedSongs[i], queuedSongs[j] = queuedSongs[j], queuedSongs[i] })

	// Reset player by:
	// Queueing current song
	fmt.Println("Queueing current song")
	err = client.QueueSong(currentSongID)
	if err != nil {
		reportError(w, err, "Failed to queue current song")
		return
	}

	// Skipping to current song
	fmt.Println("Skipping to current song")
	err = client.Next()
	if err != nil {
		reportError(w, err, "Failed to skip to current song")
		return
	}

	// Seeking to correct spot in song
	fmt.Println("Seeking to correct spot in current song")
	err = client.Seek(currentSongPosition)
	if err != nil {
		reportError(w, err, "Failed to seek to correct spot in current song")
		return
	}

	// Starting the player if necessary
	if !currentSongIsPaused {
		fmt.Println("Playing current song")
		err = client.Seek(currentSongPosition)
		if err != nil {
			reportError(w, err, "Failed to play current son")
			return
		}
	}

	// Add shuffled songs to queue
	fmt.Println("Queueing shuffled song")
	for i, song := range queuedSongs {
		err = client.QueueSong(song)
		if err != nil {
			reportError(w, err, fmt.Sprintf("Failed to queue shuffled song (i=%d)", i))
			return
		}
	}

	fmt.Println("Successfully finished")
	fmt.Fprintf(w, "Success")
}

func reportError(w http.ResponseWriter, err error, msg string) {
	fmt.Printf("%s: %s\n", msg, err.Error())
	http.Error(w, msg, http.StatusInternalServerError)
}
