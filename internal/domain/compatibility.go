package domain

import (
	"sort"
	"strings"
)

const CompatibilityRuleVersion = "2026.1"

type RiskLevel string

const (
	RiskBlocking RiskLevel = "blocking"
	RiskWarning  RiskLevel = "warning"
)

type CompatibilityFinding struct {
	RuleCode    string    `json:"ruleCode"`
	RiskLevel   RiskLevel `json:"riskLevel"`
	ItemIDs     []string  `json:"itemIds"`
	Remediation string    `json:"remediation"`
}

type CompatibilityReport struct {
	RuleVersion   string                 `json:"ruleVersion"`
	Findings      []CompatibilityFinding `json:"findings"`
	BlockingCount int                    `json:"blockingCount"`
	WarningCount  int                    `json:"warningCount"`
	Summary       string                 `json:"summary"`
}

func hasHazard(item WasteItem, hazard string) bool {
	for _, value := range item.HazardClasses {
		if value == hazard {
			return true
		}
	}
	return false
}

func (b HandoverBatch) CompatibilityPreflight() CompatibilityReport {
	items := b.SortedItems()
	findings := make([]CompatibilityFinding, 0)
	addPair := func(left, right WasteItem, a, z, code string, level RiskLevel, remediation string) {
		if (hasHazard(left, a) && hasHazard(right, z)) || (hasHazard(left, z) && hasHazard(right, a)) {
			findings = append(findings, CompatibilityFinding{RuleCode: code, RiskLevel: level, ItemIDs: []string{left.ID, right.ID}, Remediation: remediation})
		}
	}
	for i := 0; i < len(items); i++ {
		container := strings.ToLower(items[i].ContainerType)
		if hasHazard(items[i], "reactive") && (strings.Contains(container, "metal") || strings.Contains(container, "金属")) {
			findings = append(findings, CompatibilityFinding{RuleCode: "CONTAINER_REACTIVE_METAL", RiskLevel: RiskBlocking, ItemIDs: []string{items[i].ID}, Remediation: "反应性废弃物不得使用金属容器，请改用经相容性确认的惰性容器。"})
		}
		if hasHazard(items[i], "corrosive") && (strings.Contains(container, "metal") || strings.Contains(container, "金属")) {
			findings = append(findings, CompatibilityFinding{RuleCode: "CONTAINER_CORROSIVE_METAL", RiskLevel: RiskWarning, ItemIDs: []string{items[i].ID}, Remediation: "请复核腐蚀性废弃物与金属容器的材质相容性。"})
		}
		for j := i + 1; j < len(items); j++ {
			addPair(items[i], items[j], "oxidizing", "flammable", "HAZARD_OXIDIZER_FLAMMABLE", RiskBlocking, "氧化性与易燃废弃物必须拆分为不同交接批次。")
			addPair(items[i], items[j], "reactive", "corrosive", "HAZARD_REACTIVE_CORROSIVE", RiskBlocking, "反应性与腐蚀性废弃物必须隔离并分别交接。")
			if items[i].DisposalCategory != items[j].DisposalCategory && (hasHazard(items[i], "infectious") || hasHazard(items[j], "infectious")) {
				findings = append(findings, CompatibilityFinding{RuleCode: "CATEGORY_INFECTIOUS_MIX", RiskLevel: RiskWarning, ItemIDs: []string{items[i].ID, items[j].ID}, Remediation: "感染性废弃物与其他处置类别同批时，请确认接收单位的分区接收条件。"})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].RiskLevel != findings[j].RiskLevel {
			return findings[i].RiskLevel < findings[j].RiskLevel
		}
		if findings[i].RuleCode != findings[j].RuleCode {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		return strings.Join(findings[i].ItemIDs, "\x00") < strings.Join(findings[j].ItemIDs, "\x00")
	})
	report := CompatibilityReport{RuleVersion: CompatibilityRuleVersion, Findings: findings}
	for _, finding := range findings {
		if finding.RiskLevel == RiskBlocking {
			report.BlockingCount++
		} else {
			report.WarningCount++
		}
	}
	report.Summary = compatibilitySummary(report.BlockingCount, report.WarningCount)
	return report
}

func compatibilitySummary(blocking, warning int) string {
	return "阻断项 " + itoa(blocking) + " 个，提示项 " + itoa(warning) + " 个"
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
