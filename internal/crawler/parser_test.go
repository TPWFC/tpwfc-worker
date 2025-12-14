package crawler

import (
	"testing"
)

func TestParser_ParseDocument_Comprehensive(t *testing.T) {
	markdown := `
<!-- BASIC_INFO_START -->
| INCIDENT_ID | WANG_FUK_COURT_FIRE_2025 |
| DATE_RANGE | 2025-11-26/2025-11-28 |
| LOCATION | 大埔宏福苑宏昌閣 |
| MAP | [地圖](https://maps.google.com) |
| DISASTER_LEVEL | LEVEL_5 |
| DURATION | 01:19:27:00 |
| AFFECTED_BUILDINGS | 7 |
| SOURCES | HK01,SBS_AUSTRALIA |
<!-- BASIC_INFO_END -->

### 起火原因

<!-- FIRE_CAUSE_START -->
外牆大型維修工程的棚架起火。
<!-- FIRE_CAUSE_END -->

### 災情嚴重性

<!-- SEVERITY_START -->
是次火警為香港60年來最嚴重的火災事故。
<!-- SEVERITY_END -->

## 關鍵統計數據

<!-- KEY_STATISTICS_START -->
| FIRE_DURATION | 01:19:27:00 |
| FIRE_LEVEL | LEVEL_5 |
| FINAL_DEATHS | 128 |
| FIREFIGHTER_CASUALTIES | INJURED:11,DEAD:1 |
| FIREFIGHTERS_DEPLOYED | 1250 |
| FIRE_VEHICLES | 304 |
| HELP_CASES | 346 |
| HELP_CASES_PROCESSED | 296 |
| AFFECTED_BUILDINGS | 7 |
| SHELTER_USERS | 900 |
| MISSING_PERSONS | 279 |
| UNIDENTIFIED_BODIES | 89 |
<!-- KEY_STATISTICS_END -->

## 資料來源

<!-- SOURCES_START -->
| SOURCE_NAME | SOURCE_TITLE | SOURCE_URL |
|-------------|--------------|------------|
| HK01 | 宏福苑大火 | https://hk01.com |
| SBS | Title | |
<!-- SOURCES_END -->

## 註釋

<!-- NOTES_START -->
- Note 1
- Note 2
<!-- NOTES_END -->
`

	parser := NewParser()

	doc, err := parser.ParseDocument(markdown)
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	// Verify Basic Info
	if doc.BasicInfo.IncidentID != "WANG_FUK_COURT_FIRE_2025" {
		t.Errorf("Expected IncidentID WANG_FUK_COURT_FIRE_2025, got %s", doc.BasicInfo.IncidentID)
	}

	if doc.BasicInfo.Map != "[地圖](https://maps.google.com)" {
		t.Errorf("Expected Map markdown link [地圖](https://maps.google.com), got %s", doc.BasicInfo.Map)
	}

	if doc.BasicInfo.Duration.Days != 1 || doc.BasicInfo.Duration.Hours != 19 {
		t.Errorf("Expected Duration 1d 19h, got %d d %d h", doc.BasicInfo.Duration.Days, doc.BasicInfo.Duration.Hours)
	}

	// Verify Fire Cause
	if doc.FireCause != "外牆大型維修工程的棚架起火。" {
		t.Errorf("Unexpected Fire Cause: %s", doc.FireCause)
	}

	// Verify Key Statistics
	if doc.KeyStatistics.FinalDeaths != 128 {
		t.Errorf("Expected FinalDeaths 128, got %d", doc.KeyStatistics.FinalDeaths)
	}

	if doc.KeyStatistics.FirefighterCasualties.Deaths != 1 || doc.KeyStatistics.FirefighterCasualties.Injured != 11 {
		t.Errorf("Expected FirefighterCasualties Deaths:1, Injured:11, got Deaths:%d, Injured:%d",
			doc.KeyStatistics.FirefighterCasualties.Deaths, doc.KeyStatistics.FirefighterCasualties.Injured)
	}

	if doc.KeyStatistics.FirefightersDeployed != 1250 {
		t.Errorf("Expected FirefightersDeployed 1250, got %d", doc.KeyStatistics.FirefightersDeployed)
	}

	// Verify Sources
	if len(doc.Sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(doc.Sources))
	}

	if doc.Sources[0].Name != "HK01" {
		t.Errorf("Expected Source[0] Name HK01, got %s", doc.Sources[0].Name)
	}

	// Verify Notes
	if len(doc.Notes) != 2 {
		t.Errorf("Expected 2 notes, got %d", len(doc.Notes))
	}
}

func TestParser_ParseCategoryMetrics(t *testing.T) {
	markdown := `
<!-- CATEGORY_METRICS_START -->

| CATEGORY | METRIC_KEY | METRIC_LABEL | METRIC_VALUE | METRIC_UNIT |
|----------|------------|--------------|--------------|-------------|
| FIREFIGHTING | PERSONNEL_DEPLOYED | 出動人員 | 1250 | 人 |
| RELIEF | AID_FUND | 援助資金 | 300000000 | 港元 |
| FIRE_ESCALATION | MAX_FIRE_LEVEL | 最高級別 | 5 | 級 |

<!-- CATEGORY_METRICS_END -->
`
	parser := NewParser()
	doc, err := parser.ParseDetailedTimeline(markdown)
	if err != nil {
		t.Fatalf("ParseDetailedTimeline failed: %v", err)
	}

	if len(doc.CategoryMetrics) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(doc.CategoryMetrics))
	}

	// Test first metric
	if doc.CategoryMetrics[0].Category != "FIREFIGHTING" {
		t.Errorf("Expected Category FIREFIGHTING, got %s", doc.CategoryMetrics[0].Category)
	}
	if doc.CategoryMetrics[0].MetricKey != "PERSONNEL_DEPLOYED" {
		t.Errorf("Expected MetricKey PERSONNEL_DEPLOYED, got %s", doc.CategoryMetrics[0].MetricKey)
	}
	if doc.CategoryMetrics[0].MetricLabel != "出動人員" {
		t.Errorf("Expected MetricLabel 出動人員, got %s", doc.CategoryMetrics[0].MetricLabel)
	}
	if doc.CategoryMetrics[0].MetricValue != 1250 {
		t.Errorf("Expected MetricValue 1250, got %f", doc.CategoryMetrics[0].MetricValue)
	}
	if doc.CategoryMetrics[0].MetricUnit != "人" {
		t.Errorf("Expected MetricUnit 人, got %s", doc.CategoryMetrics[0].MetricUnit)
	}

	// Test second metric (large number)
	if doc.CategoryMetrics[1].MetricValue != 300000000 {
		t.Errorf("Expected MetricValue 300000000, got %f", doc.CategoryMetrics[1].MetricValue)
	}
}
