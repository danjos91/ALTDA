package validate

import (
	"fmt"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
)

// addActivity counts an activity in the hours column its kind maps to.
func addActivity(u *units, kind string, hours float64) {
	switch kind {
	case "lecture":
		u.lecture += half(hours)
	case "practice":
		u.practice += half(hours)
	case "lab":
		u.lab += half(hours)
	case "selfStudy":
		u.selfStudy += half(hours)
	case "control":
		u.assessment += half(hours)
	}
}

func validateHours(c *checker, root *program.Root) {
	curriculum := root.Curriculum.V

	var modulesAndAttestation units
	for mi := range curriculum.Modules {
		module := &curriculum.Modules[mi].V
		modulePath := fmt.Sprintf("/curriculum/modules/%d", mi)
		moduleRow := unitsOf(module.Hours)

		checkRow(c, module.Hours, modulePath+"/hours", module.ID)

		var topicsSum units
		for ti := range module.Topics {
			topic := &module.Topics[ti].V
			topicPath := fmt.Sprintf("%s/topics/%d", modulePath, ti)

			checkRow(c, topic.Hours, topicPath+"/hours", topic.ID)
			topicsSum = topicsSum.plus(unitsOf(topic.Hours))
			checkActivities(c, topic, topicPath+"/activities")
		}

		// A module-level assessment may live only on the module row.
		topicsSum.assessment = moduleRow.assessment
		topicsSum.total = topicsSum.lecture + topicsSum.practice + topicsSum.lab +
			topicsSum.selfStudy + topicsSum.assessment
		c.check(ruleHoursTopicsEqModule,
			topicsSum.sameKinds(moduleRow) && topicsSum.total == moduleRow.total,
			modulePath+"/hours",
			fmt.Sprintf("%s: topics add up to %v h, module says %v h",
				module.ID, hoursOf(topicsSum.total), hoursOf(moduleRow.total)),
			module.ID)

		modulesAndAttestation = modulesAndAttestation.plus(moduleRow)
	}

	attestation := root.FinalAttestation.V.Hours
	checkRow(c, attestation, "/finalAttestation/hours", "Final attestation")
	modulesAndAttestation = modulesAndAttestation.plus(unitsOf(attestation))

	declared := unitsOf(curriculum.Totals)
	c.check(ruleHoursTotalsField, modulesAndAttestation.sameAll(declared), "/curriculum/totals",
		fmt.Sprintf("declared totals (%v h) differ from modules + final attestation (%v h)",
			hoursOf(declared.total), hoursOf(modulesAndAttestation.total)))

	requested := root.Header.V.TotalHours
	c.check(ruleHoursEqRequested, modulesAndAttestation.total == half(requested), "/header/totalHours",
		fmt.Sprintf("The plan adds up to %v h, requested %v h",
			hoursOf(modulesAndAttestation.total), requested))

	checkPracticeShare(c, root, modulesAndAttestation)
}

// checkRow verifies the two identities every hours row must satisfy.
func checkRow(c *checker, h program.Hours, path, label string) {
	contact := h.Lecture + h.Practice + h.Lab
	c.check(ruleHoursContact, half(h.Contact) == half(contact), path,
		fmt.Sprintf("%s: contact=%v, expected %v", label, h.Contact, contact))

	total := contact + h.SelfStudy + h.Assessment
	c.check(ruleHoursRowSum, half(h.Total) == half(total), path,
		fmt.Sprintf("%s: total=%v, expected %v", label, h.Total, total))
}

// checkPracticeShare: (practice + lab) / total must reach the share the
// request asked for. It only applies when the request states one.
func checkPracticeShare(c *checker, root *program.Root, grand units) {
	if root.Request == nil || root.Request.V.Params == nil {
		return
	}
	minShare := root.Request.V.Params.V.MinPracticeShare
	if minShare == nil || grand.total <= 0 {
		return
	}

	share := float64(grand.practice+grand.lab) / float64(grand.total)
	c.check(ruleHoursPracticeShare, share+1e-9 >= *minShare, "/curriculum/totals",
		fmt.Sprintf("Practice share %.0f%% < required %.0f%%", share*100, *minShare*100))
}

// checkActivities: when a topic lists activities, they must reproduce the
// topic's hours kind by kind.
func checkActivities(c *checker, topic *program.Topic, path string) {
	if len(topic.Activities) == 0 {
		return
	}

	var fromActivities units
	for _, activity := range topic.Activities {
		addActivity(&fromActivities, activity.V.Kind, activity.V.Hours)
	}

	topicRow := unitsOf(topic.Hours)
	c.check(ruleHoursActivitiesEqTopic, fromActivities.sameKinds(topicRow), path,
		fmt.Sprintf("%s: activities do not reproduce the topic's hours", topic.ID), topic.ID)
}
