package helpers

import (
	"time"

	"go.rumenx.com/sitemap"
)

type SitemapInterface interface {
	Add(path string)
	Get() *sitemap.Sitemap
}
type Sitemap struct {
	targetMap *sitemap.Sitemap
	baseURL   string
}

func (s *Sitemap) Add(path string) {
	s.targetMap.Add(path, time.Now(), 1.0, sitemap.Daily)
}

func (s *Sitemap) Get() *sitemap.Sitemap {
	return s.targetMap
}

func NewSitemap(url string) *Sitemap {
	sitemap := Sitemap{targetMap: sitemap.New(), baseURL: url}
	return &sitemap
}
