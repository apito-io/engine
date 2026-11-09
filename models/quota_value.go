package models

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// QuotaValue is one plan quota. JSON numbers stay numbers ("1000", "-1", "0").
// A monthly cap is the string "M1000".
type QuotaValue string

// AsJSON returns a number for numeric quotas and the original text for M-quotas.
func (q QuotaValue) AsJSON() any {
	s := strings.TrimSpace(string(q))
	if s == "" {
		return nil
	}
	if s[0] == 'M' || s[0] == 'm' {
		return s
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	return n
}

func (q QuotaValue) MarshalJSON() ([]byte, error) {
	s := strings.TrimSpace(string(q))
	if s == "" {
		return []byte("null"), nil
	}
	if s[0] == 'M' || s[0] == 'm' {
		return json.Marshal(s)
	}
	if _, err := strconv.Atoi(s); err == nil {
		return []byte(s), nil
	}
	return json.Marshal(s)
}

func (q *QuotaValue) UnmarshalJSON(b []byte) error {
	if q == nil {
		return nil
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*q = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*q = QuotaValue(strings.TrimSpace(s))
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err != nil {
		return err
	}
	if i, err := n.Int64(); err == nil {
		*q = QuotaValue(strconv.FormatInt(i, 10))
		return nil
	}
	f, err := n.Float64()
	if err != nil {
		return err
	}
	*q = QuotaValue(strconv.FormatInt(int64(f), 10))
	return nil
}
