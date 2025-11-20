package opml

import (
	"MrRSS/internal/models"
	"encoding/xml"
	"io"
)

type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

type Head struct {
	Title string `xml:"title"`
}

type Body struct {
	Outlines []Outline `xml:"outline"`
}

type Outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	Type     string    `xml:"type,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	Outlines []Outline `xml:"outline"` // Nested outlines
}

func Parse(r io.Reader) ([]models.Feed, error) {
	var doc OPML
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}

	var feeds []models.Feed
	var extract func([]Outline, string)
	extract = func(outlines []Outline, category string) {
		for _, o := range outlines {
			if o.XMLURL != "" {
				title := o.Title
				if title == "" {
					title = o.Text
				}
				feeds = append(feeds, models.Feed{
					Title:    title,
					URL:      o.XMLURL,
					Category: category,
				})
			}

			newCategory := category
			if o.XMLURL == "" && o.Text != "" {
				if newCategory != "" {
					newCategory += "/" + o.Text
				} else {
					newCategory = o.Text
				}
			}

			if len(o.Outlines) > 0 {
				extract(o.Outlines, newCategory)
			}
		}
	}
	extract(doc.Body.Outlines, "")
	return feeds, nil
}

func Generate(feeds []models.Feed) ([]byte, error) {
	doc := OPML{
		Version: "1.0",
		Head: Head{
			Title: "MrRSS Subscriptions",
		},
	}

	for _, f := range feeds {
		doc.Body.Outlines = append(doc.Body.Outlines, Outline{
			Text:   f.Title,
			Title:  f.Title,
			Type:   "rss",
			XMLURL: f.URL,
		})
	}

	return xml.MarshalIndent(doc, "", "  ")
}
