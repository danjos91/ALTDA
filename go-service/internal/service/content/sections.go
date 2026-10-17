package content

import "github.com/VladimirAzanza/ALTDA/go-service/internal/program"

// The template sections the content step reports on. The official list per
// program type comes from the АЦР examples (a task for the PM); until then an
// existing list is respected: known keys get their status updated and any
// section this step does not know is left as it is.
type sectionSpec struct {
	key, title, jsonPath string
}

var sectionSpecs = []sectionSpec{
	{"title_page", "Титульный лист", "/header"},
	{"explanatory_note", "Пояснительная записка", "/goal"},
	{"planned_results", "Планируемые результаты", "/competencies"},
	{"curriculum", "Учебный план", "/curriculum"},
	{"calendar", "Календарный учебный график", "/calendar"},
	{"lectures", "Планы занятий", "/lectures"},
	{"assessment_materials", "Оценочные материалы", "/assessment"},
	{"sources", "Список источников", "/sources"},
	{"normative_basis", "Нормативная база", "/normativeBasis"},
}

// sectionStatus decides each section's state from what the program holds.
func sectionStatus(root *program.Root, key string, placeholder bool) string {
	generated := "filled"
	if placeholder {
		generated = "placeholder"
	}

	switch key {
	case "title_page":
		return "placeholder" // order number, date and signatures are left for a person
	case "explanatory_note":
		return "filled"
	case "planned_results":
		if len(root.Competencies) > 0 {
			return "filled"
		}
		return "missing"
	case "curriculum", "calendar":
		return "filled"
	case "lectures", "assessment_materials":
		return generated
	case "sources":
		if len(root.Sources) > 0 {
			return "filled"
		}
		return "missing" // nothing is invented: sources come from a verified catalog
	case "normative_basis":
		if len(root.NormativeBasis) > 0 {
			return "filled"
		}
		return "missing"
	}
	return ""
}

func updateSections(root *program.Root, placeholder bool) {
	existing := map[string]int{}
	for i, s := range root.TemplateSections {
		existing[s.V.Key] = i
	}

	for _, spec := range sectionSpecs {
		status := sectionStatus(root, spec.key, placeholder)
		if i, ok := existing[spec.key]; ok {
			root.TemplateSections[i].V.Status = status
			if root.TemplateSections[i].V.JSONPath == "" {
				root.TemplateSections[i].V.JSONPath = spec.jsonPath
			}
			continue
		}
		root.TemplateSections = append(root.TemplateSections, program.Node[program.TemplateSection]{
			V: program.TemplateSection{Key: spec.key, Title: spec.title, Status: status, JSONPath: spec.jsonPath},
		})
	}
}
