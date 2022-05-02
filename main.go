package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	texttospeechpb "google.golang.org/genproto/googleapis/cloud/texttospeech/v1"
)

var speechClient *texttospeech.Client
var ctx context.Context
var failure = -1
var success = 0

func init() {
	var err error

	ctx = context.Background()

	speechClient, err = texttospeech.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	defer speechClient.Close()

	os.Exit(run())
}

func run() int {
	audio := []byte{}

	// read ADIF from std
	qsos, err := read(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", "Failed to read input:", err)
		return failure
	}

	// collect each qso in mp3 format
	for _, qso := range qsos {
		data, err := speak(qso)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", "Failed to read input:", err)
			return failure
		}

		audio = append(audio, data...)
	}

	// write to file
	filename := "output.mp3"
	err = ioutil.WriteFile(filename, audio, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", "Failed to write output:", err)
		return failure
	}
	fmt.Printf("Audio content written to file: %v\n", filename)

	return success
}

func read(in io.Reader) ([]string, error) {
	s := bufio.NewScanner(in)
	qsos := []string{}

	for s.Scan() {
		qso := s.Text()

		if strings.HasPrefix(qso, "<QSO_DATE:") {
			r, err := parse(qso)
			if err != nil {
				return qsos, err
			}

			qsos = append(qsos, r)
		}
	}

	return qsos, nil
}

type fieldInfo struct {
	key               string
	spoken            string
	suffix            string
	truncateTrailingN int
	spaced            bool
}

func parse(record string) (string, error) {

	fields := []fieldInfo{
		{key: "<TIME_ON:", spoken: "time"},
		{key: "<BAND:", spoken: "band", truncateTrailingN: 1, suffix: " meters "},
		{key: "<MODE:", spoken: "mode"},
		{key: "<CALL:", spoken: "station", spaced: true},
		{key: "<RST_SENT:", spoken: "send"},
		{key: "<RST_RCVD:", spoken: "received"},
		{key: "<SIG_INFO:", spoken: "information"},
		{key: "<COMMENT:", spoken: "comment"},
		{key: "<OPERATOR:", spoken: "operator", spaced: true},
	}

	end := ">"

	var response strings.Builder

	for _, field := range fields {
		startIdx := strings.Index(record, field.key)
		if startIdx == -1 {
			continue // field missing
		}

		endIdx := strings.Index(record[startIdx:], end) + startIdx

		dataLen := record[startIdx+len(field.key) : endIdx]
		dl, err := strconv.Atoi(dataLen)
		if err != nil {
			return "", err
		}

		dataIdxStart := startIdx + len(field.key) + len(dataLen) + len(end)
		dataIdxEnd := dataIdxStart + dl

		data := strings.TrimSpace(record[dataIdxStart:dataIdxEnd])

		response.WriteString(field.spoken)
		response.WriteRune(' ')

		if field.spaced {
			for _, r := range data {
				response.WriteRune(r)
				response.WriteRune(' ')
			}
		} else {
			response.WriteString(data[:len(data)-int(field.truncateTrailingN)])
			response.WriteRune(' ')
		}

		if len(field.suffix) > 0 {
			response.WriteString(field.suffix)
			response.WriteRune(' ')
		}
	}

	return response.String(), nil
}

func speak(text string) ([]byte, error) {
	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-US",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
		},
	}

	resp, err := speechClient.SynthesizeSpeech(ctx, &req)
	if err != nil {
		return nil, err
	}

	return resp.AudioContent, nil

	// filename := "output.mp3"
	// err := ioutil.WriteFile(filename, audioContent, 0644)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("Audio content written to file: %v\n", filename)

	// return nil
}
