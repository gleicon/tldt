package main

import "github.com/JesusIslam/tldr"

func Summarize(sentences int, body string) (string, error) {
	bag := tldr.New()
	return bag.Summarize(body, sentences)
}
