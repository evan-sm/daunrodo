package downloader

import (
	"context"
	"fmt"

	"daunrodo/internal/entity"
	"daunrodo/internal/storage"
)

// Rule routes jobs matching Match to DL.
type Rule struct {
	Match func(url string) bool
	DL    Downloader
}

// Router selects a Downloader by URL rules, falling back to defaultDL.
type Router struct {
	defaultDL Downloader
	rules     []Rule
}

// NewRouter creates a Router with a default downloader and optional rules.
func NewRouter(defaultDL Downloader, rules ...Rule) Downloader {
	return &Router{
		defaultDL: defaultDL,
		rules:     rules,
	}
}

// Process dispatches the job to the first matching rule, or the default downloader.
func (r *Router) Process(ctx context.Context, job *entity.Job, storer storage.Storer) error {
	if job != nil {
		for _, rule := range r.rules {
			if rule.Match != nil && rule.DL != nil && rule.Match(job.URL) {
				if err := rule.DL.Process(ctx, job, storer); err != nil {
					return fmt.Errorf("routed downloader: %w", err)
				}

				return nil
			}
		}
	}

	if err := r.defaultDL.Process(ctx, job, storer); err != nil {
		return fmt.Errorf("default downloader: %w", err)
	}

	return nil
}
