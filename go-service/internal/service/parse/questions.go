package parse

// requiredParams must be known before the skeleton step may start.
var requiredParams = []string{"programType", "title", "domain", "totalHours", "audience", "entryLevel"}

var questions = map[string]string{
	"programType":          "Какой тип программы: ДПП повышения квалификации или дополнительная общеразвивающая программа?",
	"title":                "Как называется программа?",
	"domain":               "К какой области ИТ относится программа?",
	"totalHours":           "Сколько академических часов в программе (от 16 до 500)?",
	"audience":             "Для какой аудитории программа?",
	"entryLevel":           "Какой входной уровень слушателей?",
	"studyForm":            "Какая форма обучения: очная, очно-заочная или заочная?",
	"finalAttestationForm": "Какая форма итоговой аттестации?",
}
