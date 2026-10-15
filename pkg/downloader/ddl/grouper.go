package ddl

import (
	"context"
	"fmt"
	"strings"
)

// HosterRank returns priority score (lower is higher priority)
func HosterRank(hoster string) int {
	switch hoster {
	case HosterPixelDrain:
		return 1
	case HosterGoFile:
		return 2
	case HosterFileditch:
		return 3
	case Hoster1fichier:
		return 4
	case HosterDirect:
		return 5
	default:
		return 6
	}
}

// Grouper analyzes and structures raw URIs into multi-part plans with hoster mirrors
type Grouper struct {
	resolvers []HosterResolver
}

// NewGrouper initializes standard hoster resolvers
func NewGrouper(oneFichierAPIKey string) *Grouper {
	return &Grouper{
		resolvers: []HosterResolver{
			NewPixelDrainResolver(),
			NewGoFileResolver(),
			NewOneFichierResolver(oneFichierAPIKey),
			NewDirectResolver(),
		},
	}
}

// DetectHoster determines which hoster handles the given URL
func (g *Grouper) DetectHoster(rawURL string) (string, HosterResolver) {
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" || strings.HasPrefix(strings.ToLower(cleanURL), "magnet:") {
		return "", nil
	}
	for _, r := range g.resolvers {
		if r.CanHandle(cleanURL) {
			return r.Name(), r
		}
	}
	return HosterDirect, NewDirectResolver()
}

// GroupURIs organizes URIs into hoster buckets
func (g *Grouper) GroupURIs(uris []string) map[string][]string {
	grouped := make(map[string][]string)
	for _, raw := range uris {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "magnet:") {
			continue
		}
		hoster, _ := g.DetectHoster(trimmed)
		if hoster != "" {
			grouped[hoster] = append(grouped[hoster], trimmed)
		}
	}
	return grouped
}

// GetAvailableHosters returns unique hoster names found in URIs sorted by priority
func (g *Grouper) GetAvailableHosters(uris []string) []string {
	seen := make(map[string]bool)
	var list []string
	for _, raw := range uris {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "magnet:") {
			continue
		}
		hoster, _ := g.DetectHoster(trimmed)
		if hoster != "" && !seen[hoster] {
			seen[hoster] = true
			list = append(list, hoster)
		}
	}

	// Sort by rank
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if HosterRank(list[j]) < HosterRank(list[i]) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	return list
}

// BuildPlan constructs a multi-part download plan with mirrors
func (g *Grouper) BuildPlan(uris []string, preferredHoster string) (*GameDownloadPlan, error) {
	if len(uris) == 0 {
		return nil, fmt.Errorf("no URIs provided")
	}

	grouped := g.GroupURIs(uris)
	if len(grouped) == 0 {
		return nil, fmt.Errorf("could not classify any URIs into supported hosters")
	}

	available := g.GetAvailableHosters(uris)

	// Determine total parts: maximum number of links provided by any single hoster
	totalParts := 0
	for _, hostLinks := range grouped {
		if len(hostLinks) > totalParts {
			totalParts = len(hostLinks)
		}
	}
	if totalParts == 0 {
		totalParts = 1
	}

	// Select primary hoster: use preferred if available, otherwise highest-ranked available
	selectedHost := ""
	if preferredHoster != "" {
		for _, h := range available {
			if strings.EqualFold(h, preferredHoster) {
				selectedHost = h
				break
			}
		}
	}
	if selectedHost == "" && len(available) > 0 {
		selectedHost = available[0]
	}

	parts := make([]PartPlan, totalParts)
	for i := 0; i < totalParts; i++ {
		parts[i] = PartPlan{
			PartIndex:  i + 1,
			TotalParts: totalParts,
			Mirrors:    make([]*ResolvedItem, 0),
		}

		// Collect links for this part from each hoster
		for _, host := range available {
			links := grouped[host]
			if i < len(links) {
				item := &ResolvedItem{
					DirectURL:   links[i],
					Hoster:      host,
					OriginalURL: links[i],
				}
				parts[i].Mirrors = append(parts[i].Mirrors, item)
			}
		}

		// Pick selected URL for this part
		for _, m := range parts[i].Mirrors {
			if m.Hoster == selectedHost {
				parts[i].SelectedURL = m
				break
			}
		}
		if parts[i].SelectedURL == nil && len(parts[i].Mirrors) > 0 {
			parts[i].SelectedURL = parts[i].Mirrors[0]
		}
	}

	return &GameDownloadPlan{
		Parts:          parts,
		AvailableHosts: available,
		SelectedHoster: selectedHost,
	}, nil
}

// ResolvePartDirectURL resolves the direct link for a specific mirror
func (g *Grouper) ResolvePartDirectURL(ctx context.Context, item *ResolvedItem) (*ResolvedItem, error) {
	_, resolver := g.DetectHoster(item.OriginalURL)
	if resolver == nil {
		return nil, fmt.Errorf("no resolver available for %s", item.OriginalURL)
	}
	resolved, err := resolver.Resolve(ctx, item.OriginalURL)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}
