package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/lunny/html2md"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode"
)

type Date time.Time

func (d Date) String() string {
	return time.Time(d).Format(time.RFC3339)
}

func (d *Date) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var v string
	dec.DecodeElement(&v, &start)
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

type Author struct {
	Name string `xml:"name"`
	Uri  string `xml:"uri"`
}

type Export struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Entries []Entry  `xml:"entry"`
}

type Entry struct {
	ID          string `xml:"id"`
	Updated     Date   `xml:"updated"`
	Title       string `xml:"title"`
	Content     string `xml:"content"`
	Links       Links  `xml:"link"`
	Author      Author `xml:"author"`
	Description string
	Extra       string
	Repo        string
}

type Link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type Links []Link

var templ = `---
title: "{{ .Title }}: {{ .Updated | ymd }}"
date: {{ .Updated }}
description: "{{ .Description }}"
changelog:
- {{ .Repo }}
version: "{{ .Title }}"
author:
  name: "{{ .Author.Name }}"
---

{{ .Content }}
`

var funcMap = template.FuncMap{
	"ymd": yearMonthDate,
}
var t = template.Must(template.New("").Funcs(funcMap).Parse(templ))

func yearMonthDate(date Date) string {
	d := time.Time(date)
	return fmt.Sprintf("%0d-%02d-%02d", d.Year(), d.Month(), d.Day())
}

func main() {
	log.SetFlags(0)

	extra := flag.String("extra", "", "additional metadata to set in frontmatter")
	flag.Parse()

	args := flag.Args()

	if len(args) != 2 {
		log.Printf("Usage: %s [options] <xmlfile> <targetdir>", os.Args[0])
		log.Println("options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	dir := args[1]

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err == nil {
			info, err = os.Stat(dir)
		}
	}
	if err != nil {
		log.Fatal(err)
	}

	if info == nil || !info.IsDir() {
		log.Fatal("Second argument is not a directory.")
	}

	b, err := ioutil.ReadFile(args[0])
	if err != nil {
		log.Fatal(err)
	}

	exp := Export{}

	err = xml.Unmarshal(b, &exp)
	if err != nil {
		log.Fatal(err)
	}

	if len(exp.Entries) < 1 {
		log.Fatal("No releases found!")
	}

	count := 0
	drafts := 0
	for _, entry := range exp.Entries {
		entry.Repo = strings.Replace(exp.Title, "Release notes from ", "", 1)
		if len(exp.Title) > 0 {
			entry.Description = fmt.Sprintf("%s: %s", exp.Title, entry.Title)
		}
		if extra != nil {
			entry.Extra = *extra
		}
		entry.Content = html2md.Convert(entry.Content)

		if err := writeEntry(entry, dir); err != nil {
			log.Fatalf("Failed writing post %q to disk:\n%s", entry.Title, err)
		}
		count++
	}
	log.Printf("Wrote %d published posts to disk.", count)
	log.Printf("Wrote %d drafts to disk.", drafts)
}

func writeEntry(e Entry, dir string) error {
	filename := filepath.Join(dir, makePath(e.Title)+".md")
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, e)
}

// Take a string with any characters and replace it so the string could be used in a path.
// E.g. Social Media -> social-media
func makePath(s string) string {
	return unicodeSanitize(strings.ToLower(strings.Replace(strings.TrimSpace(s), " ", "-", -1)))
}

func unicodeSanitize(s string) string {
	source := []rune(s)
	target := make([]rune, 0, len(source))

	for _, r := range source {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			target = append(target, r)
		}
	}

	return string(target)
}
