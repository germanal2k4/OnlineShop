package packaging

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type Packaging []string

func (p Packaging) Value() (driver.Value, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (p *Packaging) Scan(src interface{}) error {
	if src == nil {
		*p = nil
		return nil
	}
	str, ok := src.(string)
	if !ok {
		return fmt.Errorf("packaging: expected string, got %T", src)
	}
	var tmp []string
	if err := json.Unmarshal([]byte(str), &tmp); err != nil {
		return err
	}
	*p = tmp
	return nil
}
