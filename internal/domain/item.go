package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
)

var allowedUnits = map[string]bool{"kg": true, "g": true, "L": true, "mL": true, "piece": true}

var categoryUnits = map[string]map[string]bool{
	"solid":     {"kg": true, "g": true},
	"liquid":    {"L": true, "mL": true},
	"container": {"piece": true},
}

var allowedHazards = map[string]bool{
	"flammable": true, "corrosive": true, "toxic": true, "oxidizing": true,
	"reactive": true, "infectious": true, "environmental": true,
}

type WasteItem struct {
	ID               string       `json:"id"`
	BatchID          string       `json:"batchId"`
	MaterialName     string       `json:"materialName"`
	HazardClasses    []string     `json:"hazardClasses"`
	Quantity         float64      `json:"quantity"`
	Unit             string       `json:"unit"`
	DisposalCategory string       `json:"disposalCategory"`
	ContainerType    string       `json:"containerType"`
	SealChecked      bool         `json:"sealChecked"`
	LabelChecked     bool         `json:"labelChecked"`
	ReviewStatus     ReviewStatus `json:"reviewStatus"`
	CorrectionNote   string       `json:"correctionNote,omitempty"`
	UpdatedAt        time.Time    `json:"updatedAt"`
}

type ItemInput struct {
	MaterialName     string   `json:"materialName"`
	HazardClasses    []string `json:"hazardClasses"`
	Quantity         float64  `json:"quantity"`
	Unit             string   `json:"unit"`
	DisposalCategory string   `json:"disposalCategory"`
}

type PackageInput struct {
	ContainerType string `json:"containerType"`
	SealChecked   bool   `json:"sealChecked"`
	LabelChecked  bool   `json:"labelChecked"`
}

func (in ItemInput) Normalize() ItemInput {
	in.MaterialName = strings.TrimSpace(in.MaterialName)
	in.Unit = strings.TrimSpace(in.Unit)
	in.DisposalCategory = strings.ToLower(strings.TrimSpace(in.DisposalCategory))
	seen := make(map[string]bool)
	clean := make([]string, 0, len(in.HazardClasses))
	for _, value := range in.HazardClasses {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			clean = append(clean, value)
		}
	}
	sort.Strings(clean)
	in.HazardClasses = clean
	return in
}

func (in ItemInput) Validate() error {
	in = in.Normalize()
	if in.MaterialName == "" {
		return Required("materialName", "废弃物名称")
	}
	if len(in.MaterialName) > 120 {
		return Invalid("materialName", "废弃物名称不能超过 120 个字符")
	}
	if len(in.HazardClasses) == 0 {
		return Required("hazardClasses", "危险属性")
	}
	for _, hazard := range in.HazardClasses {
		if !allowedHazards[hazard] {
			return Invalid("hazardClasses", fmt.Sprintf("不支持的危险属性：%s", hazard))
		}
	}
	if math.IsNaN(in.Quantity) || math.IsInf(in.Quantity, 0) || in.Quantity <= 0 || in.Quantity > 1_000_000 {
		return Invalid("quantity", "数量必须大于 0 且不超过 1000000")
	}
	if !allowedUnits[in.Unit] {
		return Invalid("unit", "数量单位必须是 kg、g、L、mL 或 piece")
	}
	units, ok := categoryUnits[in.DisposalCategory]
	if !ok {
		return Invalid("disposalCategory", "处置类别必须是 solid、liquid 或 container")
	}
	if !units[in.Unit] {
		return Invalid("unit", "数量单位与处置类别不一致")
	}
	return nil
}

func (in PackageInput) Validate() error {
	if strings.TrimSpace(in.ContainerType) == "" {
		return Required("containerType", "容器类型")
	}
	if len(strings.TrimSpace(in.ContainerType)) > 80 {
		return Invalid("containerType", "容器类型不能超过 80 个字符")
	}
	return nil
}

func (w WasteItem) PackageComplete() bool {
	return strings.TrimSpace(w.ContainerType) != "" && w.SealChecked && w.LabelChecked
}
