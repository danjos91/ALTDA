package validate

// rule identifies one validation rule. The catalog below is the single place
// where IDs, categories and severities are defined.
type rule struct {
	id       string
	category Category
	severity Severity
}

var (
	ruleHoursContact           = rule{"HOURS_CONTACT", CategoryHours, SeverityError}
	ruleHoursRowSum            = rule{"HOURS_ROW_SUM", CategoryHours, SeverityError}
	ruleHoursTopicsEqModule    = rule{"HOURS_TOPICS_EQ_MODULE", CategoryHours, SeverityError}
	ruleHoursTotalsField       = rule{"HOURS_TOTALS_FIELD", CategoryHours, SeverityError}
	ruleHoursEqRequested       = rule{"HOURS_EQ_REQUESTED", CategoryHours, SeverityError}
	ruleHoursPracticeShare     = rule{"HOURS_PRACTICE_SHARE", CategoryHours, SeverityError}
	ruleHoursActivitiesEqTopic = rule{"HOURS_ACTIVITIES_EQ_TOPIC", CategoryHours, SeverityError}

	// Rules of the skeleton step; some are shared with the full program.
	ruleSkeletonModuleWeights   = rule{"SKELETON_MODULE_WEIGHTS", CategoryHours, SeverityError}
	ruleSkeletonTopicWeights    = rule{"SKELETON_TOPIC_WEIGHTS", CategoryHours, SeverityError}
	ruleSkeletonDanglingOutcome = rule{"SKELETON_DANGLING_OUTCOME", CategoryConsistency, SeverityError}
	ruleCoverageOutcomeTaught   = rule{"COVERAGE_OUTCOME_TAUGHT", CategoryCoverage, SeverityError}
	ruleOrderPrerequisites      = rule{"ORDER_PREREQUISITES", CategoryConsistency, SeverityError}

	// Rules of the full program.
	ruleCalendarSessionNumbers  = rule{"CALENDAR_SESSION_NUMBERS", CategoryConsistency, SeverityError}
	ruleCalendarHours           = rule{"CALENDAR_HOURS", CategoryHours, SeverityWarning}
	ruleDanglingTopic           = rule{"CONSISTENCY_DANGLING_TOPIC", CategoryConsistency, SeverityError}
	ruleCoverageOutcomeAssessed = rule{"COVERAGE_OUTCOME_ASSESSED", CategoryCoverage, SeverityError}
	ruleOutcomePrefix           = rule{"CONSISTENCY_OUTCOME_PREFIX", CategoryConsistency, SeverityError}
	ruleDanglingOutcome         = rule{"CONSISTENCY_DANGLING_OUTCOME", CategoryConsistency, SeverityError}
	ruleDanglingReference       = rule{"CONSISTENCY_DANGLING_REFERENCE", CategoryConsistency, SeverityError}
	ruleDanglingSource          = rule{"CONSISTENCY_DANGLING_SOURCE", CategoryConsistency, SeverityError}
	ruleProfStandardLink        = rule{"CONSISTENCY_PROFSTANDARD_LINK", CategoryConsistency, SeverityError}
	ruleLectureMinutes          = rule{"LECTURE_MINUTES", CategoryHours, SeverityError}
	ruleTestKey                 = rule{"TEST_KEY", CategoryConsistency, SeverityError}
	ruleTaskMaxScore            = rule{"TASK_MAXSCORE", CategoryConsistency, SeverityError}
	ruleTaskDuration            = rule{"TASK_DURATION", CategoryHours, SeverityError}
	ruleToolOutdated            = rule{"TOOL_OUTDATED", CategoryConsistency, SeverityWarning}
	ruleSourceUnverified        = rule{"SOURCE_UNVERIFIED", CategorySources, SeverityWarning}
	ruleSectionMissing          = rule{"SECTION_MISSING", CategoryCompleteness, SeverityError}
)

// checker collects the result of every rule evaluation.
type checker struct {
	checks []Check
}

func (c *checker) check(r rule, passed bool, path, message string, related ...string) {
	c.checks = append(c.checks, Check{
		RuleID:     r.id,
		Category:   r.category,
		Severity:   r.severity,
		Passed:     passed,
		Message:    message,
		Path:       path,
		RelatedIDs: related,
	})
}

func (c *checker) summary() Summary {
	var s Summary
	for _, check := range c.checks {
		switch {
		case check.Passed:
			s.Passed++
		case check.Severity == SeverityError:
			s.Errors++
		case check.Severity == SeverityWarning:
			s.Warnings++
		}
		if !check.Passed && check.RuleID == "SOURCE_UNVERIFIED" {
			s.UnverifiedSources++
		}
	}
	return s
}
