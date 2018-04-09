package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type Line struct {
	Character string `json:"character"`
	Text      string `json:"text"`
}

// Episode stores the metadata about a single episode
type Episode struct {
	Name string
	URL  string
}

func getEpisodes(season int) (list []Episode) {
	response, err := http.Get(fmt.Sprintf("http://mlp.wikia.com/wiki/Category:Season_%v_transcripts", season))
	if err != nil {
		fmt.Println("Error downloading episode list: ", err.Error())
		return
	}
	defer response.Body.Close()

	z := html.NewTokenizer(response.Body)

	tt := z.Next()
	for {
		switch {
		case tt == html.ErrorToken:
			return
		case tt == html.StartTagToken:
			t := z.Token()

			if t.Data == "a" {
				tt = z.Next()
				if tt == html.TextToken {
					link := z.Token()
					if strings.HasPrefix(link.Data, "Transcripts/") {
						for _, a := range t.Attr {
							if a.Key == "href" {
								list = append(list, Episode{
									Name: link.Data[12:],
									URL:  a.Val,
								})
							}
						}
					}
				}
				continue
			}
		}
		tt = z.Next()
	}
}

func processLine(lines []Line, z *html.Tokenizer) []Line {
	depth := 1
	line := ""
	character := ""
	tt := z.Next()

	for {
		switch tt {
		case html.ErrorToken:
			return lines
		case html.StartTagToken:
			t := z.Token()
			if t.Data == "dd" { // The MLP wiki uses nested lists to express song lyrics
				if len(line) > 0 {
					lines = append(lines, Line{character, line})
				}
				line = ""
				character = ""
				depth++
			} else if t.Data == "b" && line == "" { // bold text at the beginning of any line is considered a character
				if tt = z.Next(); tt == html.TextToken {
					character = z.Token().Data
				} else {
					continue
				}
			}
		case html.TextToken:
			line += z.Token().Data
		case html.EndTagToken:
			if z.Token().Data == "dd" {
				if depth--; depth <= 0 {
					if len(line) > 0 {
						lines = append(lines, Line{character, line})
					}
					return lines
				}
			}
		}
		tt = z.Next()
	}
}

func getEpisodeNumber(episode Episode) int {
	response, err := http.Get("http://mlp.wikia.com/wiki" + episode.URL[17:])
	if err != nil {
		fmt.Println("Error downloading episode page for "+episode.Name+": ", err.Error())
		return 0
	}
	defer response.Body.Close()

	z := html.NewTokenizer(response.Body)
	found := false

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			panic("Could not find episode number for " + episode.Name)
		case html.TextToken:
			t := z.Token()
			if found {
				i, err := strconv.Atoi(strings.Trim(t.Data, " \n\r\t\v"))
				if err != nil {
					panic(err.Error())
				}
				return i
			}
			found = strings.TrimSpace(t.Data) == "Season episode №:"
		}
	}
}

func processEpisode(episode Episode) (lines []Line) {
	response, err := http.Get("http://mlp.wikia.com" + episode.URL)
	if err != nil {
		fmt.Println("Error downloading transcript for "+episode.Name+": ", err.Error())
		return
	}
	defer response.Body.Close()

	z := html.NewTokenizer(response.Body)

	// Extract all lines from the transcript
	tt := z.Next()
	for {
		switch tt {
		case html.ErrorToken:
			return
		case html.StartTagToken:
			t := z.Token()
			if t.Data == "dd" {
				lines = processLine(lines, z)
			}
		case html.EndTagToken:
			t := z.Token()
			if t.Data == "dd" {
				fmt.Println("Mismatched dd tags!")
				return
			}
		}
		tt = z.Next()
	}
}

func fixEpisode(lines []Line) []Line {
	prev := "" // Store last character tag
	out := make([]Line, 0, len(lines))

	// Calculate proper character tags for each line
	for _, v := range lines {
		if len(v.Character) > 0 {
			prev = v.Character
			if v.Character[0] == '[' { // This is a song verse heading, so remove the line but set the character tag
				prev = strings.Trim(v.Character, "[]")
				continue
			}
		}
		switch v.Text[0] {
		case ':': // Character text
			v.Text = v.Text[1:] // Strip ':' from text
		case '[': // Action text
			prev = v.Character
			if len(v.Character) == 0 { // A non-empty character is an unresolved character tag
				v.Text = strings.Trim(v.Text, " \n\r\t\v")
				v.Text = v.Text[1 : len(v.Text)-1]
			}
			if index := strings.IndexRune(v.Text, ':'); index >= 0 { // Attempt to resolve action text with character tags
				if v.Text[:index] == "music" {
					break
				}
				prev += v.Text[:index]
				v.Text = v.Text[index+1:]
				//fmt.Println(prev)
				//fmt.Println(v.Text)
			}
		default: // Either a song line, an extension of a previous line, or an unresolved character tag
			if len(v.Character) > 0 { // If the character tag is set and we hit this, we need to fully resolve it.
				if index := strings.IndexRune(v.Text, ':'); index >= 0 {
					prev += v.Text[:index]
					v.Text = v.Text[index+1:]
				} else { // If we can't find any colon at all, someone made a typo on the wiki
					fmt.Println("TYPO IN THE WIKI: ", v.Character, ":", v.Text)
				}
			}
			break
		}
		out = append(out, Line{strings.Trim(prev, " \n\r\t\v"), strings.Trim(v.Text, " \n\r\t\v")})
	}

	return out
}

func main() {
	// Parse command line arguments. Usage: transcript-packer [-indexed] [min] [max]
	// If no numbers are given, defaults to 1-7. One number is treated as the max. Given two numbers, first is min, second is max.
	seasons := make(map[int]map[string][]Line)
	indexed := flag.Bool("indexed", false, "If true, indexes by number instead of by name.")
	flag.Parse()
	min := 1
	max := 7

	if flag.NArg() > 1 {
		min, _ = strconv.Atoi(flag.Arg(1))
	}
	if flag.NArg() > 0 {
		max, _ = strconv.Atoi(flag.Arg(0))
	}

	// Process seasons from min to max
	for s := min; s <= max; s++ {
		fmt.Println("Processing Season", s)
		season := make(map[string][]Line)
		episodes := getEpisodes(s)
		for _, v := range episodes {
			fmt.Printf("Processing %v: %v\n", s, v.Name)
			n := v.Name   // By default we index on episode name
			if *indexed { // If indexed, we need to find the episode number
				n = strconv.Itoa(getEpisodeNumber(v))
			}
			season[n] = fixEpisode(processEpisode(v))
		}

		seasons[s] = season
	}

	// Write out parsed episodes as JSON
	out, err := json.Marshal(&seasons)
	if err == nil {
		err = ioutil.WriteFile("transcripts.json", out, 0666)
	}
	if err != nil {
		fmt.Println("Error writing JSON: ", err.Error())
	}
}
