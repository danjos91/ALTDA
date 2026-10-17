package validate

import (
	"fmt"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// validateTools flags tools marked outdated (used when updating old programs).
func validateTools(c *checker, root *program.Root) {
	for _, tool := range root.Tools {
		if tool.V.Status != "outdated" {
			continue
		}
		latest := tool.V.LatestVersion
		if latest == "" {
			latest = "?"
		}
		c.check(ruleToolOutdated, false, "",
			strings.TrimSpace(fmt.Sprintf("%s %s is outdated (current: %s); topics: %v",
				tool.V.Name, tool.V.Version, latest, tool.V.UsedInTopics)), tool.V.UsedInTopics...)
	}
}

func validateSources(c *checker, root *program.Root) {
	for i, s := range root.Sources {
		c.check(ruleSourceUnverified, s.V.Verified, fmt.Sprintf("/sources/%d", i),
			fmt.Sprintf("Source %s: unverified", s.V.ID), s.V.ID)
	}
}

func validateSections(c *checker, root *program.Root) {
	for _, s := range root.TemplateSections {
		c.check(ruleSectionMissing, s.V.Status != "missing", "",
			fmt.Sprintf("Empty template section: %s", s.V.Title), s.V.Key)
	}
}
