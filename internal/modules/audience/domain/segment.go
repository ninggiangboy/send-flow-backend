package domain

import (
	"fmt"
	"strings"
)

type SegmentRule struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

func ExtractRules(def map[string]any) ([]SegmentRule, error) {
	rulesRaw, ok := def["rules"]
	if !ok {
		return nil, ErrSegmentDefinitionInvalid
	}
	rulesArr, ok := rulesRaw.([]any)
	if !ok || len(rulesArr) == 0 {
		return nil, ErrSegmentDefinitionInvalid
	}

	rules := make([]SegmentRule, 0, len(rulesArr))
	for _, r := range rulesArr {
		ruleMap, ok := r.(map[string]any)
		if !ok {
			return nil, ErrSegmentDefinitionInvalid
		}
		field, _ := ruleMap["field"].(string)
		op, _ := ruleMap["operator"].(string)
		val := ruleMap["value"]
		if field == "" || op == "" {
			return nil, ErrSegmentDefinitionInvalid
		}
		rules = append(rules, SegmentRule{Field: field, Operator: op, Value: val})
	}
	return rules, nil
}

func getContactField(c Contact, field string) interface{} {
	switch field {
	case "email":
		return c.Email
	case "email_normalized":
		return c.EmailNormalized
	case "status":
		return string(c.Status)
	case "first_name":
		return c.FirstName
	case "last_name":
		return c.LastName
	case "created_at":
		return c.CreatedAt
	case "updated_at":
		return c.UpdatedAt
	case "tags":
		return c.Tags
	default:
		if len(field) > 11 && field[:11] == "attributes." {
			if c.Attributes != nil {
				return c.Attributes[field[11:]]
			}
		}
		if c.Attributes != nil {
			return c.Attributes[field]
		}
		return nil
	}
}

func MatchesSegment(c Contact, rules []SegmentRule) bool {
	for _, rule := range rules {
		fieldVal := getContactField(c, rule.Field)
		if !evaluateRule(fieldVal, rule.Operator, rule.Value) {
			return false
		}
	}
	return true
}

func evaluateRule(fieldVal interface{}, operator string, ruleVal interface{}) bool {
	switch operator {
	case "eq":
		return compareEq(fieldVal, ruleVal)
	case "neq":
		return !compareEq(fieldVal, ruleVal)
	case "contains":
		return contains(fieldVal, ruleVal)
	case "in":
		return inList(fieldVal, ruleVal)
	case "gte":
		return compareGte(fieldVal, ruleVal)
	case "lte":
		return compareLte(fieldVal, ruleVal)
	default:
		return false
	}
}

func compareEq(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch va := a.(type) {
	case string:
		vb, ok := b.(string)
		return ok && va == vb
	case float64:
		vb, ok := b.(float64)
		return ok && va == vb
	case bool:
		vb, ok := b.(bool)
		return ok && va == vb
	default:
		return false
	}
}

func contains(fieldVal, ruleVal interface{}) bool {
	if fieldVal == nil || ruleVal == nil {
		return false
	}
	switch va := fieldVal.(type) {
	case string:
		vb, ok := ruleVal.(string)
		return ok && containsString(va, vb)
	case []string:
		vb, ok := ruleVal.(string)
		if !ok {
			return false
		}
		for _, s := range va {
			if s == vb {
				return true
			}
		}
		return false
	case []interface{}:
		vb := fmt.Sprintf("%v", ruleVal)
		for _, s := range va {
			if fmt.Sprintf("%v", s) == vb {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func containsString(s, substr string) bool {
	return len(substr) > 0 && strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func inList(fieldVal, ruleVal interface{}) bool {
	if fieldVal == nil || ruleVal == nil {
		return false
	}
	list, ok := ruleVal.([]interface{})
	if !ok {
		return false
	}
	for _, item := range list {
		if compareEq(fieldVal, item) {
			return true
		}
	}
	return false
}

func compareGte(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	fa, aok := toFloat64(a)
	fb, bok := toFloat64(b)
	if aok && bok {
		return fa >= fb
	}
	sa, aok := a.(string)
	sb, bok := b.(string)
	if aok && bok {
		return sa >= sb
	}
	return false
}

func compareLte(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	fa, aok := toFloat64(a)
	fb, bok := toFloat64(b)
	if aok && bok {
		return fa <= fb
	}
	sa, aok := a.(string)
	sb, bok := b.(string)
	if aok && bok {
		return sa <= sb
	}
	return false
}

func toFloat64(v interface{}) (float64, bool) {
	switch va := v.(type) {
	case float64:
		return va, true
	case int:
		return float64(va), true
	case int64:
		return float64(va), true
	default:
		return 0, false
	}
}
