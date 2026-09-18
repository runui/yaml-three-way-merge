package smdintent

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type namedEntry struct {
	name  string
	value any
}

func reconcileNamedOrigins(oldYAML, userYAML, upstreamYAML, finalYAML []byte) ([]byte, int, error) {
	oldDoc, err := decodeDocument(oldYAML)
	if err != nil {
		return nil, 0, err
	}
	userDoc, err := decodeDocument(userYAML)
	if err != nil {
		return nil, 0, err
	}
	upstreamDoc, err := decodeDocument(upstreamYAML)
	if err != nil {
		return nil, 0, err
	}
	finalDoc, err := decodeDocument(finalYAML)
	if err != nil {
		return nil, 0, err
	}

	rewrites := 0
	for _, serviceName := range serviceNames(oldDoc, userDoc, upstreamDoc) {
		for _, field := range []string{"depends_on", "networks"} {
			merged, changed := mergeNamedOrigins(
				namedEntries(oldDoc, serviceName, field),
				namedEntries(userDoc, serviceName, field),
				namedEntries(upstreamDoc, serviceName, field),
			)
			if !changed {
				continue
			}
			service := ensureService(finalDoc, serviceName)
			if len(merged) == 0 {
				delete(service, field)
			} else {
				value := make(map[string]any, len(merged))
				for _, entry := range merged {
					value[entry.name] = entry.value
				}
				service[field] = value
			}
			rewrites++
		}
	}
	content, err := yaml.Marshal(finalDoc)
	return content, rewrites, err
}

func mergeNamedOrigins(old, user, upstream []namedEntry) ([]namedEntry, bool) {
	oldByName := entriesByName(old)
	userByName := entriesByName(user)
	upstreamByName := entriesByName(upstream)
	oldByToken, oldUnique := entriesByStableToken(old)
	userByToken, userUnique := entriesByStableToken(user)
	upstreamByToken, upstreamUnique := entriesByStableToken(upstream)

	changed := false
	seen := map[string]bool{}
	var result []namedEntry
	for _, oldEntry := range old {
		userEntry, userPresent := userByName[oldEntry.name]
		upstreamEntry, upstreamPresent := upstreamByName[oldEntry.name]
		token := stableOriginToken(oldEntry.name)
		if token != "" && oldUnique[token] && tokenIsUniqueOrAbsent(token, userByToken, userUnique) && tokenIsUniqueOrAbsent(token, upstreamByToken, upstreamUnique) {
			if candidate, exists := userByToken[token]; exists {
				userEntry, userPresent = candidate, true
			}
			if candidate, exists := upstreamByToken[token]; exists {
				upstreamEntry, upstreamPresent = candidate, true
			}
			if (userPresent && userEntry.name != oldEntry.name) || (upstreamPresent && upstreamEntry.name != oldEntry.name) {
				changed = true
			}
		}
		if !userPresent {
			continue
		}
		selected := userEntry
		if userEntry.name == oldEntry.name {
			if !upstreamPresent {
				continue
			}
			selected = upstreamEntry
		}
		appendNamedEntry(&result, seen, selected)
	}
	for _, entry := range upstream {
		if _, exact := oldByName[entry.name]; exact || matchedOldToken(entry, oldByToken, oldUnique) {
			continue
		}
		appendNamedEntry(&result, seen, entry)
	}
	for _, entry := range user {
		if _, exact := oldByName[entry.name]; exact || matchedOldToken(entry, oldByToken, oldUnique) {
			continue
		}
		appendNamedEntry(&result, seen, entry)
	}
	return result, changed
}

func tokenIsUniqueOrAbsent(token string, entries map[string]namedEntry, unique map[string]bool) bool {
	_, exists := entries[token]
	return !exists || unique[token]
}

func matchedOldToken(entry namedEntry, old map[string]namedEntry, unique map[string]bool) bool {
	token := stableOriginToken(entry.name)
	_, exists := old[token]
	return token != "" && exists && unique[token]
}

func appendNamedEntry(result *[]namedEntry, seen map[string]bool, entry namedEntry) {
	if seen[entry.name] {
		return
	}
	seen[entry.name] = true
	*result = append(*result, entry)
}

func entriesByName(entries []namedEntry) map[string]namedEntry {
	result := make(map[string]namedEntry, len(entries))
	for _, entry := range entries {
		result[entry.name] = entry
	}
	return result
}

func entriesByStableToken(entries []namedEntry) (map[string]namedEntry, map[string]bool) {
	result := map[string]namedEntry{}
	unique := map[string]bool{}
	for _, entry := range entries {
		token := stableOriginToken(entry.name)
		if token == "" {
			continue
		}
		if _, exists := result[token]; exists {
			unique[token] = false
			continue
		}
		result[token] = entry
		unique[token] = true
	}
	return result, unique
}

func stableOriginToken(value string) string {
	index := strings.LastIndexByte(value, '-')
	if index < 0 || index == len(value)-1 {
		return ""
	}
	return value[index+1:]
}

func namedEntries(document map[string]any, serviceName, field string) []namedEntry {
	value := serviceAt(document, serviceName)[field]
	var result []namedEntry
	switch current := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			result = append(result, namedEntry{name: key, value: normalizeNamedEntryValue(field, current[key])})
		}
	case []any:
		for _, raw := range current {
			result = append(result, namedEntry{name: fmt.Sprint(raw), value: normalizeNamedEntryValue(field, nil)})
		}
	}
	return result
}

func normalizeNamedEntryValue(field string, value any) any {
	if field != "depends_on" {
		if value == nil {
			return map[string]any{}
		}
		return value
	}
	mapping, _ := value.(map[string]any)
	if len(mapping) == 0 {
		return map[string]any{"condition": "service_started", "required": true}
	}
	return value
}

func decodeDocument(content []byte) (map[string]any, error) {
	var result map[string]any
	if err := yaml.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func serviceNames(documents ...map[string]any) []string {
	set := map[string]bool{}
	for _, document := range documents {
		services, _ := document["services"].(map[string]any)
		for name := range services {
			set[name] = true
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func serviceAt(document map[string]any, serviceName string) map[string]any {
	services, _ := document["services"].(map[string]any)
	service, _ := services[serviceName].(map[string]any)
	if service == nil {
		return map[string]any{}
	}
	return service
}

func ensureService(document map[string]any, serviceName string) map[string]any {
	services, _ := document["services"].(map[string]any)
	if services == nil {
		services = map[string]any{}
		document["services"] = services
	}
	service, _ := services[serviceName].(map[string]any)
	if service == nil {
		service = map[string]any{}
		services[serviceName] = service
	}
	return service
}
